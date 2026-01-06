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

// ComputeNodesServiceServer implements the gRPC ComputeNodesService.
type ComputeNodesServiceServer struct {
	v1.UnimplementedComputeNodesServiceServer
	logger *zap.Logger
}

// RegisterComputeNodesService registers the ComputeNodesService with the gRPC server.
func RegisterComputeNodesService(server *grpc.Server, logger *zap.Logger) {
	v1.RegisterComputeNodesServiceServer(server, &ComputeNodesServiceServer{
		logger: logger,
	})
}

// ListComputeNodes lists all compute nodes.
func (s *ComputeNodesServiceServer) ListComputeNodes(ctx context.Context, req *v1.ListComputeNodesRequest) (*v1.ListComputeNodesResponse, error) {
	var nodes []models.ComputeNode
	if err := database.DB.WithContext(ctx).Preload("PortMappings.SwitchPort.Switch").Find(&nodes).Error; err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	protoNodes := make([]*v1.ComputeNode, len(nodes))
	for i := range nodes {
		protoNodes[i] = computeNodeToProto(&nodes[i])
	}

	return &v1.ListComputeNodesResponse{
		ComputeNodes: protoNodes,
	}, nil
}

// GetComputeNode retrieves a compute node by ID.
func (s *ComputeNodesServiceServer) GetComputeNode(ctx context.Context, req *v1.GetComputeNodeRequest) (*v1.GetComputeNodeResponse, error) {
	if req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}

	var node models.ComputeNode
	if err := database.DB.WithContext(ctx).Preload("PortMappings.SwitchPort.Switch").First(&node, "id = ?", req.Id).Error; err != nil {
		return nil, status.Error(codes.NotFound, "compute node not found")
	}

	return &v1.GetComputeNodeResponse{
		ComputeNode: computeNodeToProto(&node),
	}, nil
}

// CreateComputeNode creates a new compute node.
func (s *ComputeNodesServiceServer) CreateComputeNode(ctx context.Context, req *v1.CreateComputeNodeRequest) (*v1.CreateComputeNodeResponse, error) {
	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}

	node := models.ComputeNode{
		ID:          uuid.New().String(),
		Name:        req.Name,
		Hostname:    req.Hostname,
		IPAddress:   req.IpAddress,
		MACAddress:  req.MacAddress,
		Description: req.Description,
	}

	if err := database.DB.WithContext(ctx).Create(&node).Error; err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &v1.CreateComputeNodeResponse{
		ComputeNode: computeNodeToProto(&node),
	}, nil
}

// UpdateComputeNode updates an existing compute node.
func (s *ComputeNodesServiceServer) UpdateComputeNode(ctx context.Context, req *v1.UpdateComputeNodeRequest) (*v1.UpdateComputeNodeResponse, error) {
	if req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}

	var node models.ComputeNode
	if err := database.DB.WithContext(ctx).First(&node, "id = ?", req.Id).Error; err != nil {
		return nil, status.Error(codes.NotFound, "compute node not found")
	}

	if req.Name != "" {
		node.Name = req.Name
	}
	if req.Hostname != "" {
		node.Hostname = req.Hostname
	}
	if req.IpAddress != "" {
		node.IPAddress = req.IpAddress
	}
	if req.MacAddress != "" {
		node.MACAddress = req.MacAddress
	}
	if req.Description != "" {
		node.Description = req.Description
	}

	if err := database.DB.WithContext(ctx).Save(&node).Error; err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &v1.UpdateComputeNodeResponse{
		ComputeNode: computeNodeToProto(&node),
	}, nil
}

// DeleteComputeNode deletes a compute node.
func (s *ComputeNodesServiceServer) DeleteComputeNode(ctx context.Context, req *v1.DeleteComputeNodeRequest) (*v1.DeleteComputeNodeResponse, error) {
	if req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}

	if err := database.DB.WithContext(ctx).Delete(&models.ComputeNode{}, "id = ?", req.Id).Error; err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &v1.DeleteComputeNodeResponse{}, nil
}

// ListPortMappings lists port mappings for a compute node.
func (s *ComputeNodesServiceServer) ListPortMappings(ctx context.Context, req *v1.ListPortMappingsRequest) (*v1.ListPortMappingsResponse, error) {
	if req.ComputeNodeId == "" {
		return nil, status.Error(codes.InvalidArgument, "compute_node_id is required")
	}

	var mappings []models.ComputeNodePortMapping
	if err := database.DB.WithContext(ctx).
		Preload("SwitchPort.Switch").
		Where("compute_node_id = ?", req.ComputeNodeId).
		Find(&mappings).Error; err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	protoMappings := make([]*v1.PortMapping, len(mappings))
	for i := range mappings {
		protoMappings[i] = portMappingToProto(&mappings[i])
	}

	return &v1.ListPortMappingsResponse{
		PortMappings: protoMappings,
	}, nil
}

// AddPortMapping adds a port mapping to a compute node.
func (s *ComputeNodesServiceServer) AddPortMapping(ctx context.Context, req *v1.AddPortMappingRequest) (*v1.AddPortMappingResponse, error) {
	if req.ComputeNodeId == "" {
		return nil, status.Error(codes.InvalidArgument, "compute_node_id is required")
	}
	if req.SwitchPortId == "" {
		return nil, status.Error(codes.InvalidArgument, "switch_port_id is required")
	}

	// Verify compute node exists
	var node models.ComputeNode
	if err := database.DB.WithContext(ctx).First(&node, "id = ?", req.ComputeNodeId).Error; err != nil {
		return nil, status.Error(codes.NotFound, "compute node not found")
	}

	// Verify switch port exists
	var port models.SwitchPort
	if err := database.DB.WithContext(ctx).Preload("Switch").First(&port, "id = ?", req.SwitchPortId).Error; err != nil {
		return nil, status.Error(codes.NotFound, "switch port not found")
	}

	mapping := models.ComputeNodePortMapping{
		ID:            uuid.New().String(),
		ComputeNodeID: req.ComputeNodeId,
		SwitchPortID:  req.SwitchPortId,
		NICName:       req.NicName,
		VLAN:          int(req.Vlan),
	}

	if err := database.DB.WithContext(ctx).Create(&mapping).Error; err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	// Reload with associations
	database.DB.WithContext(ctx).Preload("SwitchPort.Switch").First(&mapping, "id = ?", mapping.ID)

	return &v1.AddPortMappingResponse{
		PortMapping: portMappingToProto(&mapping),
	}, nil
}

// DeletePortMapping removes a port mapping.
func (s *ComputeNodesServiceServer) DeletePortMapping(ctx context.Context, req *v1.DeletePortMappingRequest) (*v1.DeletePortMappingResponse, error) {
	if req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}

	if err := database.DB.WithContext(ctx).Delete(&models.ComputeNodePortMapping{}, "id = ?", req.Id).Error; err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &v1.DeletePortMappingResponse{}, nil
}

// computeNodeToProto converts a models.ComputeNode to proto.
func computeNodeToProto(n *models.ComputeNode) *v1.ComputeNode {
	if n == nil {
		return nil
	}

	node := &v1.ComputeNode{
		Id:          n.ID,
		Name:        n.Name,
		Hostname:    n.Hostname,
		IpAddress:   n.IPAddress,
		MacAddress:  n.MACAddress,
		Description: n.Description,
		CreatedAt:   timestamppb.New(n.CreatedAt),
		UpdatedAt:   timestamppb.New(n.UpdatedAt),
	}

	for _, m := range n.PortMappings {
		node.PortMappings = append(node.PortMappings, portMappingToProto(&m))
	}

	return node
}

// portMappingToProto converts a models.ComputeNodePortMapping to proto.
func portMappingToProto(m *models.ComputeNodePortMapping) *v1.PortMapping {
	if m == nil {
		return nil
	}

	mapping := &v1.PortMapping{
		Id:            m.ID,
		ComputeNodeId: m.ComputeNodeID,
		SwitchPortId:  m.SwitchPortID,
		NicName:       m.NICName,
		Vlan:          int32(m.VLAN),
		CreatedAt:     timestamppb.New(m.CreatedAt),
	}

	if m.SwitchPort != nil {
		mapping.SwitchPortName = m.SwitchPort.Name
		mapping.SwitchId = m.SwitchPort.SwitchID
		if m.SwitchPort.Switch != nil {
			mapping.SwitchName = m.SwitchPort.Switch.Name
		}
	}

	return mapping
}
