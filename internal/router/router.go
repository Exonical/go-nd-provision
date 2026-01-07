package router

import (
	"github.com/banglin/go-nd/internal/config"
	"github.com/banglin/go-nd/internal/database"
	"github.com/banglin/go-nd/internal/handlers"
	"github.com/banglin/go-nd/internal/ndclient"
	"github.com/gin-gonic/gin"
)

func Setup(ndClient *ndclient.Client, cfg *config.Config) *gin.Engine {
	r := gin.Default()

	// Initialize handlers
	fabricHandler := handlers.NewFabricHandler(ndClient)
	computeHandler := handlers.NewComputeHandler()
	securityHandler := handlers.NewSecurityHandler(ndClient)
	jobHandler := handlers.NewJobHandler(database.DB, ndClient, &cfg.NexusDashboard)

	// Health check
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	// API v1 routes
	v1 := r.Group("/api/v1")
	{
		// Fabric routes (new API for querying)
		fabrics := v1.Group("/fabrics")
		{
			fabrics.GET("", fabricHandler.GetFabrics)
			fabrics.GET("/:id", fabricHandler.GetFabric)
			fabrics.POST("", fabricHandler.CreateFabric)
			fabrics.POST("/sync", fabricHandler.SyncFabrics)

			// Switch routes
			fabrics.GET("/:id/switches", fabricHandler.GetSwitches)
			fabrics.POST("/:id/switches", fabricHandler.CreateSwitch)
			fabrics.GET("/:id/switches/:switchId", fabricHandler.GetSwitch)
			fabrics.POST("/:id/switches/sync", fabricHandler.SyncSwitches)

			// Network routes
			fabrics.GET("/:id/networks", fabricHandler.GetNetworks)

			// Switch port routes
			fabrics.POST("/:id/ports/sync", fabricHandler.SyncAllPorts) // Sync all ports in fabric
			fabrics.GET("/:id/switches/:switchId/ports", fabricHandler.GetSwitchPorts)
			fabrics.GET("/:id/switches/:switchId/ports/:portId", fabricHandler.GetSwitchPort)
			fabrics.POST("/:id/switches/:switchId/ports", fabricHandler.CreateSwitchPort)
			fabrics.POST("/:id/switches/:switchId/ports/sync", fabricHandler.SyncSwitchPorts)
			fabrics.DELETE("/:id/switches/:switchId/ports", fabricHandler.DeleteSwitchPorts)
		}

		// Compute node routes
		compute := v1.Group("/compute-nodes")
		{
			compute.GET("", computeHandler.GetComputeNodes)
			compute.GET("/:id", computeHandler.GetComputeNode)
			compute.POST("", computeHandler.CreateComputeNode)
			compute.PUT("/:id", computeHandler.UpdateComputeNode)
			compute.DELETE("/:id", computeHandler.DeleteComputeNode)

			// Port mapping routes
			compute.GET("/:id/port-mappings", computeHandler.GetPortMappings)
			compute.POST("/:id/port-mappings", computeHandler.AddPortMapping)
			compute.DELETE("/:id/port-mappings/:mappingId", computeHandler.DeletePortMapping)
		}

		// Compute nodes by switch/port lookup
		v1.GET("/switches/:switchId/compute-nodes", computeHandler.GetComputeNodesBySwitch)
		v1.GET("/ports/:portId/compute-nodes", computeHandler.GetComputeNodesBySwitchPort)

		// Security routes (Legacy 3.x API)
		security := v1.Group("/security")
		{
			// Security Groups with Port Selectors
			groups := security.Group("/groups")
			{
				groups.GET("", securityHandler.GetSecurityGroups)
				groups.GET("/ndfc", securityHandler.ListNDFCSecurityGroups)
				groups.DELETE("/ndfc/:groupId", securityHandler.DeleteNDFCSecurityGroup)
				groups.GET("/:id", securityHandler.GetSecurityGroup)
				groups.POST("", securityHandler.CreateSecurityGroup)
				groups.DELETE("/:id", securityHandler.DeleteSecurityGroup)
			}

			// Security Contracts
			contracts := security.Group("/contracts")
			{
				contracts.GET("", securityHandler.GetSecurityContracts)
				contracts.GET("/:id", securityHandler.GetSecurityContract)
				contracts.POST("", securityHandler.CreateSecurityContract)
				contracts.DELETE("/:id", securityHandler.DeleteSecurityContract)
			}

			// Security Associations
			associations := security.Group("/associations")
			{
				associations.GET("", securityHandler.GetSecurityAssociations)
				associations.GET("/:id", securityHandler.GetSecurityAssociation)
				associations.POST("", securityHandler.CreateSecurityAssociation)
				associations.DELETE("/:id", securityHandler.DeleteSecurityAssociation)
			}
		}

		// Job routes (Slurm integration)
		jobs := v1.Group("/jobs")
		{
			jobs.GET("", jobHandler.ListJobs)
			jobs.POST("", jobHandler.SubmitJob)
			jobs.GET("/:slurm_job_id", jobHandler.GetJob)
			jobs.POST("/:slurm_job_id/complete", jobHandler.CompleteJob)
			jobs.POST("/cleanup", jobHandler.CleanupExpiredJobs)
		}
	}

	return r
}
