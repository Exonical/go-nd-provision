package interceptors

import (
	"context"
	"runtime/debug"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// RecoveryInterceptor recovers from panics and returns Internal error.
type RecoveryInterceptor struct {
	logger *zap.Logger
}

// NewRecoveryInterceptor creates a new recovery interceptor.
func NewRecoveryInterceptor(logger *zap.Logger) *RecoveryInterceptor {
	return &RecoveryInterceptor{logger: logger}
}

// Unary returns a unary server interceptor for panic recovery.
func (r *RecoveryInterceptor) Unary() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (resp interface{}, err error) {
		defer func() {
			if p := recover(); p != nil {
				r.logger.Error("gRPC panic recovered",
					zap.String("method", info.FullMethod),
					zap.Any("panic", p),
					zap.String("stack", string(debug.Stack())),
				)
				err = status.Error(codes.Internal, "internal server error")
			}
		}()

		return handler(ctx, req)
	}
}

// Stream returns a stream server interceptor for panic recovery.
func (r *RecoveryInterceptor) Stream() grpc.StreamServerInterceptor {
	return func(
		srv interface{},
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) (err error) {
		defer func() {
			if p := recover(); p != nil {
				r.logger.Error("gRPC stream panic recovered",
					zap.String("method", info.FullMethod),
					zap.Any("panic", p),
					zap.String("stack", string(debug.Stack())),
				)
				err = status.Error(codes.Internal, "internal server error")
			}
		}()

		return handler(srv, ss)
	}
}
