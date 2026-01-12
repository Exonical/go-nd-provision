package main

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/banglin/go-nd/internal/cache"
	"github.com/banglin/go-nd/internal/config"
	"github.com/banglin/go-nd/internal/database"
	"github.com/banglin/go-nd/internal/logger"
	"github.com/banglin/go-nd/internal/ndclient"
	"github.com/banglin/go-nd/internal/router"
	"github.com/banglin/go-nd/internal/sync"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func main() {
	// Load configuration
	cfg := config.Load()

	// Initialize logger
	if err := logger.Initialize(cfg.Server.Mode); err != nil {
		panic("Failed to initialize logger: " + err.Error())
	}
	defer logger.Sync()

	// Set Gin mode
	gin.SetMode(cfg.Server.Mode)

	// Initialize database
	if err := database.Initialize(&cfg.Database); err != nil {
		logger.Fatal("Failed to initialize database", zap.Error(err))
	}
	defer func() {
		if err := database.Close(); err != nil {
			logger.Error("Failed to close database", zap.Error(err))
		}
	}()

	// Run migrations
	if err := database.Migrate(); err != nil {
		logger.Fatal("Failed to run migrations", zap.Error(err))
	}

	// Initialize Valkey cache
	if err := cache.Initialize(&cfg.Valkey); err != nil {
		logger.Warn("Failed to initialize Valkey cache", zap.Error(err))
		// Continue without cache - it's optional
	} else {
		defer cache.Client.Close()
	}

	// Initialize Nexus Dashboard client
	ndClient, err := ndclient.NewClient(&cfg.NexusDashboard)
	if err != nil {
		logger.Warn("Failed to initialize Nexus Dashboard client", zap.Error(err))
		// Continue without ND client for local-only operations
	}

	// Start background sync worker
	var syncWorker *sync.Worker
	if ndClient != nil {
		syncWorker = sync.NewWorker(ndClient, &cfg.NexusDashboard, cfg.Server.InstanceID)
		syncWorker.Start()
	}

	// Setup router
	r := router.Setup(ndClient, cfg)

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-quit
		logger.Info("Shutting down server...")
		if syncWorker != nil {
			syncWorker.Stop()
		}
	}()

	// Start server
	addr := ":" + cfg.Server.Port
	logger.Info("Starting server", zap.String("address", addr))
	if err := r.Run(addr); err != nil {
		logger.Fatal("Failed to start server", zap.Error(err))
	}
}
