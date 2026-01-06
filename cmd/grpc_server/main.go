package main

import (
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/banglin/go-nd/internal/cache"
	"github.com/banglin/go-nd/internal/config"
	"github.com/banglin/go-nd/internal/database"
	"github.com/banglin/go-nd/internal/grpc/interceptors"
	grpcservices "github.com/banglin/go-nd/internal/grpc/services"
	"github.com/banglin/go-nd/internal/logger"
	"github.com/banglin/go-nd/internal/ndclient"
	"github.com/banglin/go-nd/internal/services"

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

	// Get gRPC-specific config from environment
	grpcPort := getEnv("GRPC_PORT", "9090")
	grpcAuthToken := os.Getenv("GRPC_AUTH_TOKEN")
	if grpcAuthToken == "" {
		logger.Fatal("GRPC_AUTH_TOKEN environment variable is required")
	}

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

	logger.Info("Connected to database")

	// Create Nexus Dashboard client
	ndClient, err := ndclient.NewClient(&cfg.NexusDashboard)
	if err != nil {
		logger.Warn("Failed to create Nexus Dashboard client", zap.Error(err))
	}

	// Create job service (reuse existing service layer)
	jobService := services.NewJobService(database.DB, ndClient, &cfg.NexusDashboard)

	// Create interceptors
	recoveryInterceptor := interceptors.NewRecoveryInterceptor(log)
	loggingInterceptor := interceptors.NewLoggingInterceptor(log)
	authInterceptor := interceptors.NewAuthInterceptor(grpcAuthToken, []string{
		"/grpc.health.v1.Health/Check",
		"/grpc.health.v1.Health/Watch",
		"/grpc.reflection.v1.ServerReflection/ServerReflectionInfo",
		"/grpc.reflection.v1alpha.ServerReflection/ServerReflectionInfo",
	})

	// Create gRPC server with interceptors (order matters: recovery -> logging -> auth)
	server := grpc.NewServer(
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
	grpcservices.RegisterJobsService(server, jobService, log)
	grpcservices.RegisterComputeNodesService(server, log)
	grpcservices.RegisterFabricsService(server, ndClient, log)

	// Register health service
	healthServer := health.NewServer()
	healthpb.RegisterHealthServer(server, healthServer)
	healthServer.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)

	// Register reflection for grpcurl/grpcui (disable in production if needed)
	if getEnv("GRPC_REFLECTION", "true") == "true" {
		reflection.Register(server)
		log.Info("gRPC reflection enabled")
	}

	// Start listening
	addr := fmt.Sprintf(":%s", grpcPort)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatal("Failed to listen", zap.String("addr", addr), zap.Error(err))
	}

	// Handle graceful shutdown
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh

		log.Info("Shutting down gRPC server...")
		healthServer.SetServingStatus("", healthpb.HealthCheckResponse_NOT_SERVING)
		server.GracefulStop()
	}()

	log.Info("Starting gRPC server", zap.String("addr", addr))
	if err := server.Serve(listener); err != nil {
		log.Fatal("Failed to serve", zap.Error(err))
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
