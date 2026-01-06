package interceptors

import (
	"context"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

// LoggingInterceptor logs gRPC requests using zap.
type LoggingInterceptor struct {
	logger *zap.Logger
}

// NewLoggingInterceptor creates a new logging interceptor.
func NewLoggingInterceptor(logger *zap.Logger) *LoggingInterceptor {
	return &LoggingInterceptor{logger: logger}
}

// Unary returns a unary server interceptor for logging.
func (l *LoggingInterceptor) Unary() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		start := time.Now()

		resp, err := handler(ctx, req)

		duration := time.Since(start)
		code := status.Code(err)

		fields := []zap.Field{
			zap.String("method", info.FullMethod),
			zap.Duration("duration", duration),
			zap.String("code", code.String()),
		}

		if err != nil {
			fields = append(fields, zap.Error(err))
			l.logger.Warn("gRPC request failed", fields...)
		} else {
			l.logger.Info("gRPC request", fields...)
		}

		return resp, err
	}
}

// Stream returns a stream server interceptor for logging.
func (l *LoggingInterceptor) Stream() grpc.StreamServerInterceptor {
	return func(
		srv interface{},
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		start := time.Now()

		err := handler(srv, ss)

		duration := time.Since(start)
		code := status.Code(err)

		fields := []zap.Field{
			zap.String("method", info.FullMethod),
			zap.Duration("duration", duration),
			zap.String("code", code.String()),
			zap.Bool("stream", true),
		}

		if err != nil {
			fields = append(fields, zap.Error(err))
			l.logger.Warn("gRPC stream failed", fields...)
		} else {
			l.logger.Info("gRPC stream", fields...)
		}

		return err
	}
}
