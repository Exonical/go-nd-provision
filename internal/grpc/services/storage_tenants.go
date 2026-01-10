package services

import (
	"context"

	v1 "github.com/banglin/go-nd/gen/go_nd/v1"
	"github.com/banglin/go-nd/internal/database"
	"github.com/banglin/go-nd/internal/models"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// StorageTenantsServiceServer implements the gRPC StorageTenantsService.
type StorageTenantsServiceServer struct {
	v1.UnimplementedStorageTenantsServiceServer
	logger *zap.Logger
}

// RegisterStorageTenantsService registers the StorageTenantsService with the gRPC server.
func RegisterStorageTenantsService(server *grpc.Server, logger *zap.Logger) {
	v1.RegisterStorageTenantsServiceServer(server, &StorageTenantsServiceServer{
		logger: logger,
	})
}

// ListStorageTenants lists all storage tenants.
func (s *StorageTenantsServiceServer) ListStorageTenants(ctx context.Context, req *v1.ListStorageTenantsRequest) (*v1.ListStorageTenantsResponse, error) {
	var tenants []models.StorageTenant
	if err := database.DB.WithContext(ctx).Find(&tenants).Error; err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	protoTenants := make([]*v1.StorageTenant, len(tenants))
	for i := range tenants {
		protoTenants[i] = storageTenantToProto(&tenants[i])
	}

	return &v1.ListStorageTenantsResponse{
		StorageTenants: protoTenants,
	}, nil
}

// GetStorageTenant retrieves a storage tenant by key.
func (s *StorageTenantsServiceServer) GetStorageTenant(ctx context.Context, req *v1.GetStorageTenantRequest) (*v1.GetStorageTenantResponse, error) {
	if req.Key == "" {
		return nil, status.Error(codes.InvalidArgument, "key is required")
	}

	var tenant models.StorageTenant
	if err := database.DB.WithContext(ctx).First(&tenant, "key = ?", req.Key).Error; err != nil {
		return nil, status.Error(codes.NotFound, "storage tenant not found")
	}

	return &v1.GetStorageTenantResponse{
		StorageTenant: storageTenantToProto(&tenant),
	}, nil
}

// CreateStorageTenant creates a new storage tenant.
func (s *StorageTenantsServiceServer) CreateStorageTenant(ctx context.Context, req *v1.CreateStorageTenantRequest) (*v1.CreateStorageTenantResponse, error) {
	if req.Key == "" {
		return nil, status.Error(codes.InvalidArgument, "key is required")
	}
	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}

	// Check for duplicate key
	var existing models.StorageTenant
	if err := database.DB.WithContext(ctx).First(&existing, "key = ?", req.Key).Error; err == nil {
		return nil, status.Error(codes.AlreadyExists, "storage tenant with this key already exists")
	}

	tenant := models.StorageTenant{
		ID:                 uuid.New().String(),
		Key:                req.Key,
		Description:        req.Name,
		StorageNetworkName: req.StorageNetworkName,
	}

	if err := database.DB.WithContext(ctx).Create(&tenant).Error; err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &v1.CreateStorageTenantResponse{
		StorageTenant: storageTenantToProto(&tenant),
	}, nil
}

// UpdateStorageTenant updates an existing storage tenant.
func (s *StorageTenantsServiceServer) UpdateStorageTenant(ctx context.Context, req *v1.UpdateStorageTenantRequest) (*v1.UpdateStorageTenantResponse, error) {
	if req.Key == "" {
		return nil, status.Error(codes.InvalidArgument, "key is required")
	}

	var tenant models.StorageTenant
	if err := database.DB.WithContext(ctx).First(&tenant, "key = ?", req.Key).Error; err != nil {
		return nil, status.Error(codes.NotFound, "storage tenant not found")
	}

	if req.Name != "" {
		tenant.Description = req.Name
	}
	if req.StorageNetworkName != "" {
		tenant.StorageNetworkName = req.StorageNetworkName
	}

	if err := database.DB.WithContext(ctx).Save(&tenant).Error; err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &v1.UpdateStorageTenantResponse{
		StorageTenant: storageTenantToProto(&tenant),
	}, nil
}

// DeleteStorageTenant deletes a storage tenant.
func (s *StorageTenantsServiceServer) DeleteStorageTenant(ctx context.Context, req *v1.DeleteStorageTenantRequest) (*v1.DeleteStorageTenantResponse, error) {
	if req.Key == "" {
		return nil, status.Error(codes.InvalidArgument, "key is required")
	}

	if err := database.DB.WithContext(ctx).Delete(&models.StorageTenant{}, "key = ?", req.Key).Error; err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &v1.DeleteStorageTenantResponse{}, nil
}

// storageTenantToProto converts a models.StorageTenant to proto.
func storageTenantToProto(t *models.StorageTenant) *v1.StorageTenant {
	if t == nil {
		return nil
	}
	return &v1.StorageTenant{
		Id:                 t.ID,
		Key:                t.Key,
		Name:               t.Description,
		StorageNetworkName: t.StorageNetworkName,
		CreatedAt:          timestamppb.New(t.CreatedAt),
		UpdatedAt:          timestamppb.New(t.UpdatedAt),
	}
}
