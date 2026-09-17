package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	_ "time/tzdata" // reporting timezone available even in minimal container images

	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

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
var AppVersion = "1.13.0"

func main() {
	log.Println("Starting TeslaCost Full-Stack Server...")

	// 1. Load configuration
	cfg := config.Load()

	// 2. Initialize encryption module
	encryptor, err := crypto.NewEncryptor(cfg.AppEncryptionKey)
	if err != nil {
		log.Fatalf("Failed to initialize crypto module: %v", err)
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
		log.Printf("[warning] Database connection failed: %v. Running in offline/unconnected mode for now.", err)
	} else {
		defer dbPool.Close()
		if migErr := dbPool.Migrate(ctx); migErr != nil {
			log.Fatalf("Database migration failed: %v", migErr)
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
						log.Printf("[auth] Initial admin account created successfully (%s)", adminUser.Email)
					} else {
						log.Printf("[auth] Failed to create initial admin account: %v", err)
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
		w.Header().Set("Content-Type", "application/json")
		dbStatus := "connected"
		if dbPool == nil {
			dbStatus = "disconnected"
		}
		json.NewEncoder(w).Encode(map[string]any{
			"status":    "healthy",
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
				log.Fatalf("OIDC initialization failed: %v", oidcErr)
			}
			oidcService = svc
			log.Printf("[auth] OIDC SSO enabled — issuer: %s, provider: %s", cfg.OIDCIssuerURL, cfg.OIDCProviderName)
		} else {
			log.Println("[auth] OIDC not configured — using local JWT auth only")
		}

		authHandler := handlers.NewAuthHandler(repo, cfg, oidcService)
		vehicleHandler := handlers.NewVehicleHandler(repo, encryptor, syncService)
		driveHandler := handlers.NewDriveHandler(repo, carpoolService)
		tireHandler := handlers.NewTireHandler(repo, tireWearService)

		storageService, err := storage.NewFileStorageService(cfg.StorageDir)
		if err != nil {
			log.Fatalf("Failed to initialize file storage service: %v", err)
		}
		expenseHandler := handlers.NewExpenseHandler(repo, storageService)
		tcoHandler := handlers.NewTCOHandler(repo, tcoService)
		carpoolHandler := handlers.NewCarpoolHandler(repo, carpoolService)
		checkpointHandler := handlers.NewCheckpointHandler(repo)
		reminderHandler := handlers.NewReminderHandler(repo, notificationService)
		vehicleMemberHandler := handlers.NewVehicleMemberHandler(repo)

		// Public Auth
		r.Route("/api/auth", func(r chi.Router) {
			r.Get("/config", authHandler.GetConfig)
			r.Post("/register", authHandler.Register)
			r.Post("/login", authHandler.Login)
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
				r.Post("/{vehicleId}/tires/quick-rotate", tireHandler.QuickRotate)
				r.Put("/{vehicleId}/tires/{tireId}", tireHandler.Update)
				r.Delete("/{vehicleId}/tires/{tireId}", tireHandler.Delete)
				r.Post("/{vehicleId}/tires/{tireId}/dispose", tireHandler.Dispose)
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
		log.Printf("TeslaCost API & Web listening on port %s (Base URL: %s)", cfg.Port, cfg.AppBaseURL)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("Server error: %v", err)
		}
	}()

	<-stopChan
	log.Println("Shutting down server...")
	cancelBg()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Server forced shutdown error: %v", err)
	}

	log.Println("TeslaCost server stopped cleanly.")
}
