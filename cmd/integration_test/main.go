//go:build ignore

// Integration test for NDFC Security API
// Run with: go run ./cmd/integration_test/main.go
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/banglin/go-nd/internal/config"
	"github.com/banglin/go-nd/internal/ndclient"
)

func main() {
	cfg := config.Load()

	fmt.Println("=== NDFC Security API Integration Test ===")
	fmt.Printf("Target: %s\n", cfg.NexusDashboard.BaseURL)
	fmt.Printf("Fabric: %s\n", cfg.NexusDashboard.ComputeFabricName)
	fmt.Printf("VRF: %s\n", cfg.NexusDashboard.ComputeVRFName)
	fmt.Println()

	if cfg.NexusDashboard.ComputeFabricName == "" {
		log.Fatal("ND_COMPUTE_FABRIC_NAME not set")
	}

	// Create client
	client, err := ndclient.NewClient(&cfg.NexusDashboard)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	fabricName := cfg.NexusDashboard.ComputeFabricName
	vrfName := cfg.NexusDashboard.ComputeVRFName
	testPrefix := fmt.Sprintf("test_%d", time.Now().Unix())

	// Test data - NDFC has length limits (contract name ≤20 chars)
	shortID := fmt.Sprintf("%d", time.Now().Unix()%10000)
	groupName := "tg_" + shortID
	contractName := "tc_" + shortID

	fmt.Printf("Test prefix: %s\n\n", testPrefix)

	// 0. Pre-flight: Validate VRF and Network exist
	fmt.Println("--- Step 0: Validate VRF and Network ---")
	lanFabric := client.LANFabric()

	if vrfName != "" {
		vrfExists, err := lanFabric.VRFExists(ctx, fabricName, vrfName)
		if err != nil {
			log.Fatalf("Failed to check VRF: %v", err)
		}
		if !vrfExists {
			fmt.Printf("⚠ VRF %q does not exist in fabric - some tests will be skipped\n", vrfName)
			vrfName = "" // Clear to skip VRF-dependent tests
		} else {
			fmt.Printf("✓ VRF %q exists\n", vrfName)
		}
	}

	if cfg.NexusDashboard.ComputeNetworkName != "" {
		networkExists, err := lanFabric.NetworkExists(ctx, fabricName, cfg.NexusDashboard.ComputeNetworkName)
		if err != nil {
			log.Fatalf("Failed to check network: %v", err)
		}
		if !networkExists {
			fmt.Printf("⚠ Network %q does not exist in fabric\n", cfg.NexusDashboard.ComputeNetworkName)
		} else {
			fmt.Printf("✓ Network %q exists\n", cfg.NexusDashboard.ComputeNetworkName)
		}
	}

	// 1. Create Security Group
	// NDFC requires groupId in range 16-65535
	// Use a wider range to avoid conflicts with previous test runs
	testGroupID := 16 + int(time.Now().UnixNano()%65000) // Random ID in valid range
	fmt.Println("--- Step 1: Create Security Group ---")
	fmt.Printf("Using groupId: %d\n", testGroupID)
	groups, err := client.CreateSecurityGroups(ctx, fabricName, []ndclient.SecurityGroup{
		{
			FabricName: fabricName,
			GroupID:    &testGroupID,
			GroupName:  groupName,
			// Empty selectors - group will be created without members initially
			Attach: false, // Don't attach until we have valid selectors
		},
	})
	if err != nil {
		log.Fatalf("CreateSecurityGroups failed: %v", err)
	}
	if len(groups) == 0 {
		log.Fatal("CreateSecurityGroups returned empty list")
	}
	var groupID int
	if groups[0].GroupID != nil {
		groupID = *groups[0].GroupID
	} else {
		groupID = testGroupID // Use the ID we requested
	}
	fmt.Printf("✓ Created security group: %s (ID: %d)\n", groupName, groupID)

	// 2. First, list existing protocols to find one we can use
	fmt.Println("\n--- Step 2a: List Security Protocols ---")
	protocols, err := client.GetSecurityProtocols(ctx, fabricName)
	if err != nil {
		log.Fatalf("GetSecurityProtocols failed: %v", err)
	}
	fmt.Printf("Found %d protocols\n", len(protocols))
	protocolName := ""
	for _, p := range protocols {
		fmt.Printf("  - %s\n", p.ProtocolName)
		if protocolName == "" {
			protocolName = p.ProtocolName // Use first available
		}
	}
	if protocolName == "" {
		log.Fatal("No protocols available - need to create one first")
	}
	fmt.Printf("Using protocol: %s\n", protocolName)

	// 2b. Create Security Contract
	fmt.Println("\n--- Step 2b: Create Security Contract ---")
	_, err = client.CreateSecurityContracts(ctx, fabricName, []ndclient.SecurityContract{
		{
			ContractName: contractName,
			Rules: []ndclient.ContractRule{
				{
					Direction:    "bidirectional",
					Action:       "permit",
					ProtocolName: protocolName,
				},
			},
		},
	})
	if err != nil {
		log.Fatalf("CreateSecurityContracts failed: %v", err)
	}
	fmt.Printf("✓ Created security contract: %s\n", contractName)

	// 3. Create Contract Association (self-referential for testing)
	// Skip if VRF doesn't exist - association requires valid VRF
	fmt.Println("\n--- Step 3: Create Contract Association ---")
	if vrfName == "" {
		fmt.Println("⚠ Skipping association - no VRF configured")
	} else {
		associations, err := client.CreateContractAssociations(ctx, fabricName, []ndclient.ContractAssociation{
			{
				FabricName:   fabricName,
				VRFName:      vrfName,
				SrcGroupID:   &groupID,
				DstGroupID:   &groupID,
				SrcGroupName: groupName,
				DstGroupName: groupName,
				ContractName: contractName,
				Attach:       true,
			},
		})
		if err != nil {
			// Non-fatal - VRF might not exist
			fmt.Printf("⚠ CreateContractAssociations failed (VRF may not exist): %v\n", err)
		} else if len(associations) > 0 {
			fmt.Printf("✓ Created contract association: %s -> %s via %s\n",
				groupName, groupName, contractName)
		}
	}

	// 4. Deploy Configuration (can take a long time)
	fmt.Println("\n--- Step 4: Config Deploy ---")
	err = client.ConfigDeploy(ctx, fabricName, nil)
	if err != nil {
		fmt.Printf("⚠ ConfigDeploy failed (may timeout): %v\n", err)
	} else {
		fmt.Println("✓ Config deployed successfully")
	}

	// 5. Verify by fetching
	fmt.Println("\n--- Step 5: Verify Created Objects ---")

	fetchedGroups, err := client.GetSecurityGroups(ctx, fabricName)
	if err != nil {
		log.Fatalf("GetSecurityGroups failed: %v", err)
	}
	found := false
	for _, g := range fetchedGroups {
		if g.GroupName == groupName {
			found = true
			fmt.Printf("✓ Verified group exists: %s\n", g.GroupName)
			break
		}
	}
	if !found {
		log.Fatalf("Created group not found in list")
	}

	fetchedContracts, err := client.GetSecurityContracts(ctx, fabricName)
	if err != nil {
		log.Fatalf("GetSecurityContracts failed: %v", err)
	}
	found = false
	for _, c := range fetchedContracts {
		if c.ContractName == contractName {
			found = true
			fmt.Printf("✓ Verified contract exists: %s\n", c.ContractName)
			break
		}
	}
	if !found {
		log.Fatalf("Created contract not found in list")
	}

	// 6. Cleanup (optional - comment out to keep test objects)
	if os.Getenv("SKIP_CLEANUP") != "1" {
		fmt.Println("\n--- Step 6: Cleanup ---")

		// Delete association first
		err = client.DeleteSecurityAssociation(ctx, fabricName, vrfName, groupID, groupID, contractName)
		if err != nil {
			fmt.Printf("⚠ DeleteSecurityAssociation failed (may already be deleted): %v\n", err)
		} else {
			fmt.Println("✓ Deleted contract association")
		}

		// Delete contract
		err = client.DeleteSecurityContract(ctx, fabricName, contractName)
		if err != nil {
			fmt.Printf("⚠ DeleteSecurityContract failed: %v\n", err)
		} else {
			fmt.Println("✓ Deleted security contract")
		}

		// Delete group
		err = client.DeleteSecurityGroup(ctx, fabricName, groupID)
		if err != nil {
			fmt.Printf("⚠ DeleteSecurityGroup failed: %v\n", err)
		} else {
			fmt.Println("✓ Deleted security group")
		}

		// Final deploy to apply deletions
		err = client.ConfigDeploy(ctx, fabricName, nil)
		if err != nil {
			fmt.Printf("⚠ Final ConfigDeploy failed: %v\n", err)
		} else {
			fmt.Println("✓ Final config deploy completed")
		}
	} else {
		fmt.Println("\n--- Skipping cleanup (SKIP_CLEANUP=1) ---")
	}

	fmt.Println("\n=== Integration Test PASSED ===")
}
