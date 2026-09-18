package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
	_ "time/tzdata" // reporting timezone available even in minimal container images

	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/go-chi/httprate"

	"github.com/teslacost/teslacost/internal/auth"
	"github.com/teslacost/teslacost/internal/config"
	"github.com/teslacost/teslacost/internal/crypto"
	"github.com/teslacost/teslacost/internal/database"
	"github.com/teslacost/teslacost/internal/handlers"
	appMiddleware "github.com/teslacost/teslacost/internal/middleware"
	"github.com/teslacost/teslacost/internal/services"
	"github.com/teslacost/teslacost/internal/storage"
	"github.com/teslacost/teslacost/web"
)

// AppVersion is the application version, injected at build time via -ldflags "-X main.AppVersion=...".
var AppVersion = "1.17.0"

// requestIDHandler wraps a slog.Handler to attach the chi request ID (if any is present on the
// context) to every log record. This is what lets a "request_id" field emitted by a *Context
// slog call (e.g. slog.ErrorContext in writeRepoError) be correlated with the chi access log
// line for the same request.
type requestIDHandler struct {
	slog.Handler
}

func (h requestIDHandler) Handle(ctx context.Context, r slog.Record) error {
	if reqID := chiMiddleware.GetReqID(ctx); reqID != "" {
		r.AddAttrs(slog.String("request_id", reqID))
	}
	return h.Handler.Handle(ctx, r)
}

func (h requestIDHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return requestIDHandler{h.Handler.WithAttrs(attrs)}
}

func (h requestIDHandler) WithGroup(name string) slog.Handler {
	return requestIDHandler{h.Handler.WithGroup(name)}
}

// configureLogging sets the process-wide slog default: JSON output in production (log
// aggregators, jq-friendly), human-readable text otherwise. Level defaults to Info in
// production (never Debug) and Debug in development; LOG_LEVEL overrides either.
func configureLogging(cfg *config.Config) {
	level := slog.LevelInfo
	if !strings.EqualFold(cfg.Environment, "production") {
		level = slog.LevelDebug
	}
	if raw := strings.TrimSpace(os.Getenv("LOG_LEVEL")); raw != "" {
		var parsed slog.Level
		if err := parsed.UnmarshalText([]byte(strings.ToUpper(raw))); err == nil {
			level = parsed
		}
	}

	var handler slog.Handler
	opts := &slog.HandlerOptions{Level: level}
	if strings.EqualFold(cfg.Environment, "production") {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}
	slog.SetDefault(slog.New(requestIDHandler{handler}))
}

func main() {
	// 1. Load configuration
	cfg := config.Load()
	configureLogging(cfg)

	slog.Info("Starting TeslaCost Full-Stack Server...")

	// Warn loudly (without blocking startup) if a production deployment still uses one of the
	// well-known placeholder secrets shipped in docker-compose.yml / .env.example.
	if strings.EqualFold(cfg.Environment, "production") {
		if warnings := cfg.InsecureDefaults(); len(warnings) > 0 {
			for _, w := range warnings {
				slog.Warn("insecure default secret detected in production", "component", "security", "detail", w)
			}
		}
	}

	// 2. Initialize encryption module
	encryptor, err := crypto.NewEncryptor(cfg.AppEncryptionKey)
	if err != nil {
		slog.Error("failed to initialize crypto module", "error", err)
		os.Exit(1)
	}

	// 3. Connect to PostgreSQL (with retry to wait for DB startup)
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	dbPool, err := database.Connect(ctx, cfg.DatabaseURL)
	var repo *database.Repository
	var syncService *services.SyncService
	var tireWearService *services.TireWearService
	var tcoService *services.TCOService
	var carpoolService *services.CarpoolService
	var notificationService *services.NotificationService

	if err != nil {
		slog.Warn("database connection failed, running in offline/unconnected mode for now", "error", err)
	} else {
		defer dbPool.Close()
		if migErr := dbPool.Migrate(ctx); migErr != nil {
			slog.Error("database migration failed", "error", migErr)
			os.Exit(1)
		}
		repo = database.NewRepository(dbPool.Pool)

		// Seed initial admin if configured and user does not exist
		if cfg.InitialAdminEmail != "" && cfg.InitialAdminPassword != "" {
			_, err := repo.GetUserByEmail(ctx, cfg.InitialAdminEmail)
			if err != nil && errors.Is(err, database.ErrNotFound) {
				hash, err := auth.HashPassword(cfg.InitialAdminPassword)
				if err == nil {
					adminUser, err := repo.CreateUser(ctx, cfg.InitialAdminEmail, hash)
					if err == nil {
						slog.Info("initial admin account created successfully", "component", "auth", "email", adminUser.Email)
					} else {
						slog.Error("failed to create initial admin account", "component", "auth", "error", err)
					}
				}
			}
		}

		notificationService = services.NewNotificationService(repo)
		syncService = services.NewSyncService(repo, encryptor)
		syncService.SetNotificationService(notificationService)
		tireWearService = services.NewTireWearService(repo)
		tcoService = services.NewTCOService(dbPool.Pool, cfg.ReportingTimezone)
		carpoolService = services.NewCarpoolService(dbPool.Pool, repo)
	}

	// 4. Setup Chi router
	r := chi.NewRouter()

	// Standard middlewares
	r.Use(chiMiddleware.RequestID)
	r.Use(chiMiddleware.RealIP)
	r.Use(chiMiddleware.Logger)
	r.Use(chiMiddleware.Recoverer)
	r.Use(chiMiddleware.Timeout(60 * time.Second))

	// CORS setup
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   cfg.AllowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token", "Idempotency-Key"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Public Health Check Endpoint
	healthHandler := func(w http.ResponseWriter, r *http.Request) {
		dbStatus := "disconnected"
		if dbPool != nil {
			pingCtx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
			defer cancel()
			if err := dbPool.Pool.Ping(pingCtx); err == nil {
				dbStatus = "connected"
			}
		}

		status := "healthy"
		httpStatus := http.StatusOK
		if dbStatus != "connected" {
			status = "unhealthy"
			httpStatus = http.StatusServiceUnavailable
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(httpStatus)
		json.NewEncoder(w).Encode(map[string]any{
			"status":    status,
			"database":  dbStatus,
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"version":   AppVersion,
		})
	}
	r.Get("/api/health", healthHandler)
	r.Get("/healthz", healthHandler)
	r.Get("/api/version", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"version": AppVersion,
		})
	})

	// API Routes
	if repo != nil {
		// Initialize OIDC service if configured.
		var oidcService *auth.OIDCService
		if cfg.OIDCEnabled {
			svc, oidcErr := auth.NewOIDCService(context.Background(), cfg)
			if oidcErr != nil {
				slog.Error("OIDC initialization failed", "component", "auth", "error", oidcErr)
				os.Exit(1)
			}
			oidcService = svc
			slog.Info("OIDC SSO enabled", "component", "auth", "issuer", cfg.OIDCIssuerURL, "provider", cfg.OIDCProviderName)
		} else {
			slog.Info("OIDC not configured, using local JWT auth only", "component", "auth")
		}

		authHandler := handlers.NewAuthHandler(repo, cfg, oidcService)
		vehicleHandler := handlers.NewVehicleHandler(repo, encryptor, syncService)

		tollDetectionService, err := services.NewTollDetectionService(repo, encryptor)
		if err != nil {
			slog.Error("failed to initialize toll detection service", "error", err)
			os.Exit(1)
		}
		driveHandler := handlers.NewDriveHandler(repo, carpoolService, tollDetectionService)
		tireHandler := handlers.NewTireHandler(repo, tireWearService)

		storageService, err := storage.NewFileStorageService(cfg.StorageDir)
		if err != nil {
			slog.Error("failed to initialize file storage service", "error", err)
			os.Exit(1)
		}

		// Migrate any legacy unmigrated documents from PostgreSQL BYTEA column to the storage volume
		migCtx, migCancel := context.WithTimeout(context.Background(), 2*time.Minute)
		if count, err := repo.MigrateLegacyDocuments(migCtx, storageService.Save); err != nil {
			slog.Warn("legacy document migration encountered an error", "component", "storage", "error", err)
		} else if count > 0 {
			slog.Info("migrated legacy documents from database to volume storage", "component", "storage", "count", count)
		}
		migCancel()

		expenseHandler := handlers.NewExpenseHandler(repo, storageService)
		tcoHandler := handlers.NewTCOHandler(repo, tcoService)
		carpoolHandler := handlers.NewCarpoolHandler(repo, carpoolService)
		checkpointHandler := handlers.NewCheckpointHandler(repo)
		reminderHandler := handlers.NewReminderHandler(repo, notificationService)
		vehicleMemberHandler := handlers.NewVehicleMemberHandler(repo)

		// Public Auth
		r.Route("/api/auth", func(r chi.Router) {
			r.Get("/config", authHandler.GetConfig)
			// Rate limited by IP: these are the credential-guessing surface (password brute
			// force, account enumeration via registration). 10 attempts/minute is generous for
			// a legitimate user retrying a typo but blocks automated guessing.
			r.With(httprate.LimitByIP(10, time.Minute)).Post("/register", authHandler.Register)
			r.With(httprate.LimitByIP(10, time.Minute)).Post("/login", authHandler.Login)
			r.Post("/refresh", authHandler.RefreshToken)
			r.Post("/logout", authHandler.Logout)
			// OIDC Authorization Code Flow endpoints (public — no JWT required)
			r.Get("/oidc/login", authHandler.OIDCLogin)
			r.Get("/oidc/callback", authHandler.OIDCCallback)
		})

		// Protected Routes
		r.Group(func(r chi.Router) {
			r.Use(appMiddleware.AuthenticateJWT(cfg.JWTSecret))
			r.Use(handlers.Idempotency(repo))

			r.Get("/api/auth/me", authHandler.Me)

			// Vehicles
			r.Route("/api/vehicles", func(r chi.Router) {
				r.Get("/", vehicleHandler.List)
				r.Post("/", vehicleHandler.Create)
				r.Post("/test-connection", vehicleHandler.TestTeslaMateRaw)
				r.Get("/{id}", vehicleHandler.Get)
				r.Put("/{id}", vehicleHandler.Update)
				r.Delete("/{id}", vehicleHandler.Delete)
				r.Post("/{id}/teslamate/test", vehicleHandler.TestTeslaMate)
				r.Post("/{id}/sync", vehicleHandler.Sync)
				r.Get("/{id}/sync", vehicleHandler.GetSyncStatus)
				r.Get("/{id}/ownership", vehicleHandler.GetOwnership)
				r.Put("/{id}/ownership", vehicleHandler.SaveOwnership)
				r.Delete("/{id}/ownership", vehicleHandler.DeleteOwnership)
				r.Put("/{id}/pre-teslamate-energy", vehicleHandler.UpdatePreTeslaMateEnergy)
				r.Get("/{id}/odometer-at", vehicleHandler.GetOdometerAtDate)
				r.Get("/{vehicleId}/data-quality", tcoHandler.GetDataQuality)

				// Shared Vehicle Members
				r.Get("/{id}/members", vehicleMemberHandler.ListMembers)
				r.Post("/{id}/members", vehicleMemberHandler.AddMember)
				r.Put("/{id}/members/{memberId}", vehicleMemberHandler.UpdateMemberRole)
				r.Delete("/{id}/members/{memberId}", vehicleMemberHandler.RemoveMember)

				// Odometer Checkpoints
				r.Get("/{vehicleId}/odometer-checkpoints", checkpointHandler.List)
				r.Post("/{vehicleId}/odometer-checkpoints", checkpointHandler.Create)
				r.Put("/{vehicleId}/odometer-checkpoints/{checkpointId}", checkpointHandler.Update)
				r.Delete("/{vehicleId}/odometer-checkpoints/{checkpointId}", checkpointHandler.Delete)

				// Drives
				r.Get("/{vehicleId}/drives", driveHandler.List)
				r.Get("/{vehicleId}/drives/{driveId}/expenses", driveHandler.GetDriveExpenses)
				r.Patch("/{vehicleId}/drives/{driveId}/tags", driveHandler.UpdateTags)
				r.Patch("/{vehicleId}/drives/{driveId}/toll-review", driveHandler.SetTollReview)
				r.Get("/{vehicleId}/drives/{driveId}/toll-detection", driveHandler.GetTollDetection)
				r.Post("/{vehicleId}/drives/{driveId}/detect-tolls", driveHandler.DetectTolls)
				r.Post("/{vehicleId}/drives/{driveId}/apply-toll-estimate", driveHandler.ApplyTollEstimate)
				r.Post("/{vehicleId}/drives/apply-toll-estimates", driveHandler.ApplyTollEstimatesBulk)
				r.Post("/{vehicleId}/trip-groups", driveHandler.CreateTripGroup)
				r.Get("/{vehicleId}/trip-groups", driveHandler.ListTripGroups)
				r.Put("/{vehicleId}/trip-groups/{groupId}", driveHandler.UpdateTripGroup)
				r.Delete("/{vehicleId}/trip-groups/{groupId}", driveHandler.DeleteTripGroup)

				// Carpooling / BlaBlaCar
				r.Get("/{vehicleId}/carpools", carpoolHandler.List)
				r.Post("/{vehicleId}/carpools", carpoolHandler.Create)
				r.Post("/{vehicleId}/carpools/recalculate", carpoolHandler.Recalculate)
				r.Get("/{vehicleId}/carpools/estimate", carpoolHandler.Estimate)
				r.Get("/{vehicleId}/carpools/{id}", carpoolHandler.Get)
				r.Put("/{vehicleId}/carpools/{id}", carpoolHandler.Update)
				r.Delete("/{vehicleId}/carpools/{id}", carpoolHandler.Delete)

				// Tires
				r.Get("/{vehicleId}/tires", tireHandler.List)
				r.Post("/{vehicleId}/tires", tireHandler.Create)
				r.Post("/{vehicleId}/tires/batch", tireHandler.BatchCreate)
				r.Patch("/{vehicleId}/tires/batch", tireHandler.BatchUpdate)
				r.Post("/{vehicleId}/tires/batch-dispose", tireHandler.BatchDispose)
				r.Post("/{vehicleId}/tires/quick-rotate", tireHandler.QuickRotate)
				r.Put("/{vehicleId}/tires/{tireId}", tireHandler.Update)
				r.Delete("/{vehicleId}/tires/{tireId}", tireHandler.Delete)
				r.Post("/{vehicleId}/tires/{tireId}/dispose", tireHandler.Dispose)
				r.Post("/{vehicleId}/tires/{tireId}/copy-history", tireHandler.CopyHistory)
				r.Get("/{vehicleId}/tires/{tireId}/history", tireHandler.GetHistory)
				r.Post("/{vehicleId}/tires/{tireId}/sessions", tireHandler.CreateSession)
				r.Put("/{vehicleId}/tires/{tireId}/sessions/{sessionId}", tireHandler.UpdateSession)
				r.Delete("/{vehicleId}/tires/{tireId}/sessions/{sessionId}", tireHandler.DeleteSession)
				r.Post("/{vehicleId}/tires/{tireId}/logs", tireHandler.AddLog)
				r.Put("/{vehicleId}/tires/{tireId}/logs/{logId}", tireHandler.UpdateLog)
				r.Delete("/{vehicleId}/tires/{tireId}/logs/{logId}", tireHandler.DeleteLog)
				r.Post("/{vehicleId}/tire-rotations", tireHandler.Rotate)

				// Expenses
				r.Get("/{vehicleId}/expenses", expenseHandler.ListDriveExpenses)
				r.Post("/{vehicleId}/expenses", expenseHandler.CreateDriveExpense)
				r.Put("/{vehicleId}/expenses/{expenseId}", expenseHandler.UpdateDriveExpense)
				r.Delete("/{vehicleId}/expenses/{expenseId}", expenseHandler.DeleteDriveExpense)
				r.Get("/{vehicleId}/maintenance", expenseHandler.ListMaintenance)
				r.Post("/{vehicleId}/maintenance", expenseHandler.CreateMaintenance)
				r.Put("/{vehicleId}/maintenance/{maintenanceId}", expenseHandler.UpdateMaintenance)
				r.Delete("/{vehicleId}/maintenance/{maintenanceId}", expenseHandler.DeleteMaintenance)
				r.Get("/{vehicleId}/charges", expenseHandler.ListCharges)
				r.Post("/{vehicleId}/charges", expenseHandler.CreateManualCharge)
				r.Put("/{vehicleId}/charges/{chargeId}", expenseHandler.UpdateCharge)
				r.Delete("/{vehicleId}/charges/{chargeId}", expenseHandler.DeleteManualCharge)

				// Documents & Invoices
				r.Get("/{vehicleId}/documents", expenseHandler.ListDocuments)
				r.Post("/{vehicleId}/documents", expenseHandler.UploadDocument)
				r.Get("/{vehicleId}/documents/{docId}", expenseHandler.DownloadDocument)
				r.Delete("/{vehicleId}/documents/{docId}", expenseHandler.DeleteDocument)

				// Maintenance Reminders & Webhooks
				r.Get("/{vehicleId}/reminders", reminderHandler.List)
				r.Post("/{vehicleId}/reminders", reminderHandler.Create)
				r.Put("/{vehicleId}/reminders/{reminderId}", reminderHandler.Update)
				r.Delete("/{vehicleId}/reminders/{reminderId}", reminderHandler.Delete)
				r.Post("/{vehicleId}/reminders/{reminderId}/complete", reminderHandler.Complete)
				r.Get("/{vehicleId}/webhook", reminderHandler.GetWebhook)
				r.Put("/{vehicleId}/webhook", reminderHandler.SaveWebhook)
				r.Delete("/{vehicleId}/webhook", reminderHandler.DeleteWebhook)
				r.Post("/{vehicleId}/webhook/test", reminderHandler.TestWebhook)

				// TCO Analytics
				r.Get("/{vehicleId}/tco", tcoHandler.GetTCO)
			})
		})
	}

	// Embedded Vue 3 Frontend SPA serving
	spaServer := handlers.NewSPAServer(web.GetDistFS())
	r.Handle("/*", spaServer)

	// Server setup
	server := &http.Server{
		Addr:         fmt.Sprintf(":%s", cfg.Port),
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 75 * time.Second, // above the 60s request timeout so long syncs can still respond
		IdleTimeout:  60 * time.Second,
	}

	stopChan := make(chan os.Signal, 1)
	signal.Notify(stopChan, os.Interrupt, syscall.SIGTERM)

	// Background workers (Auto-sync)
	bgCtx, cancelBg := context.WithCancel(context.Background())
	defer cancelBg()

	if syncService != nil && cfg.SyncIntervalMinutes > 0 {
		go syncService.StartBackgroundWorker(bgCtx, cfg.SyncIntervalMinutes)
	}

	go func() {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("recovered panic in HTTP server goroutine", "error", r)
				os.Exit(1)
			}
		}()
		slog.Info("TeslaCost API & Web listening", "port", cfg.Port, "base_url", cfg.AppBaseURL)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	<-stopChan
	slog.Info("Shutting down server...")
	cancelBg()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Error("server forced shutdown error", "error", err)
		os.Exit(1)
	}

	slog.Info("TeslaCost server stopped cleanly.")
}
