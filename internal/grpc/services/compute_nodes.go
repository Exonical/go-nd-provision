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

// ListInterfaces lists interfaces for a compute node.
func (s *ComputeNodesServiceServer) ListInterfaces(ctx context.Context, req *v1.ListInterfacesRequest) (*v1.ListInterfacesResponse, error) {
	if req.ComputeNodeId == "" {
		return nil, status.Error(codes.InvalidArgument, "compute_node_id is required")
	}

	var interfaces []models.ComputeNodeInterface
	if err := database.DB.WithContext(ctx).Where("compute_node_id = ?", req.ComputeNodeId).Find(&interfaces).Error; err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	protoInterfaces := make([]*v1.ComputeNodeInterface, len(interfaces))
	for i := range interfaces {
		protoInterfaces[i] = interfaceToProto(&interfaces[i])
	}

	return &v1.ListInterfacesResponse{
		Interfaces: protoInterfaces,
	}, nil
}

// CreateInterface creates a new interface for a compute node.
func (s *ComputeNodesServiceServer) CreateInterface(ctx context.Context, req *v1.CreateInterfaceRequest) (*v1.CreateInterfaceResponse, error) {
	if req.ComputeNodeId == "" {
		return nil, status.Error(codes.InvalidArgument, "compute_node_id is required")
	}
	if req.Role == "" {
		return nil, status.Error(codes.InvalidArgument, "role is required")
	}
	if req.Role != "compute" && req.Role != "storage" {
		return nil, status.Error(codes.InvalidArgument, "role must be 'compute' or 'storage'")
	}

	// Check node exists
	var node models.ComputeNode
	if err := database.DB.WithContext(ctx).First(&node, "id = ?", req.ComputeNodeId).Error; err != nil {
		return nil, status.Error(codes.NotFound, "compute node not found")
	}

	// Check max 2 interfaces
	var count int64
	database.DB.Model(&models.ComputeNodeInterface{}).Where("compute_node_id = ?", req.ComputeNodeId).Count(&count)
	if count >= 2 {
		return nil, status.Error(codes.FailedPrecondition, "node already has maximum of 2 interfaces")
	}

	// Check no duplicate role
	var existing models.ComputeNodeInterface
	if err := database.DB.WithContext(ctx).Where("compute_node_id = ? AND role = ?", req.ComputeNodeId, req.Role).First(&existing).Error; err == nil {
		return nil, status.Error(codes.AlreadyExists, "interface with this role already exists")
	}

	iface := models.ComputeNodeInterface{
		ID:            uuid.New().String(),
		ComputeNodeID: req.ComputeNodeId,
		Role:          models.InterfaceRole(req.Role),
		Hostname:      req.Hostname,
		IPAddress:     req.IpAddress,
		MACAddress:    req.MacAddress,
	}

	if err := database.DB.WithContext(ctx).Create(&iface).Error; err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &v1.CreateInterfaceResponse{
		Interface: interfaceToProto(&iface),
	}, nil
}

// UpdateInterface updates an existing interface.
func (s *ComputeNodesServiceServer) UpdateInterface(ctx context.Context, req *v1.UpdateInterfaceRequest) (*v1.UpdateInterfaceResponse, error) {
	if req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}

	var iface models.ComputeNodeInterface
	if err := database.DB.WithContext(ctx).First(&iface, "id = ?", req.Id).Error; err != nil {
		return nil, status.Error(codes.NotFound, "interface not found")
	}

	if req.Hostname != "" {
		iface.Hostname = req.Hostname
	}
	if req.IpAddress != "" {
		iface.IPAddress = req.IpAddress
	}
	if req.MacAddress != "" {
		iface.MACAddress = req.MacAddress
	}

	if err := database.DB.WithContext(ctx).Save(&iface).Error; err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &v1.UpdateInterfaceResponse{
		Interface: interfaceToProto(&iface),
	}, nil
}

// DeleteInterface deletes an interface.
func (s *ComputeNodesServiceServer) DeleteInterface(ctx context.Context, req *v1.DeleteInterfaceRequest) (*v1.DeleteInterfaceResponse, error) {
	if req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}

	if err := database.DB.WithContext(ctx).Delete(&models.ComputeNodeInterface{}, "id = ?", req.Id).Error; err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &v1.DeleteInterfaceResponse{}, nil
}

// AssignPortToInterface assigns a port mapping to an interface.
func (s *ComputeNodesServiceServer) AssignPortToInterface(ctx context.Context, req *v1.AssignPortToInterfaceRequest) (*v1.AssignPortToInterfaceResponse, error) {
	if req.ComputeNodeId == "" || req.PortMappingId == "" {
		return nil, status.Error(codes.InvalidArgument, "compute_node_id and port_mapping_id are required")
	}

	var mapping models.ComputeNodePortMapping
	if err := database.DB.WithContext(ctx).Where("id = ? AND compute_node_id = ?", req.PortMappingId, req.ComputeNodeId).First(&mapping).Error; err != nil {
		return nil, status.Error(codes.NotFound, "port mapping not found")
	}

	if req.InterfaceId != "" {
		// Validate interface exists and belongs to node
		var iface models.ComputeNodeInterface
		if err := database.DB.WithContext(ctx).Where("id = ? AND compute_node_id = ?", req.InterfaceId, req.ComputeNodeId).First(&iface).Error; err != nil {
			return nil, status.Error(codes.NotFound, "interface not found or doesn't belong to this node")
		}
		// Check interface doesn't already have a port
		var existingMapping models.ComputeNodePortMapping
		if err := database.DB.WithContext(ctx).Where("interface_id = ? AND id != ?", req.InterfaceId, req.PortMappingId).First(&existingMapping).Error; err == nil {
			return nil, status.Error(codes.FailedPrecondition, "interface already has a port assigned")
		}
		mapping.InterfaceID = &req.InterfaceId
	} else {
		mapping.InterfaceID = nil
	}

	if err := database.DB.WithContext(ctx).Save(&mapping).Error; err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	database.DB.Preload("SwitchPort.Switch").First(&mapping, "id = ?", mapping.ID)

	return &v1.AssignPortToInterfaceResponse{
		PortMapping: portMappingToProto(&mapping),
	}, nil
}

// BulkAssignPortMappings assigns multiple ports to nodes/interfaces.
func (s *ComputeNodesServiceServer) BulkAssignPortMappings(ctx context.Context, req *v1.BulkAssignPortMappingsRequest) (*v1.BulkAssignPortMappingsResponse, error) {
	results := make([]*v1.BulkAssignmentResult, 0, len(req.Assignments))

	for _, assignment := range req.Assignments {
		result := &v1.BulkAssignmentResult{
			SwitchPortId: assignment.SwitchPortId,
		}

		// Find existing mapping
		var mapping models.ComputeNodePortMapping
		err := database.DB.WithContext(ctx).Where("switch_port_id = ?", assignment.SwitchPortId).First(&mapping).Error

		if assignment.NodeId == "" {
			// Unassign
			if err == nil {
				if err := database.DB.WithContext(ctx).Delete(&mapping).Error; err != nil {
					result.Success = false
					result.Error = err.Error()
				} else {
					result.Success = true
					result.Action = "deleted"
				}
			} else {
				result.Success = true
				result.Action = "no_change"
			}
		} else {
			// Assign to node
			var node models.ComputeNode
			if err := database.DB.WithContext(ctx).Where("id = ? OR name = ?", assignment.NodeId, assignment.NodeId).First(&node).Error; err != nil {
				result.Success = false
				result.Error = "node not found"
				results = append(results, result)
				continue
			}

			var interfaceID *string
			if assignment.InterfaceId != "" {
				var iface models.ComputeNodeInterface
				if err := database.DB.WithContext(ctx).Where("id = ? AND compute_node_id = ?", assignment.InterfaceId, node.ID).First(&iface).Error; err != nil {
					result.Success = false
					result.Error = "interface not found or doesn't belong to this node"
					results = append(results, result)
					continue
				}
				// Check interface doesn't already have a port
				var existingMapping models.ComputeNodePortMapping
				if err := database.DB.WithContext(ctx).Where("interface_id = ? AND switch_port_id != ?", assignment.InterfaceId, assignment.SwitchPortId).First(&existingMapping).Error; err == nil {
					result.Success = false
					result.Error = "interface already has a port assigned"
					results = append(results, result)
					continue
				}
				interfaceID = &assignment.InterfaceId
			}

			if err == nil {
				// Update existing
				mapping.ComputeNodeID = node.ID
				mapping.InterfaceID = interfaceID
				if err := database.DB.WithContext(ctx).Save(&mapping).Error; err != nil {
					result.Success = false
					result.Error = err.Error()
				} else {
					result.Success = true
					result.Action = "updated"
					result.MappingId = mapping.ID
				}
			} else {
				// Create new
				mapping = models.ComputeNodePortMapping{
					ID:            uuid.New().String(),
					ComputeNodeID: node.ID,
					SwitchPortID:  assignment.SwitchPortId,
					InterfaceID:   interfaceID,
				}
				if err := database.DB.WithContext(ctx).Create(&mapping).Error; err != nil {
					result.Success = false
					result.Error = err.Error()
				} else {
					result.Success = true
					result.Action = "created"
					result.MappingId = mapping.ID
				}
			}
		}
		results = append(results, result)
	}

	return &v1.BulkAssignPortMappingsResponse{
		Results: results,
		Total:   int32(len(results)),
	}, nil
}

// interfaceToProto converts a models.ComputeNodeInterface to proto.
func interfaceToProto(i *models.ComputeNodeInterface) *v1.ComputeNodeInterface {
	if i == nil {
		return nil
	}
	return &v1.ComputeNodeInterface{
		Id:            i.ID,
		ComputeNodeId: i.ComputeNodeID,
		Role:          string(i.Role),
		Hostname:      i.Hostname,
		IpAddress:     i.IPAddress,
		MacAddress:    i.MACAddress,
		CreatedAt:     timestamppb.New(i.CreatedAt),
		UpdatedAt:     timestamppb.New(i.UpdatedAt),
	}
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
