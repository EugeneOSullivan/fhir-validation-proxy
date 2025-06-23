package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"fhir-validation-proxy/internal/auth"
	"fhir-validation-proxy/internal/config"
	"fhir-validation-proxy/internal/proxy"
	"fhir-validation-proxy/internal/validator"
	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	// Load configuration
	cfg, err := config.LoadConfig("configs/server.yaml")
	if err != nil {
		log.Printf("Warning: Failed to load config file, using defaults: %v", err)
		cfg, _ = config.LoadConfig("")
	}

	// Load validation components
	if err := validator.LoadProfiles(cfg.Validation.ProfilesPath); err != nil {
		log.Fatalf("Failed to load profiles: %v", err)
	}

	if err := validator.LoadRules(cfg.Validation.CustomRulesPath); err != nil {
		log.Fatalf("Failed to load rules: %v", err)
	}

	if err := validator.LoadRecipes(cfg.Validation.RecipesPath); err != nil {
		log.Fatalf("Failed to load recipes: %v", err)
	}

	// Initialize authentication middleware
	authMiddleware, err := auth.NewMiddleware(
		cfg.GoogleCloud.ServiceAccountKey,
		cfg.Security.RequireAuthentication,
	)
	if err != nil {
		log.Fatalf("Failed to initialize auth middleware: %v", err)
	}

	// Create FHIR proxy
	fhirProxy := proxy.NewFHIRProxy(cfg)

	// Setup routers
	router := mux.NewRouter()

	// Apply middleware
	if cfg.Security.AuditLogging {
		router.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				authMiddleware.AuditLog(next.ServeHTTP)(w, r)
			})
		})
	}

	if cfg.Security.RequireAuthentication {
		router.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Skip auth for health check and metrics
				if r.URL.Path == "/health" || r.URL.Path == "/metrics" {
					next.ServeHTTP(w, r)
					return
				}
				authMiddleware.AuthenticateRequest(next.ServeHTTP)(w, r)
			})
		})
	}

	// Setup FHIR routes
	fhirProxy.SetupRoutes(router)

	// Setup metrics endpoint
	if cfg.Monitoring.EnableMetrics {
		router.Handle("/metrics", promhttp.Handler()).Methods("GET")
	}

	// Create HTTP server
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:      router,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
	}

	// Start server in a goroutine
	go func() {
		log.Printf("FHIR Validation Proxy starting on port %d", cfg.Server.Port)
		log.Printf("Google Cloud Project: %s", cfg.GoogleCloud.ProjectID)
		log.Printf("FHIR Store URL: %s", cfg.GetFHIRStoreURL())
		log.Printf("Authentication required: %v", cfg.Security.RequireAuthentication)
		log.Printf("Audit logging enabled: %v", cfg.Security.AuditLogging)
		
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed to start: %v", err)
		}
	}()

	// Start metrics server if enabled
	if cfg.Monitoring.EnableMetrics && cfg.Monitoring.MetricsPort != cfg.Server.Port {
		metricsRouter := mux.NewRouter()
		metricsRouter.Handle("/metrics", promhttp.Handler()).Methods("GET")
		metricsSrv := &http.Server{
			Addr:    fmt.Sprintf(":%d", cfg.Monitoring.MetricsPort),
			Handler: metricsRouter,
		}

		go func() {
			log.Printf("Metrics server starting on port %d", cfg.Monitoring.MetricsPort)
			if err := metricsSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Printf("Metrics server failed: %v", err)
			}
		}()
	}

	// Wait for interrupt signal to gracefully shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal("Server forced to shutdown:", err)
	}

	log.Println("Server exited")
}
