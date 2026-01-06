package handlers

import (
	"fmt"
	"net/http"

	"github.com/banglin/go-nd/internal/config"
	"github.com/banglin/go-nd/internal/ndclient"
	"github.com/banglin/go-nd/internal/services"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// JobHandler handles HTTP requests for job operations
type JobHandler struct {
	svc *services.JobService
}

// NewJobHandler creates a new JobHandler
func NewJobHandler(db *gorm.DB, ndClient *ndclient.Client, cfg *config.NexusDashboardConfig) *JobHandler {
	return &JobHandler{
		svc: services.NewJobService(db, ndClient, cfg),
	}
}

// SubmitJobInput represents the input from Slurm when a job is submitted
type SubmitJobInput struct {
	SlurmJobID   string   `json:"slurm_job_id" binding:"required"`
	Name         string   `json:"name"`
	ComputeNodes []string `json:"compute_nodes" binding:"required"`
}

// SubmitJob handles job submission from Slurm and provisions security
func (h *JobHandler) SubmitJob(c *gin.Context) {
	var input SubmitJobInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	result, err := h.svc.Provision(c.Request.Context(), services.ProvisionInput{
		SlurmJobID:   input.SlurmJobID,
		Name:         input.Name,
		ComputeNodes: input.ComputeNodes,
	})

	if err != nil {
		// Check if it's a conflict error
		if result != nil && !result.Created {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error(), "job": result.Job})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if result.Created {
		c.JSON(http.StatusCreated, result.Job)
	} else {
		c.JSON(http.StatusOK, result.Job)
	}
}

// CompleteJob handles job completion and deprovisions security
func (h *JobHandler) CompleteJob(c *gin.Context) {
	slurmJobID := c.Param("slurm_job_id")

	job, err := h.svc.GetJob(c.Request.Context(), slurmJobID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Job not found"})
		return
	}

	if job.Status == "completed" {
		c.JSON(http.StatusOK, gin.H{"message": "Job already completed", "job": job})
		return
	}

	if err := h.svc.Deprovision(c.Request.Context(), job); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to deprovision job",
			"details": err.Error(),
			"job_id":  job.ID,
		})
		return
	}

	// Reload job to get updated state
	job, _ = h.svc.GetJob(c.Request.Context(), slurmJobID)
	c.JSON(http.StatusOK, job)
}

// GetJob retrieves a job by Slurm job ID
func (h *JobHandler) GetJob(c *gin.Context) {
	slurmJobID := c.Param("slurm_job_id")

	job, err := h.svc.GetJob(c.Request.Context(), slurmJobID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Job not found"})
		return
	}

	c.JSON(http.StatusOK, job)
}

// ListJobs lists all jobs with optional status filter
func (h *JobHandler) ListJobs(c *gin.Context) {
	status := c.Query("status")

	jobs, err := h.svc.ListJobs(c.Request.Context(), status)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, jobs)
}

// CleanupExpiredJobs finds and deprovisions expired jobs
func (h *JobHandler) CleanupExpiredJobs(c *gin.Context) {
	cleaned, err := h.svc.CleanupExpiredJobs(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":      fmt.Sprintf("Cleaned up %d expired jobs", len(cleaned)),
		"cleaned_jobs": cleaned,
	})
}
