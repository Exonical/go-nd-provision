package ndclient

import (
	"context"
	"fmt"
	"net/url"
)

// Legacy NDFC Security API client methods
// Uses /appcenter/cisco/ndfc/api/v1/security

// Security Group methods

func (c *Client) CreateSecurityGroups(ctx context.Context, fabricName string, groups []SecurityGroup) ([]SecurityGroup, error) {
	if err := requireNonEmpty("fabricName", fabricName); err != nil {
		return nil, err
	}
	if len(groups) == 0 {
		return nil, fmt.Errorf("groups cannot be empty")
	}

	// Validate and sanitize each group
	sanitized := make([]SecurityGroup, len(groups))
	for i, g := range groups {
		if err := validateSecurityGroup(g); err != nil {
			return nil, fmt.Errorf("groups[%d]: %w", i, err)
		}
		sanitized[i] = sanitizeGroupForRequest(g)
	}

	path, err := c.secFabricPath(fabricName, "groups")
	if err != nil {
		return nil, err
	}

	var out BatchResponseGroups
	if err := c.Post(ctx, path, sanitized, &out); err != nil {
		// Include response body in error for debugging
		if apiErr, ok := err.(*APIError); ok {
			return nil, fmt.Errorf("%s (fabric=%s): %w, body: %s", opCreateSecGroups, fabricName, err, apiErr.BodyString(500))
		}
		return nil, fmt.Errorf("%s (fabric=%s): %w", opCreateSecGroups, fabricName, err)
	}
	if err := batchErr(opCreateSecGroups, fabricName, out.BatchResponse); err != nil {
		return nil, err
	}
	return out.SuccessList, nil
}

func (c *Client) CreateSecurityGroup(ctx context.Context, fabricName string, group *SecurityGroup) (*SecurityGroup, error) {
	if group == nil {
		return nil, fmt.Errorf("group is nil")
	}
	out, err := c.CreateSecurityGroups(ctx, fabricName, []SecurityGroup{*group})
	if err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("create security group (fabric=%s, name=%s): empty response", fabricName, group.GroupName)
	}
	return &out[0], nil
}

func (c *Client) GetSecurityGroups(ctx context.Context, fabricName string) ([]SecurityGroup, error) {
	if err := requireNonEmpty("fabricName", fabricName); err != nil {
		return nil, err
	}

	path, err := c.secFabricPath(fabricName, "groups")
	if err != nil {
		return nil, err
	}

	var out []SecurityGroup
	if err := c.Get(ctx, path, &out); err != nil {
		return nil, fmt.Errorf("get security groups (fabric=%s): %w", fabricName, err)
	}
	return out, nil
}

// GetSecurityGroupByName retrieves a security group by its name (not ID)
func (c *Client) GetSecurityGroupByName(ctx context.Context, fabricName, groupName string) (*SecurityGroup, error) {
	if err := requireNonEmpty("fabricName", fabricName); err != nil {
		return nil, err
	}
	if err := requireNonEmpty("groupName", groupName); err != nil {
		return nil, err
	}

	path, err := c.secFabricPath(fabricName, "groups", groupName)
	if err != nil {
		return nil, err
	}

	var out SecurityGroup
	if err := c.Get(ctx, path, &out); err != nil {
		return nil, fmt.Errorf("get security group (fabric=%s, name=%s): %w", fabricName, groupName, err)
	}
	return &out, nil
}

func (c *Client) UpdateSecurityGroups(ctx context.Context, fabricName string, groups []SecurityGroup) ([]SecurityGroup, error) {
	if err := requireNonEmpty("fabricName", fabricName); err != nil {
		return nil, err
	}
	if len(groups) == 0 {
		return nil, fmt.Errorf("groups cannot be empty")
	}

	// Validate and sanitize each group
	sanitized := make([]SecurityGroup, len(groups))
	for i, g := range groups {
		if err := validateSecurityGroup(g); err != nil {
			return nil, fmt.Errorf("groups[%d]: %w", i, err)
		}
		sanitized[i] = sanitizeGroupForRequest(g)
	}

	path, err := c.secFabricPath(fabricName, "groups")
	if err != nil {
		return nil, err
	}

	var out []SecurityGroup
	if err := c.Put(ctx, path, sanitized, &out); err != nil {
		return nil, fmt.Errorf("update security groups (fabric=%s): %w", fabricName, err)
	}
	return out, nil
}

func (c *Client) DeleteSecurityGroup(ctx context.Context, fabricName string, groupID int) error {
	if err := requireNonEmpty("fabricName", fabricName); err != nil {
		return err
	}
	if groupID <= 0 {
		return fmt.Errorf("groupID must be > 0")
	}

	// First, detach the security group by updating with attach=false
	detachGroup := SecurityGroup{
		FabricName: fabricName,
		GroupID:    &groupID,
		Attach:     false,
	}
	// Ignore error - group might already be detached
	_, _ = c.UpdateSecurityGroups(ctx, fabricName, []SecurityGroup{detachGroup})

	basePath, err := c.secFabricPath(fabricName, "groups")
	if err != nil {
		return err
	}

	q := url.Values{}
	q.Set("groupId", fmt.Sprintf("%d", groupID))
	path := addQuery(basePath, q)

	if err := c.Delete(ctx, path); err != nil {
		return fmt.Errorf("delete security group (fabric=%s, groupId=%d): %w", fabricName, groupID, err)
	}
	return nil
}

// Security Protocol methods

func (c *Client) CreateSecurityProtocols(ctx context.Context, fabricName string, protocols []SecurityProtocol) ([]SecurityProtocol, error) {
	if err := requireNonEmpty("fabricName", fabricName); err != nil {
		return nil, err
	}
	if len(protocols) == 0 {
		return nil, fmt.Errorf("protocols cannot be empty")
	}

	// Validate each protocol
	for i, p := range protocols {
		if err := validateSecurityProtocol(p); err != nil {
			return nil, fmt.Errorf("protocols[%d]: %w", i, err)
		}
	}

	path, err := c.secFabricPath(fabricName, "protocols")
	if err != nil {
		return nil, err
	}

	var out []SecurityProtocol
	if err := c.Post(ctx, path, protocols, &out); err != nil {
		return nil, fmt.Errorf("create security protocols (fabric=%s): %w", fabricName, err)
	}
	return out, nil
}

func (c *Client) CreateSecurityProtocol(ctx context.Context, fabricName string, protocol *SecurityProtocol) (*SecurityProtocol, error) {
	if protocol == nil {
		return nil, fmt.Errorf("protocol is nil")
	}
	out, err := c.CreateSecurityProtocols(ctx, fabricName, []SecurityProtocol{*protocol})
	if err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("create security protocol (fabric=%s, name=%s): empty response", fabricName, protocol.ProtocolName)
	}
	return &out[0], nil
}

func (c *Client) GetSecurityProtocols(ctx context.Context, fabricName string) ([]SecurityProtocol, error) {
	path, err := c.secFabricPath(fabricName, "protocols")
	if err != nil {
		return nil, err
	}

	var out []SecurityProtocol
	if err := c.Get(ctx, path, &out); err != nil {
		return nil, fmt.Errorf("get security protocols (fabric=%s): %w", fabricName, err)
	}
	return out, nil
}

func (c *Client) GetSecurityProtocol(ctx context.Context, fabricName, protocolName string) (*SecurityProtocol, error) {
	path, err := c.secFabricPath(fabricName, "protocols", protocolName)
	if err != nil {
		return nil, err
	}

	var out SecurityProtocol
	if err := c.Get(ctx, path, &out); err != nil {
		return nil, fmt.Errorf("get security protocol (fabric=%s, name=%s): %w", fabricName, protocolName, err)
	}
	return &out, nil
}

func (c *Client) DeleteSecurityProtocol(ctx context.Context, fabricName, protocolName string) error {
	path, err := c.secFabricPath(fabricName, "protocols", protocolName)
	if err != nil {
		return err
	}

	if err := c.Delete(ctx, path); err != nil {
		return fmt.Errorf("delete security protocol (fabric=%s, name=%s): %w", fabricName, protocolName, err)
	}
	return nil
}

// Security Contract methods

func (c *Client) CreateSecurityContracts(ctx context.Context, fabricName string, contracts []SecurityContract) ([]SecurityContract, error) {
	if err := requireNonEmpty("fabricName", fabricName); err != nil {
		return nil, err
	}
	if len(contracts) == 0 {
		return nil, fmt.Errorf("contracts cannot be empty")
	}

	// Validate and sanitize each contract
	sanitized := make([]SecurityContract, len(contracts))
	for i, ct := range contracts {
		if err := validateSecurityContract(ct); err != nil {
			return nil, fmt.Errorf("contracts[%d]: %w", i, err)
		}
		sanitized[i] = sanitizeContractForRequest(ct)
	}

	path, err := c.secFabricPath(fabricName, "contracts")
	if err != nil {
		return nil, err
	}

	var out BatchResponseContracts
	if err := c.Post(ctx, path, sanitized, &out); err != nil {
		return nil, fmt.Errorf("%s (fabric=%s): %w", opCreateSecContracts, fabricName, err)
	}
	if err := batchErr(opCreateSecContracts, fabricName, out.BatchResponse); err != nil {
		return nil, err
	}
	return out.SuccessList, nil
}

func (c *Client) CreateSecurityContract(ctx context.Context, fabricName string, contract *SecurityContract) (*SecurityContract, error) {
	if contract == nil {
		return nil, fmt.Errorf("contract is nil")
	}
	out, err := c.CreateSecurityContracts(ctx, fabricName, []SecurityContract{*contract})
	if err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("create security contract (fabric=%s, name=%s): empty response", fabricName, contract.ContractName)
	}
	return &out[0], nil
}

func (c *Client) GetSecurityContracts(ctx context.Context, fabricName string) ([]SecurityContract, error) {
	path, err := c.secFabricPath(fabricName, "contracts")
	if err != nil {
		return nil, err
	}

	var out []SecurityContract
	if err := c.Get(ctx, path, &out); err != nil {
		return nil, fmt.Errorf("get security contracts (fabric=%s): %w", fabricName, err)
	}
	return out, nil
}

func (c *Client) GetSecurityContract(ctx context.Context, fabricName, contractName string) (*SecurityContract, error) {
	path, err := c.secFabricPath(fabricName, "contracts", contractName)
	if err != nil {
		return nil, err
	}

	var out SecurityContract
	if err := c.Get(ctx, path, &out); err != nil {
		return nil, fmt.Errorf("get security contract (fabric=%s, name=%s): %w", fabricName, contractName, err)
	}
	return &out, nil
}

func (c *Client) UpdateSecurityContract(ctx context.Context, fabricName, contractName string, contract *SecurityContract) (*SecurityContract, error) {
	path, err := c.secFabricPath(fabricName, "contracts", contractName)
	if err != nil {
		return nil, err
	}

	var out SecurityContract
	if err := c.Put(ctx, path, contract, &out); err != nil {
		return nil, fmt.Errorf("update security contract (fabric=%s, name=%s): %w", fabricName, contractName, err)
	}
	return &out, nil
}

func (c *Client) DeleteSecurityContract(ctx context.Context, fabricName, contractName string) error {
	if err := requireNonEmpty("fabricName", fabricName); err != nil {
		return err
	}
	if err := requireNonEmpty("contractName", contractName); err != nil {
		return err
	}

	basePath, err := c.secFabricPath(fabricName, "contracts")
	if err != nil {
		return err
	}

	q := url.Values{}
	q.Set("contractName", contractName)
	path := addQuery(basePath, q)

	if err := c.Delete(ctx, path); err != nil {
		return fmt.Errorf("delete security contract (fabric=%s, name=%s): %w", fabricName, contractName, err)
	}
	return nil
}

// Contract Association methods

func (c *Client) CreateContractAssociations(ctx context.Context, fabricName string, associations []ContractAssociation) ([]ContractAssociation, error) {
	if err := requireNonEmpty("fabricName", fabricName); err != nil {
		return nil, err
	}
	if len(associations) == 0 {
		return nil, fmt.Errorf("associations cannot be empty")
	}

	// Validate and sanitize each association
	sanitized := make([]ContractAssociation, len(associations))
	for i, a := range associations {
		if err := validateContractAssociation(a); err != nil {
			return nil, fmt.Errorf("associations[%d]: %w", i, err)
		}
		sanitized[i] = sanitizeAssociationForRequest(a)
	}

	path, err := c.secFabricPath(fabricName, "contractAssociations")
	if err != nil {
		return nil, err
	}

	var out BatchResponseAssociations
	if err := c.Post(ctx, path, sanitized, &out); err != nil {
		return nil, fmt.Errorf("%s (fabric=%s): %w", opCreateSecAssociations, fabricName, err)
	}
	if err := batchErr(opCreateSecAssociations, fabricName, out.BatchResponse); err != nil {
		return nil, err
	}
	return out.SuccessList, nil
}

func (c *Client) CreateSecurityAssociation(ctx context.Context, fabricName string, association *ContractAssociation) (*ContractAssociation, error) {
	if association == nil {
		return nil, fmt.Errorf("association is nil")
	}
	out, err := c.CreateContractAssociations(ctx, fabricName, []ContractAssociation{*association})
	if err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("create contract association (fabric=%s): empty response", fabricName)
	}
	return &out[0], nil
}

func (c *Client) GetSecurityAssociations(ctx context.Context, fabricName string) ([]ContractAssociation, error) {
	path, err := c.secFabricPath(fabricName, "contractAssociations")
	if err != nil {
		return nil, err
	}

	var out []ContractAssociation
	if err := c.Get(ctx, path, &out); err != nil {
		return nil, fmt.Errorf("get contract associations (fabric=%s): %w", fabricName, err)
	}
	return out, nil
}

func (c *Client) DeleteSecurityAssociation(ctx context.Context, fabricName string, vrfName string, srcGroupID, dstGroupID int, contractName string) error {
	if err := requireNonEmpty("fabricName", fabricName); err != nil {
		return err
	}
	if err := requireNonEmpty("vrfName", vrfName); err != nil {
		return err
	}
	if err := requireNonEmpty("contractName", contractName); err != nil {
		return err
	}
	if srcGroupID <= 0 {
		return fmt.Errorf("srcGroupID must be > 0")
	}
	if dstGroupID <= 0 {
		return fmt.Errorf("dstGroupID must be > 0")
	}

	basePath, err := c.secFabricPath(fabricName, "contractAssociations")
	if err != nil {
		return err
	}

	q := url.Values{}
	q.Set("vrfName", vrfName)
	q.Set("srcGroupId", fmt.Sprintf("%d", srcGroupID))
	q.Set("dstGroupId", fmt.Sprintf("%d", dstGroupID))
	q.Set("contractName", contractName)
	path := addQuery(basePath, q)

	if err := c.Delete(ctx, path); err != nil {
		return fmt.Errorf("delete contract association (fabric=%s, vrf=%s, contract=%s): %w", fabricName, vrfName, contractName, err)
	}
	return nil
}
