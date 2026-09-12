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
	"github.com/teslacost/teslacost/web"
)

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

	if err != nil {
		log.Printf("[warning] Database connection failed: %v. Running in offline/unconnected mode for now.", err)
	} else {
		defer dbPool.Close()
		if migErr := dbPool.Migrate(ctx); migErr != nil {
			log.Printf("[warning] Database migration failed: %v", migErr)
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

		syncService = services.NewSyncService(repo, encryptor)
		tireWearService = services.NewTireWearService(repo)
		tcoService = services.NewTCOService(dbPool.Pool)
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
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
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
			"version":   "1.0.0",
		})
	}
	r.Get("/api/health", healthHandler)
	r.Get("/healthz", healthHandler)

	// API Routes
	if repo != nil {
		authHandler := handlers.NewAuthHandler(repo, cfg.JWTSecret, cfg.JWTExpirationHours, cfg.DisableRegistration)
		vehicleHandler := handlers.NewVehicleHandler(repo, encryptor, syncService)
		driveHandler := handlers.NewDriveHandler(repo)
		tireHandler := handlers.NewTireHandler(repo, tireWearService)
		expenseHandler := handlers.NewExpenseHandler(repo)
		tcoHandler := handlers.NewTCOHandler(repo, tcoService)
		carpoolHandler := handlers.NewCarpoolHandler(repo, carpoolService)

		// Public Auth
		r.Route("/api/auth", func(r chi.Router) {
			r.Get("/config", authHandler.GetConfig)
			r.Post("/register", authHandler.Register)
			r.Post("/login", authHandler.Login)
		})

		// Protected Routes
		r.Group(func(r chi.Router) {
			r.Use(appMiddleware.AuthenticateJWT(cfg.JWTSecret))

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

				// Drives
				r.Get("/{vehicleId}/drives", driveHandler.List)
				r.Patch("/{vehicleId}/drives/{driveId}/tags", driveHandler.UpdateTags)
				r.Post("/{vehicleId}/trip-groups", driveHandler.CreateTripGroup)
				r.Get("/{vehicleId}/trip-groups", driveHandler.ListTripGroups)

				// Carpooling / BlaBlaCar
				r.Get("/{vehicleId}/carpools", carpoolHandler.List)
				r.Post("/{vehicleId}/carpools", carpoolHandler.Create)
				r.Get("/{vehicleId}/carpools/estimate", carpoolHandler.Estimate)
				r.Get("/{vehicleId}/carpools/{id}", carpoolHandler.Get)
				r.Put("/{vehicleId}/carpools/{id}", carpoolHandler.Update)
				r.Delete("/{vehicleId}/carpools/{id}", carpoolHandler.Delete)

				// Tires
				r.Get("/{vehicleId}/tires", tireHandler.List)
				r.Post("/{vehicleId}/tires", tireHandler.Create)
				r.Post("/{vehicleId}/tires/{tireId}/logs", tireHandler.AddLog)
				r.Post("/{vehicleId}/tire-rotations", tireHandler.Rotate)

				// Expenses
				r.Get("/{vehicleId}/expenses", expenseHandler.ListDriveExpenses)
				r.Post("/{vehicleId}/expenses", expenseHandler.CreateDriveExpense)
				r.Get("/{vehicleId}/maintenance", expenseHandler.ListMaintenance)
				r.Post("/{vehicleId}/maintenance", expenseHandler.CreateMaintenance)
				r.Get("/{vehicleId}/charges", expenseHandler.ListCharges)

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
		WriteTimeout: 15 * time.Second,
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
