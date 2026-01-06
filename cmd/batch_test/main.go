package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

const baseURL = "http://localhost:8080/api/v1"
const computeEndpoint = baseURL + "/compute-nodes"
const jobsEndpoint = baseURL + "/jobs"
const fabricID = "4000"

func main() {
	fmt.Println("=== Deploy Batcher Test ===")
	fmt.Println()

	// Step 0: Get available switch ports
	fmt.Println("Step 0: Getting available switch ports...")
	ports := getAvailablePorts(fabricID, 20)
	if len(ports) < 20 {
		fmt.Printf("Warning: Only %d ports available, need 20 for full test\n", len(ports))
		if len(ports) == 0 {
			fmt.Println("No ports available. Create switch ports first.")
			return
		}
	}
	fmt.Printf("Found %d available ports\n\n", len(ports))

	// Step 1: Create 20 compute nodes with port mappings
	fmt.Println("Step 1: Creating compute nodes with port mappings...")
	nodeNames := createComputeNodesWithPorts(ports)
	if len(nodeNames) == 0 {
		fmt.Println("Failed to create compute nodes")
		return
	}
	fmt.Printf("Created %d compute nodes with port mappings\n\n", len(nodeNames))

	// Step 1.5: Cleanup existing batch-test jobs
	fmt.Println("Step 1.5: Cleaning up existing batch-test jobs...")
	cleanupBatchTestJobs()
	fmt.Println()

	// Step 2: Submit jobs
	numJobs := 1 // Change to 5 for batch testing
	fmt.Printf("Step 2: Submitting %d job(s)...\n", numJobs)
	if numJobs > 1 {
		fmt.Println("Each job gets 4 nodes. Expecting all deploys to batch into 1-2 actual deploys.")
	}
	fmt.Println()

	var wg sync.WaitGroup
	results := make(chan jobResult, numJobs)

	startTime := time.Now()

	for i := 0; i < numJobs; i++ {
		wg.Add(1)
		go func(jobNum int) {
			defer wg.Done()

			// Each job gets 4 nodes
			startIdx := jobNum * 4
			endIdx := startIdx + 4
			if endIdx > len(nodeNames) {
				endIdx = len(nodeNames)
			}
			nodes := nodeNames[startIdx:endIdx]

			jobID := fmt.Sprintf("batch-test-%d-%d", time.Now().Unix(), jobNum)
			result := submitJob(jobID, fmt.Sprintf("Batch Test Job %d", jobNum), nodes)
			result.jobNum = jobNum
			result.submittedAt = time.Since(startTime)
			results <- result
		}(i)

		// Small delay between submissions (100ms) - still within debounce window
		time.Sleep(100 * time.Millisecond)
	}

	// Wait for all jobs to complete
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results
	fmt.Println("Results:")
	fmt.Println("--------")
	for result := range results {
		if result.err != nil {
			fmt.Printf("Job %d: FAILED after %v - %v\n", result.jobNum, result.duration, result.err)
		} else {
			fmt.Printf("Job %d: SUCCESS after %v (submitted at +%v)\n",
				result.jobNum, result.duration, result.submittedAt)
		}
	}

	totalTime := time.Since(startTime)
	fmt.Printf("\nTotal time: %v\n", totalTime)
	fmt.Println()

	// Step 3: Check logs for batching behavior
	fmt.Println("Step 3: Check server logs for:")
	fmt.Println("  - 'Deploy batch started' - should appear once")
	fmt.Println("  - 'Deploy request added to batch' - should appear 4 times")
	fmt.Println("  - 'Executing batched deploy' - should appear once")
	fmt.Println("  - 'Batched deploy succeeded' - should appear once")
}

type jobResult struct {
	jobNum      int
	err         error
	duration    time.Duration
	submittedAt time.Duration
}

type portInfo struct {
	ID       string
	SwitchID string
}

func getAvailablePorts(fabricID string, count int) []portInfo {
	var ports []portInfo

	// Get switches in fabric
	resp, err := http.Get(fmt.Sprintf("%s/fabrics/%s/switches", baseURL, fabricID))
	if err != nil {
		fmt.Printf("Failed to get switches: %v\n", err)
		return ports
	}
	defer func() { _ = resp.Body.Close() }()

	var switches []struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&switches); err != nil {
		fmt.Printf("Failed to decode switches: %v\n", err)
		return ports
	}

	// Get ports from each switch until we have enough
	for _, sw := range switches {
		if len(ports) >= count {
			break
		}

		resp, err := http.Get(fmt.Sprintf("%s/fabrics/%s/switches/%s/ports", baseURL, fabricID, sw.ID))
		if err != nil {
			continue
		}

		var switchPorts []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&switchPorts); err != nil {
			_ = resp.Body.Close()
			continue
		}
		_ = resp.Body.Close()

		for _, p := range switchPorts {
			if len(ports) >= count {
				break
			}
			ports = append(ports, portInfo{ID: p.ID, SwitchID: sw.ID})
		}
	}

	return ports
}

func createComputeNodesWithPorts(ports []portInfo) []string {
	var names []string

	for i, port := range ports {
		name := fmt.Sprintf("batch-node-%02d", i+1)
		hostname := fmt.Sprintf("batch-node-%02d.hpc.local", i+1)
		ip := fmt.Sprintf("10.100.%d.%d", (i+1)/256, (i+1)%256)

		// Create compute node
		body := map[string]string{
			"name":       name,
			"hostname":   hostname,
			"ip_address": ip,
		}

		jsonBody, _ := json.Marshal(body)
		resp, err := http.Post(computeEndpoint, "application/json", bytes.NewBuffer(jsonBody))
		if err != nil {
			fmt.Printf("  Failed to create %s: %v\n", name, err)
			continue
		}

		var nodeID string
		if resp.StatusCode == http.StatusCreated {
			var node struct {
				ID string `json:"id"`
			}
			_ = json.NewDecoder(resp.Body).Decode(&node)
			nodeID = node.ID
			fmt.Printf("  Created: %s\n", name)
		} else if resp.StatusCode == http.StatusInternalServerError {
			// Might already exist - fetch it
			bodyBytes, _ := io.ReadAll(resp.Body)
			if bytes.Contains(bodyBytes, []byte("duplicate")) || bytes.Contains(bodyBytes, []byte("UNIQUE")) {
				// Fetch existing node
				nodeID = getNodeIDByName(name)
				if nodeID != "" {
					fmt.Printf("  Exists:  %s\n", name)
				}
			} else {
				fmt.Printf("  Failed:  %s - %s\n", name, string(bodyBytes))
				_ = resp.Body.Close()
				continue
			}
		} else {
			bodyBytes, _ := io.ReadAll(resp.Body)
			fmt.Printf("  Failed:  %s - %d: %s\n", name, resp.StatusCode, string(bodyBytes))
			_ = resp.Body.Close()
			continue
		}
		_ = resp.Body.Close()

		if nodeID == "" {
			continue
		}

		// Check if node already has a port mapping
		checkResp, err := http.Get(fmt.Sprintf("%s/%s", computeEndpoint, nodeID))
		if err == nil {
			var nodeData struct {
				PortMappings []struct {
					ID string `json:"id"`
				} `json:"port_mappings"`
			}
			_ = json.NewDecoder(checkResp.Body).Decode(&nodeData)
			_ = checkResp.Body.Close()
			if len(nodeData.PortMappings) > 0 {
				fmt.Printf("    Port mapping exists (skipping)\n")
				names = append(names, name)
				continue
			}
		}

		// Add port mapping
		mappingBody := map[string]string{
			"switch_port_id": port.ID,
		}
		mappingJSON, _ := json.Marshal(mappingBody)
		mappingResp, err := http.Post(
			fmt.Sprintf("%s/%s/port-mappings", computeEndpoint, nodeID),
			"application/json",
			bytes.NewBuffer(mappingJSON),
		)
		if err != nil {
			fmt.Printf("    Failed to add port mapping: %v\n", err)
			continue
		}
		if mappingResp.StatusCode == http.StatusCreated {
			fmt.Printf("    Port mapping added\n")
		} else {
			bodyBytes, _ := io.ReadAll(mappingResp.Body)
			fmt.Printf("    Port mapping failed: %s\n", string(bodyBytes))
		}
		_ = mappingResp.Body.Close()

		names = append(names, name)
	}

	return names
}

func getNodeIDByName(name string) string {
	resp, err := http.Get(computeEndpoint)
	if err != nil {
		return ""
	}
	defer func() { _ = resp.Body.Close() }()

	var nodes []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&nodes); err != nil {
		return ""
	}

	for _, n := range nodes {
		if n.Name == name {
			return n.ID
		}
	}
	return ""
}

func cleanupBatchTestJobs() {
	// Get all jobs
	resp, err := http.Get(jobsEndpoint)
	if err != nil {
		fmt.Printf("  Failed to get jobs: %v\n", err)
		return
	}
	defer func() { _ = resp.Body.Close() }()

	var jobs []struct {
		SlurmJobID string `json:"slurm_job_id"`
		Status     string `json:"status"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&jobs); err != nil {
		fmt.Printf("  Failed to decode jobs: %v\n", err)
		return
	}

	// Complete any batch-test jobs that are not already completed
	completed := 0
	for _, job := range jobs {
		if len(job.SlurmJobID) > 10 && job.SlurmJobID[:10] == "batch-test" && job.Status != "completed" {
			completeResp, err := http.Post(
				fmt.Sprintf("%s/%s/complete", jobsEndpoint, job.SlurmJobID),
				"application/json",
				nil,
			)
			if err == nil {
				_ = completeResp.Body.Close()
				completed++
			}
		}
	}
	fmt.Printf("  Completed %d batch-test jobs\n", completed)
}

func submitJob(slurmJobID, name string, nodes []string) jobResult {
	start := time.Now()

	body := map[string]interface{}{
		"slurm_job_id":  slurmJobID,
		"name":          name,
		"compute_nodes": nodes,
	}

	jsonBody, _ := json.Marshal(body)
	resp, err := http.Post(jobsEndpoint, "application/json", bytes.NewBuffer(jsonBody))
	duration := time.Since(start)

	if err != nil {
		return jobResult{err: err, duration: duration}
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return jobResult{err: fmt.Errorf("status %d: %s", resp.StatusCode, string(bodyBytes)), duration: duration}
	}

	return jobResult{duration: duration}
}
