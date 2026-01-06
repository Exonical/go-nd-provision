package interceptors

import (
	"context"
	"crypto/subtle"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// AuthInterceptor validates Bearer tokens from gRPC metadata.
// Token is expected in "authorization" metadata key with "Bearer <token>" format.
type AuthInterceptor struct {
	token       []byte
	skipMethods map[string]bool // Methods that don't require auth (e.g., health checks)
}

// NewAuthInterceptor creates a new auth interceptor with the given token.
// skipMethods is a list of full method names to skip auth for (e.g., "/grpc.health.v1.Health/Check").
func NewAuthInterceptor(token string, skipMethods []string) *AuthInterceptor {
	skip := make(map[string]bool)
	for _, m := range skipMethods {
		skip[m] = true
	}
	return &AuthInterceptor{
		token:       []byte(token),
		skipMethods: skip,
	}
}

// Unary returns a unary server interceptor for authentication.
func (a *AuthInterceptor) Unary() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		if a.skipMethods[info.FullMethod] {
			return handler(ctx, req)
		}

		if err := a.authenticate(ctx); err != nil {
			return nil, err
		}

		return handler(ctx, req)
	}
}

// Stream returns a stream server interceptor for authentication.
func (a *AuthInterceptor) Stream() grpc.StreamServerInterceptor {
	return func(
		srv interface{},
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		if a.skipMethods[info.FullMethod] {
			return handler(srv, ss)
		}

		if err := a.authenticate(ss.Context()); err != nil {
			return err
		}

		return handler(srv, ss)
	}
}

// authenticate validates the token from context metadata.
func (a *AuthInterceptor) authenticate(ctx context.Context) error {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return status.Error(codes.Unauthenticated, "missing metadata")
	}

	values := md.Get("authorization")
	if len(values) == 0 {
		return status.Error(codes.Unauthenticated, "missing authorization header")
	}

	authHeader := values[0]
	if !strings.HasPrefix(authHeader, "Bearer ") {
		return status.Error(codes.Unauthenticated, "invalid authorization format")
	}

	token := []byte(strings.TrimPrefix(authHeader, "Bearer "))

	// Constant-time comparison to prevent timing attacks
	if subtle.ConstantTimeCompare(token, a.token) != 1 {
		return status.Error(codes.Unauthenticated, "invalid token")
	}

	return nil
}
