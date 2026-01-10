package services

import (
	"context"

	v1 "github.com/banglin/go-nd/gen/go_nd/v1"
	"github.com/banglin/go-nd/internal/models"
	"github.com/banglin/go-nd/internal/services"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// JobsServiceServer implements the gRPC JobsService.
type JobsServiceServer struct {
	v1.UnimplementedJobsServiceServer
	svc    *services.JobService
	logger *zap.Logger
}

// RegisterJobsService registers the JobsService with the gRPC server.
func RegisterJobsService(server *grpc.Server, svc *services.JobService, logger *zap.Logger) {
	v1.RegisterJobsServiceServer(server, &JobsServiceServer{
		svc:    svc,
		logger: logger,
	})
}

// SubmitJob creates a new job and provisions security groups.
func (s *JobsServiceServer) SubmitJob(ctx context.Context, req *v1.SubmitJobRequest) (*v1.SubmitJobResponse, error) {
	if req.SlurmJobId == "" {
		return nil, status.Error(codes.InvalidArgument, "slurm_job_id is required")
	}
	if len(req.ComputeNodes) == 0 {
		return nil, status.Error(codes.InvalidArgument, "compute_nodes is required")
	}

	result, err := s.svc.Provision(ctx, services.ProvisionInput{
		SlurmJobID:   req.SlurmJobId,
		Name:         req.Name,
		Tenant:       req.Tenant,
		ComputeNodes: req.ComputeNodes,
	})
	if err != nil {
		return nil, mapError(err)
	}

	return &v1.SubmitJobResponse{
		Job:     jobToProto(result.Job),
		Created: result.Created,
	}, nil
}

// GetJob retrieves a job by Slurm job ID.
func (s *JobsServiceServer) GetJob(ctx context.Context, req *v1.GetJobRequest) (*v1.GetJobResponse, error) {
	if req.SlurmJobId == "" {
		return nil, status.Error(codes.InvalidArgument, "slurm_job_id is required")
	}

	job, err := s.svc.GetJob(ctx, req.SlurmJobId)
	if err != nil {
		return nil, mapError(err)
	}

	return &v1.GetJobResponse{
		Job: jobToProto(job),
	}, nil
}

// ListJobs lists jobs with optional filtering.
func (s *JobsServiceServer) ListJobs(ctx context.Context, req *v1.ListJobsRequest) (*v1.ListJobsResponse, error) {
	// Determine status filter - use first status if multiple provided
	statusFilter := ""
	if len(req.Statuses) > 0 {
		statusFilter = protoStatusToModel(req.Statuses[0])
	}

	jobs, err := s.svc.ListJobs(ctx, statusFilter)
	if err != nil {
		return nil, mapError(err)
	}

	// Filter by fabric if specified
	var filtered []models.Job
	if req.FabricName != "" {
		for _, j := range jobs {
			if j.FabricName == req.FabricName {
				filtered = append(filtered, j)
			}
		}
	} else {
		filtered = jobs
	}

	protoJobs := make([]*v1.Job, len(filtered))
	for i := range filtered {
		protoJobs[i] = jobToProto(&filtered[i])
	}

	return &v1.ListJobsResponse{
		Jobs: protoJobs,
	}, nil
}

// CompleteJob marks a job as completed.
func (s *JobsServiceServer) CompleteJob(ctx context.Context, req *v1.CompleteJobRequest) (*v1.CompleteJobResponse, error) {
	if req.SlurmJobId == "" {
		return nil, status.Error(codes.InvalidArgument, "slurm_job_id is required")
	}

	// Get the job first
	job, err := s.svc.GetJob(ctx, req.SlurmJobId)
	if err != nil {
		return nil, mapError(err)
	}

	// Deprovision the job
	if err := s.svc.Deprovision(ctx, job); err != nil {
		return nil, mapError(err)
	}

	// Fetch updated job
	job, err = s.svc.GetJob(ctx, req.SlurmJobId)
	if err != nil {
		return nil, mapError(err)
	}

	return &v1.CompleteJobResponse{
		Job: jobToProto(job),
	}, nil
}

// CleanupExpiredJobs removes expired jobs.
func (s *JobsServiceServer) CleanupExpiredJobs(ctx context.Context, req *v1.CleanupExpiredJobsRequest) (*v1.CleanupExpiredJobsResponse, error) {
	cleanedJobIDs, err := s.svc.CleanupExpiredJobs(ctx)
	if err != nil {
		return nil, mapError(err)
	}

	return &v1.CleanupExpiredJobsResponse{
		CleanedCount:  int32(len(cleanedJobIDs)),
		CleanedJobIds: cleanedJobIDs,
	}, nil
}

// jobToProto converts a models.Job to a proto Job message.
func jobToProto(j *models.Job) *v1.Job {
	if j == nil {
		return nil
	}

	job := &v1.Job{
		Id:           j.ID,
		SlurmJobId:   j.SlurmJobID,
		Name:         j.Name,
		TenantKey:    j.TenantKey,
		Status:       modelStatusToProto(j.Status),
		FabricName:   j.FabricName,
		VrfName:      j.VRFName,
		ContractName: j.ContractName,
		SubmittedAt:  timestamppb.New(j.SubmittedAt),
	}

	if j.ErrorMessage != nil {
		job.ErrorMessage = *j.ErrorMessage
	}
	if j.ProvisionedAt != nil {
		job.ProvisionedAt = timestamppb.New(*j.ProvisionedAt)
	}
	if j.CompletedAt != nil {
		job.CompletedAt = timestamppb.New(*j.CompletedAt)
	}
	if j.ExpiresAt != nil {
		job.ExpiresAt = timestamppb.New(*j.ExpiresAt)
	}
	if j.SecurityGroupID != nil {
		job.SecurityGroupId = *j.SecurityGroupID
	}

	// Convert compute nodes
	for _, cn := range j.ComputeNodes {
		node := &v1.JobComputeNode{
			Id:            cn.ID,
			JobId:         cn.JobID,
			ComputeNodeId: cn.ComputeNodeID,
		}
		if cn.ComputeNode != nil {
			node.ComputeNodeName = cn.ComputeNode.Name
		}
		job.ComputeNodes = append(job.ComputeNodes, node)
	}

	return job
}

// modelStatusToProto converts a model status string to proto enum.
func modelStatusToProto(s string) v1.JobStatus {
	switch models.JobStatus(s) {
	case models.JobStatusPending:
		return v1.JobStatus_JOB_STATUS_PENDING
	case models.JobStatusProvisioning:
		return v1.JobStatus_JOB_STATUS_PROVISIONING
	case models.JobStatusActive:
		return v1.JobStatus_JOB_STATUS_ACTIVE
	case models.JobStatusDeprovisioning:
		return v1.JobStatus_JOB_STATUS_DEPROVISIONING
	case models.JobStatusCompleted:
		return v1.JobStatus_JOB_STATUS_COMPLETED
	case models.JobStatusCleanupFailed:
		return v1.JobStatus_JOB_STATUS_CLEANUP_FAILED
	case models.JobStatusFailed:
		return v1.JobStatus_JOB_STATUS_FAILED
	default:
		return v1.JobStatus_JOB_STATUS_UNSPECIFIED
	}
}

// protoStatusToModel converts a proto enum to model status string.
func protoStatusToModel(s v1.JobStatus) string {
	switch s {
	case v1.JobStatus_JOB_STATUS_PENDING:
		return string(models.JobStatusPending)
	case v1.JobStatus_JOB_STATUS_PROVISIONING:
		return string(models.JobStatusProvisioning)
	case v1.JobStatus_JOB_STATUS_ACTIVE:
		return string(models.JobStatusActive)
	case v1.JobStatus_JOB_STATUS_DEPROVISIONING:
		return string(models.JobStatusDeprovisioning)
	case v1.JobStatus_JOB_STATUS_COMPLETED:
		return string(models.JobStatusCompleted)
	case v1.JobStatus_JOB_STATUS_CLEANUP_FAILED:
		return string(models.JobStatusCleanupFailed)
	case v1.JobStatus_JOB_STATUS_FAILED:
		return string(models.JobStatusFailed)
	default:
		return ""
	}
}

// mapError converts service errors to gRPC status errors.
func mapError(err error) error {
	if err == nil {
		return nil
	}

	// Check for common error patterns
	errStr := err.Error()

	// Not found errors
	if contains(errStr, "not found", "does not exist") {
		return status.Error(codes.NotFound, err.Error())
	}

	// Already exists / conflict errors
	if contains(errStr, "already exists", "duplicate", "conflict") {
		return status.Error(codes.AlreadyExists, err.Error())
	}

	// Invalid input errors
	if contains(errStr, "invalid", "required", "must be") {
		return status.Error(codes.InvalidArgument, err.Error())
	}

	// Default to internal error
	return status.Error(codes.Internal, err.Error())
}

// contains checks if s contains any of the substrings.
func contains(s string, substrs ...string) bool {
	for _, sub := range substrs {
		if len(s) >= len(sub) {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
		}
	}
	return false
}
