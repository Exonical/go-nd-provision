package main

import (
	"fmt"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/banglin/go-nd/internal/cache"
	"github.com/banglin/go-nd/internal/config"
	"github.com/banglin/go-nd/internal/database"
	"github.com/banglin/go-nd/internal/grpc/interceptors"
	grpcservices "github.com/banglin/go-nd/internal/grpc/services"
	"github.com/banglin/go-nd/internal/logger"
	"github.com/banglin/go-nd/internal/ndclient"
	"github.com/banglin/go-nd/internal/router"
	"github.com/banglin/go-nd/internal/services"
	backgroundsync "github.com/banglin/go-nd/internal/sync"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
)

func main() {
	// Load configuration
	cfg := config.Load()

	// Initialize logger
	if err := logger.Initialize(cfg.Server.Mode); err != nil {
		panic("Failed to initialize logger: " + err.Error())
	}
	defer logger.Sync()

	log := logger.L()

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
	} else {
		defer cache.Client.Close()
	}

	// Initialize Nexus Dashboard client
	ndClient, err := ndclient.NewClient(&cfg.NexusDashboard)
	if err != nil {
		logger.Warn("Failed to initialize Nexus Dashboard client", zap.Error(err))
	}

	// Log enabled features
	logger.Info("Feature flags",
		zap.Bool("http", cfg.Server.EnableHTTP),
		zap.Bool("grpc", cfg.Server.EnableGRPC),
		zap.Bool("sync", cfg.Server.EnableSync))

	if !cfg.Server.EnableHTTP && !cfg.Server.EnableGRPC {
		logger.Fatal("At least one of ENABLE_HTTP or ENABLE_GRPC must be true")
	}

	// WaitGroup for graceful shutdown
	var wg sync.WaitGroup

	// Start background sync worker
	var syncWorker *backgroundsync.Worker
	if cfg.Server.EnableSync && ndClient != nil {
		syncWorker = backgroundsync.NewWorker(ndClient, &cfg.NexusDashboard, cfg.Server.InstanceID)
		syncWorker.Start()
		logger.Info("Background sync worker started")
	}

	// Start HTTP server
	var httpServer *gin.Engine
	if cfg.Server.EnableHTTP {
		httpServer = router.Setup(ndClient, cfg)
		wg.Add(1)
		go func() {
			defer wg.Done()
			addr := ":" + cfg.Server.Port
			logger.Info("Starting HTTP server", zap.String("address", addr))
			if err := httpServer.Run(addr); err != nil {
				logger.Error("HTTP server error", zap.Error(err))
			}
		}()
	}

	// Start gRPC server
	var grpcServer *grpc.Server
	var healthServer *health.Server
	if cfg.Server.EnableGRPC {
		if cfg.GRPC.AuthToken == "" {
			logger.Fatal("GRPC_AUTH_TOKEN is required when ENABLE_GRPC=true")
		}

		// Create job service
		jobService := services.NewJobService(database.DB, ndClient, &cfg.NexusDashboard)

		// Create interceptors
		recoveryInterceptor := interceptors.NewRecoveryInterceptor(log)
		loggingInterceptor := interceptors.NewLoggingInterceptor(log)
		authInterceptor := interceptors.NewAuthInterceptor(cfg.GRPC.AuthToken, []string{
			"/grpc.health.v1.Health/Check",
			"/grpc.health.v1.Health/Watch",
			"/grpc.reflection.v1.ServerReflection/ServerReflectionInfo",
			"/grpc.reflection.v1alpha.ServerReflection/ServerReflectionInfo",
		})

		// Create gRPC server
		grpcServer = grpc.NewServer(
			grpc.ChainUnaryInterceptor(
				recoveryInterceptor.Unary(),
				loggingInterceptor.Unary(),
				authInterceptor.Unary(),
			),
			grpc.ChainStreamInterceptor(
				recoveryInterceptor.Stream(),
				loggingInterceptor.Stream(),
				authInterceptor.Stream(),
			),
		)

		// Register services
		grpcservices.RegisterJobsService(grpcServer, jobService, log)
		grpcservices.RegisterComputeNodesService(grpcServer, log)
		grpcservices.RegisterFabricsService(grpcServer, ndClient, log)
		grpcservices.RegisterStorageTenantsService(grpcServer, log)

		// Register health service
		healthServer = health.NewServer()
		healthpb.RegisterHealthServer(grpcServer, healthServer)
		healthServer.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)

		// Register reflection
		if cfg.GRPC.Reflection {
			reflection.Register(grpcServer)
			log.Info("gRPC reflection enabled")
		}

		// Start listening
		addr := fmt.Sprintf(":%s", cfg.GRPC.Port)
		listener, err := net.Listen("tcp", addr)
		if err != nil {
			logger.Fatal("Failed to listen for gRPC", zap.String("addr", addr), zap.Error(err))
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			logger.Info("Starting gRPC server", zap.String("address", addr))
			if err := grpcServer.Serve(listener); err != nil {
				logger.Error("gRPC server error", zap.Error(err))
			}
		}()
	}

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down servers...")

	if syncWorker != nil {
		syncWorker.Stop()
	}

	if grpcServer != nil {
		if healthServer != nil {
			healthServer.SetServingStatus("", healthpb.HealthCheckResponse_NOT_SERVING)
		}
		grpcServer.GracefulStop()
	}

	// Note: Gin doesn't have a built-in graceful shutdown in Run()
	// For production, use http.Server with Shutdown()

	wg.Wait()
	logger.Info("Server shutdown complete")
}
