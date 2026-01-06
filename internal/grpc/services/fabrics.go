package services

import (
	"context"

	v1 "github.com/banglin/go-nd/gen/go_nd/v1"
	"github.com/banglin/go-nd/internal/cache"
	"github.com/banglin/go-nd/internal/database"
	"github.com/banglin/go-nd/internal/models"
	"github.com/banglin/go-nd/internal/ndclient"
	"github.com/banglin/go-nd/internal/sync"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// FabricsServiceServer implements the gRPC FabricsService.
type FabricsServiceServer struct {
	v1.UnimplementedFabricsServiceServer
	ndClient *ndclient.Client
	logger   *zap.Logger
}

// RegisterFabricsService registers the FabricsService with the gRPC server.
func RegisterFabricsService(server *grpc.Server, ndClient *ndclient.Client, logger *zap.Logger) {
	v1.RegisterFabricsServiceServer(server, &FabricsServiceServer{
		ndClient: ndClient,
		logger:   logger,
	})
}

// ListFabrics lists all fabrics.
func (s *FabricsServiceServer) ListFabrics(ctx context.Context, req *v1.ListFabricsRequest) (*v1.ListFabricsResponse, error) {
	var fabrics []models.Fabric
	if err := database.DB.WithContext(ctx).Find(&fabrics).Error; err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	protoFabrics := make([]*v1.Fabric, len(fabrics))
	for i := range fabrics {
		protoFabrics[i] = fabricToProto(&fabrics[i])
	}

	return &v1.ListFabricsResponse{
		Fabrics: protoFabrics,
	}, nil
}

// GetFabric retrieves a fabric by ID.
func (s *FabricsServiceServer) GetFabric(ctx context.Context, req *v1.GetFabricRequest) (*v1.GetFabricResponse, error) {
	if req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}

	var fabric models.Fabric
	if err := database.DB.WithContext(ctx).Preload("Switches").First(&fabric, "id = ?", req.Id).Error; err != nil {
		return nil, status.Error(codes.NotFound, "fabric not found")
	}

	return &v1.GetFabricResponse{
		Fabric: fabricToProto(&fabric),
	}, nil
}

// CreateFabric creates a new fabric.
func (s *FabricsServiceServer) CreateFabric(ctx context.Context, req *v1.CreateFabricRequest) (*v1.CreateFabricResponse, error) {
	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}

	fabric := models.Fabric{
		ID:   uuid.New().String(),
		Name: req.Name,
		Type: req.Type,
	}

	if err := database.DB.WithContext(ctx).Create(&fabric).Error; err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &v1.CreateFabricResponse{
		Fabric: fabricToProto(&fabric),
	}, nil
}

// SyncFabrics syncs fabrics from Nexus Dashboard.
func (s *FabricsServiceServer) SyncFabrics(ctx context.Context, req *v1.SyncFabricsRequest) (*v1.SyncFabricsResponse, error) {
	if s.ndClient == nil {
		return nil, status.Error(codes.FailedPrecondition, "Nexus Dashboard client not configured")
	}

	result, err := sync.SyncFabrics(ctx, database.DB, s.ndClient.LANFabric())
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	// Fetch synced fabrics
	var fabrics []models.Fabric
	database.DB.WithContext(ctx).Find(&fabrics)

	protoFabrics := make([]*v1.Fabric, len(fabrics))
	for i := range fabrics {
		protoFabrics[i] = fabricToProto(&fabrics[i])
	}

	return &v1.SyncFabricsResponse{
		SyncedCount: int32(result.Synced),
		Fabrics:     protoFabrics,
	}, nil
}

// ListSwitches lists switches in a fabric.
func (s *FabricsServiceServer) ListSwitches(ctx context.Context, req *v1.ListSwitchesRequest) (*v1.ListSwitchesResponse, error) {
	if req.FabricId == "" {
		return nil, status.Error(codes.InvalidArgument, "fabric_id is required")
	}

	var switches []models.Switch
	if err := database.DB.WithContext(ctx).Where("fabric_id = ?", req.FabricId).Find(&switches).Error; err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	protoSwitches := make([]*v1.Switch, len(switches))
	for i := range switches {
		protoSwitches[i] = switchToProto(&switches[i])
	}

	return &v1.ListSwitchesResponse{
		Switches: protoSwitches,
	}, nil
}

// GetSwitch retrieves a switch by ID.
func (s *FabricsServiceServer) GetSwitch(ctx context.Context, req *v1.GetSwitchRequest) (*v1.GetSwitchResponse, error) {
	if req.SwitchId == "" {
		return nil, status.Error(codes.InvalidArgument, "switch_id is required")
	}

	var sw models.Switch
	if err := database.DB.WithContext(ctx).Preload("Ports").First(&sw, "id = ?", req.SwitchId).Error; err != nil {
		return nil, status.Error(codes.NotFound, "switch not found")
	}

	return &v1.GetSwitchResponse{
		Switch: switchToProto(&sw),
	}, nil
}

// CreateSwitch creates a new switch.
func (s *FabricsServiceServer) CreateSwitch(ctx context.Context, req *v1.CreateSwitchRequest) (*v1.CreateSwitchResponse, error) {
	if req.FabricId == "" {
		return nil, status.Error(codes.InvalidArgument, "fabric_id is required")
	}
	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}
	if req.SerialNumber == "" {
		return nil, status.Error(codes.InvalidArgument, "serial_number is required")
	}

	sw := models.Switch{
		ID:           uuid.New().String(),
		Name:         req.Name,
		SerialNumber: req.SerialNumber,
		Model:        req.Model,
		IPAddress:    req.IpAddress,
		FabricID:     req.FabricId,
	}

	if err := database.DB.WithContext(ctx).Create(&sw).Error; err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &v1.CreateSwitchResponse{
		Switch: switchToProto(&sw),
	}, nil
}

// SyncSwitches syncs switches from Nexus Dashboard.
func (s *FabricsServiceServer) SyncSwitches(ctx context.Context, req *v1.SyncSwitchesRequest) (*v1.SyncSwitchesResponse, error) {
	if req.FabricId == "" {
		return nil, status.Error(codes.InvalidArgument, "fabric_id is required")
	}
	if s.ndClient == nil {
		return nil, status.Error(codes.FailedPrecondition, "Nexus Dashboard client not configured")
	}

	var fabric models.Fabric
	if err := database.DB.WithContext(ctx).First(&fabric, "id = ?", req.FabricId).Error; err != nil {
		return nil, status.Error(codes.NotFound, "fabric not found")
	}

	result, err := sync.SyncFabricSwitches(ctx, database.DB, s.ndClient.LANFabric(), &fabric)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	// Fetch synced switches
	var switches []models.Switch
	database.DB.WithContext(ctx).Where("fabric_id = ?", req.FabricId).Find(&switches)

	protoSwitches := make([]*v1.Switch, len(switches))
	for i := range switches {
		protoSwitches[i] = switchToProto(&switches[i])
	}

	return &v1.SyncSwitchesResponse{
		SyncedCount: int32(result.Synced),
		Switches:    protoSwitches,
	}, nil
}

// ListNetworks lists networks in a fabric.
func (s *FabricsServiceServer) ListNetworks(ctx context.Context, req *v1.ListNetworksRequest) (*v1.ListNetworksResponse, error) {
	if req.FabricId == "" {
		return nil, status.Error(codes.InvalidArgument, "fabric_id is required")
	}
	if s.ndClient == nil {
		return nil, status.Error(codes.FailedPrecondition, "Nexus Dashboard client not configured")
	}

	networks, err := s.ndClient.LANFabric().GetNetworksNDFC(ctx, req.FabricId)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	protoNetworks := make([]*v1.Network, len(networks))
	for i, n := range networks {
		protoNetworks[i] = &v1.Network{
			Name:   getStringFromMap(n, "networkName"),
			Fabric: getStringFromMap(n, "fabric"),
			Vrf:    getStringFromMap(n, "vrf"),
			VlanId: int32(getIntFromMap(n, "vlanId")),
		}
	}

	return &v1.ListNetworksResponse{
		Networks: protoNetworks,
	}, nil
}

// ListPorts lists ports on a switch.
func (s *FabricsServiceServer) ListPorts(ctx context.Context, req *v1.ListPortsRequest) (*v1.ListPortsResponse, error) {
	if req.SwitchId == "" {
		return nil, status.Error(codes.InvalidArgument, "switch_id is required")
	}

	var ports []models.SwitchPort
	if err := database.DB.WithContext(ctx).Where("switch_id = ?", req.SwitchId).Find(&ports).Error; err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	protoPorts := make([]*v1.SwitchPort, len(ports))
	for i := range ports {
		protoPorts[i] = switchPortToProto(&ports[i])
	}

	return &v1.ListPortsResponse{
		Ports: protoPorts,
	}, nil
}

// GetPort retrieves a port by ID.
func (s *FabricsServiceServer) GetPort(ctx context.Context, req *v1.GetPortRequest) (*v1.GetPortResponse, error) {
	if req.PortId == "" {
		return nil, status.Error(codes.InvalidArgument, "port_id is required")
	}

	var port models.SwitchPort
	if err := database.DB.WithContext(ctx).First(&port, "id = ?", req.PortId).Error; err != nil {
		return nil, status.Error(codes.NotFound, "port not found")
	}

	return &v1.GetPortResponse{
		Port: switchPortToProto(&port),
	}, nil
}

// CreatePort creates a new port.
func (s *FabricsServiceServer) CreatePort(ctx context.Context, req *v1.CreatePortRequest) (*v1.CreatePortResponse, error) {
	if req.SwitchId == "" {
		return nil, status.Error(codes.InvalidArgument, "switch_id is required")
	}
	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}

	port := models.SwitchPort{
		ID:          uuid.New().String(),
		Name:        req.Name,
		PortNumber:  req.PortNumber,
		Description: req.Description,
		IsPresent:   true,
		SwitchID:    req.SwitchId,
	}

	if err := database.DB.WithContext(ctx).Create(&port).Error; err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &v1.CreatePortResponse{
		Port: switchPortToProto(&port),
	}, nil
}

// SyncPorts syncs ports from Nexus Dashboard.
func (s *FabricsServiceServer) SyncPorts(ctx context.Context, req *v1.SyncPortsRequest) (*v1.SyncPortsResponse, error) {
	if req.SwitchId == "" {
		return nil, status.Error(codes.InvalidArgument, "switch_id is required")
	}
	if s.ndClient == nil {
		return nil, status.Error(codes.FailedPrecondition, "Nexus Dashboard client not configured")
	}

	var sw models.Switch
	if err := database.DB.WithContext(ctx).Preload("Fabric").First(&sw, "id = ?", req.SwitchId).Error; err != nil {
		return nil, status.Error(codes.NotFound, "switch not found")
	}

	// Get uplink ports to exclude
	var uplinks map[string]bool
	if sw.Fabric != nil {
		uplinks = sync.GetUplinksWithCache(ctx, s.ndClient.LANFabric(), sw.Fabric.Name, cache.Client)
	} else {
		uplinks = make(map[string]bool)
	}

	result, err := sync.SyncSwitchPorts(ctx, database.DB, s.ndClient.LANFabric(), req.SwitchId, sw.SerialNumber, uplinks)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	// Fetch synced ports
	var ports []models.SwitchPort
	database.DB.WithContext(ctx).Where("switch_id = ?", req.SwitchId).Find(&ports)

	protoPorts := make([]*v1.SwitchPort, len(ports))
	for i := range ports {
		protoPorts[i] = switchPortToProto(&ports[i])
	}

	return &v1.SyncPortsResponse{
		SyncedCount: int32(result.Synced),
		Ports:       protoPorts,
	}, nil
}

// DeletePorts deletes ports from a switch.
func (s *FabricsServiceServer) DeletePorts(ctx context.Context, req *v1.DeletePortsRequest) (*v1.DeletePortsResponse, error) {
	if req.SwitchId == "" {
		return nil, status.Error(codes.InvalidArgument, "switch_id is required")
	}

	query := database.DB.WithContext(ctx).Where("switch_id = ?", req.SwitchId)
	if len(req.PortIds) > 0 {
		query = query.Where("id IN ?", req.PortIds)
	}

	result := query.Delete(&models.SwitchPort{})
	if result.Error != nil {
		return nil, status.Error(codes.Internal, result.Error.Error())
	}

	return &v1.DeletePortsResponse{
		DeletedCount: int32(result.RowsAffected),
	}, nil
}

// fabricToProto converts a models.Fabric to proto.
func fabricToProto(f *models.Fabric) *v1.Fabric {
	if f == nil {
		return nil
	}

	return &v1.Fabric{
		Id:          f.ID,
		Name:        f.Name,
		Type:        f.Type,
		CreatedAt:   timestamppb.New(f.CreatedAt),
		UpdatedAt:   timestamppb.New(f.UpdatedAt),
		SwitchCount: int32(len(f.Switches)),
	}
}

// switchToProto converts a models.Switch to proto.
func switchToProto(sw *models.Switch) *v1.Switch {
	if sw == nil {
		return nil
	}

	return &v1.Switch{
		Id:           sw.ID,
		Name:         sw.Name,
		SerialNumber: sw.SerialNumber,
		Model:        sw.Model,
		IpAddress:    sw.IPAddress,
		FabricId:     sw.FabricID,
		CreatedAt:    timestamppb.New(sw.CreatedAt),
		UpdatedAt:    timestamppb.New(sw.UpdatedAt),
		PortCount:    int32(len(sw.Ports)),
	}
}

// switchPortToProto converts a models.SwitchPort to proto.
func switchPortToProto(p *models.SwitchPort) *v1.SwitchPort {
	if p == nil {
		return nil
	}

	port := &v1.SwitchPort{
		Id:          p.ID,
		Name:        p.Name,
		PortNumber:  p.PortNumber,
		Description: p.Description,
		AdminState:  p.AdminState,
		Speed:       p.Speed,
		IsPresent:   p.IsPresent,
		SwitchId:    p.SwitchID,
		CreatedAt:   timestamppb.New(p.CreatedAt),
		UpdatedAt:   timestamppb.New(p.UpdatedAt),
	}

	if p.LastSeenAt != nil {
		port.LastSeenAt = timestamppb.New(*p.LastSeenAt)
	}

	return port
}

// getStringFromMap safely extracts a string from a map.
func getStringFromMap(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// getIntFromMap safely extracts an int from a map.
func getIntFromMap(m map[string]interface{}, key string) int {
	if v, ok := m[key]; ok {
		switch n := v.(type) {
		case int:
			return n
		case float64:
			return int(n)
		case int64:
			return int(n)
		}
	}
	return 0
}
