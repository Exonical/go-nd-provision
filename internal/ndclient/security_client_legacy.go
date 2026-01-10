package ndclient

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/banglin/go-nd/internal/ndclient/common"
)

// Legacy NDFC Security API client methods
// Uses /appcenter/cisco/ndfc/api/v1/security

// wrapOpErr wraps an error with operation context and includes API response body if available.
// Delegates to common.WrapAPIErrorWithContext for consistent error formatting.
func wrapOpErr(op, fabric string, err error) error {
	return common.WrapAPIErrorWithContext(op, "fabric="+fabric, err)
}

// Security Group methods

func (c *Client) CreateSecurityGroups(ctx context.Context, fabricName string, groups []SecurityGroup) ([]SecurityGroup, error) {
	if err := common.RequireNonEmpty("fabricName", fabricName); err != nil {
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
		return nil, wrapOpErr(opCreateSecGroups, fabricName, err)
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
	if err := common.RequireNonEmpty("fabricName", fabricName); err != nil {
		return nil, err
	}

	path, err := c.secFabricPath(fabricName, "groups")
	if err != nil {
		return nil, err
	}

	var out []SecurityGroup
	if err := c.Get(ctx, path, &out); err != nil {
		return nil, wrapOpErr(opGetSecGroups, fabricName, err)
	}
	return out, nil
}

// GetSecurityGroupByName retrieves a security group by its name (not ID)
func (c *Client) GetSecurityGroupByName(ctx context.Context, fabricName, groupName string) (*SecurityGroup, error) {
	if err := common.RequireNonEmpty("fabricName", fabricName); err != nil {
		return nil, err
	}
	if err := common.RequireNonEmpty("groupName", groupName); err != nil {
		return nil, err
	}

	// Use list+filter approach since /groups/{name} path may not be supported
	groups, err := c.GetSecurityGroups(ctx, fabricName)
	if err != nil {
		return nil, err
	}
	for i := range groups {
		if groups[i].GroupName == groupName {
			return &groups[i], nil
		}
	}
	return nil, fmt.Errorf("%s (fabric=%s, name=%s): not found", opGetSecGroup, fabricName, groupName)
}

func (c *Client) UpdateSecurityGroups(ctx context.Context, fabricName string, groups []SecurityGroup) ([]SecurityGroup, error) {
	if err := common.RequireNonEmpty("fabricName", fabricName); err != nil {
		return nil, err
	}
	if len(groups) == 0 {
		return nil, fmt.Errorf("groups cannot be empty")
	}

	// NDFC API requires PUT to /groups/{groupId} for each group (not batch PUT to /groups)
	var results []SecurityGroup
	for i, g := range groups {
		if err := validateSecurityGroup(g); err != nil {
			return nil, fmt.Errorf("groups[%d]: %w", i, err)
		}
		if g.GroupID == nil || *g.GroupID <= 0 {
			return nil, fmt.Errorf("groups[%d]: groupID is required for update", i)
		}

		sanitized := sanitizeGroupForRequest(g)
		path, err := c.secFabricPath(fabricName, "groups", fmt.Sprintf("%d", *g.GroupID))
		if err != nil {
			return nil, err
		}

		var out SecurityGroup
		if err := c.Put(ctx, path, sanitized, &out); err != nil {
			return nil, wrapOpErr(opUpdateSecGroups, fabricName, err)
		}
		results = append(results, out)
	}
	return results, nil
}

func (c *Client) DeleteSecurityGroup(ctx context.Context, fabricName string, groupID int) error {
	if err := common.RequireNonEmpty("fabricName", fabricName); err != nil {
		return err
	}
	if groupID <= 0 {
		return fmt.Errorf("groupID must be > 0")
	}

	// First, detach the security group by updating with attach=false.
	// Best-effort: if detach fails with anything other than "not found", we still
	// attempt the delete since the group might already be detached or the delete
	// endpoint might handle attached groups.
	detachGroup := SecurityGroup{
		FabricName: fabricName,
		GroupID:    &groupID,
		Attach:     false,
	}
	// Attempt to detach before delete - ignore errors as delete may still succeed if already detached
	_, _ = c.UpdateSecurityGroups(ctx, fabricName, []SecurityGroup{detachGroup})

	basePath, err := c.secFabricPath(fabricName, "groups")
	if err != nil {
		return err
	}

	q := url.Values{}
	q.Set("groupId", fmt.Sprintf("%d", groupID))
	path := common.AddQuery(basePath, q)

	if err := c.Delete(ctx, path); err != nil {
		return wrapOpErr(opDeleteSecGroup, fabricName, err)
	}
	return nil
}

// Security Protocol methods

func (c *Client) CreateSecurityProtocols(ctx context.Context, fabricName string, protocols []SecurityProtocol) ([]SecurityProtocol, error) {
	if err := common.RequireNonEmpty("fabricName", fabricName); err != nil {
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
		return nil, wrapOpErr(opCreateSecProtocols, fabricName, err)
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
	if err := common.RequireNonEmpty("fabricName", fabricName); err != nil {
		return nil, err
	}

	path, err := c.secFabricPath(fabricName, "protocols")
	if err != nil {
		return nil, err
	}

	var out []SecurityProtocol
	if err := c.Get(ctx, path, &out); err != nil {
		return nil, wrapOpErr(opGetSecProtocols, fabricName, err)
	}
	return out, nil
}

func (c *Client) GetSecurityProtocol(ctx context.Context, fabricName, protocolName string) (*SecurityProtocol, error) {
	if err := common.RequireNonEmpty("fabricName", fabricName); err != nil {
		return nil, err
	}
	if err := common.RequireNonEmpty("protocolName", protocolName); err != nil {
		return nil, err
	}

	path, err := c.secFabricPath(fabricName, "protocols", protocolName)
	if err != nil {
		return nil, err
	}

	var out SecurityProtocol
	if err := c.Get(ctx, path, &out); err != nil {
		return nil, wrapOpErr(opGetSecProtocol, fabricName, err)
	}
	return &out, nil
}

func (c *Client) DeleteSecurityProtocol(ctx context.Context, fabricName, protocolName string) error {
	if err := common.RequireNonEmpty("fabricName", fabricName); err != nil {
		return err
	}
	if err := common.RequireNonEmpty("protocolName", protocolName); err != nil {
		return err
	}

	path, err := c.secFabricPath(fabricName, "protocols", protocolName)
	if err != nil {
		return err
	}

	if err := c.Delete(ctx, path); err != nil {
		return wrapOpErr(opDeleteSecProtocol, fabricName, err)
	}
	return nil
}

// Security Contract methods

func (c *Client) CreateSecurityContracts(ctx context.Context, fabricName string, contracts []SecurityContract) ([]SecurityContract, error) {
	if err := common.RequireNonEmpty("fabricName", fabricName); err != nil {
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
		return nil, wrapOpErr(opCreateSecContracts, fabricName, err)
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
	if err := common.RequireNonEmpty("fabricName", fabricName); err != nil {
		return nil, err
	}

	path, err := c.secFabricPath(fabricName, "contracts")
	if err != nil {
		return nil, err
	}

	var out []SecurityContract
	if err := c.Get(ctx, path, &out); err != nil {
		return nil, wrapOpErr(opGetSecContracts, fabricName, err)
	}
	return out, nil
}

func (c *Client) GetSecurityContract(ctx context.Context, fabricName, contractName string) (*SecurityContract, error) {
	if err := common.RequireNonEmpty("fabricName", fabricName); err != nil {
		return nil, err
	}
	if err := common.RequireNonEmpty("contractName", contractName); err != nil {
		return nil, err
	}

	path, err := c.secFabricPath(fabricName, "contracts", contractName)
	if err != nil {
		return nil, err
	}

	var out SecurityContract
	if err := c.Get(ctx, path, &out); err != nil {
		return nil, wrapOpErr(opGetSecContract, fabricName, err)
	}
	return &out, nil
}

func (c *Client) UpdateSecurityContract(ctx context.Context, fabricName, contractName string, contract *SecurityContract) (*SecurityContract, error) {
	if err := common.RequireNonEmpty("fabricName", fabricName); err != nil {
		return nil, err
	}
	if err := common.RequireNonEmpty("contractName", contractName); err != nil {
		return nil, err
	}

	path, err := c.secFabricPath(fabricName, "contracts", contractName)
	if err != nil {
		return nil, err
	}

	var out SecurityContract
	if err := c.Put(ctx, path, contract, &out); err != nil {
		return nil, wrapOpErr(opUpdateSecContract, fabricName, err)
	}
	return &out, nil
}

func (c *Client) DeleteSecurityContract(ctx context.Context, fabricName, contractName string) error {
	if err := common.RequireNonEmpty("fabricName", fabricName); err != nil {
		return err
	}
	if err := common.RequireNonEmpty("contractName", contractName); err != nil {
		return err
	}

	basePath, err := c.secFabricPath(fabricName, "contracts")
	if err != nil {
		return err
	}

	q := url.Values{}
	q.Set("contractName", contractName)
	path := common.AddQuery(basePath, q)

	if err := c.Delete(ctx, path); err != nil {
		return wrapOpErr(opDeleteSecContract, fabricName, err)
	}
	return nil
}

// Contract Association methods

func (c *Client) CreateContractAssociations(ctx context.Context, fabricName string, associations []ContractAssociation) ([]ContractAssociation, error) {
	if err := common.RequireNonEmpty("fabricName", fabricName); err != nil {
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
		return nil, wrapOpErr(opCreateSecAssociations, fabricName, err)
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
	if err := common.RequireNonEmpty("fabricName", fabricName); err != nil {
		return nil, err
	}

	path, err := c.secFabricPath(fabricName, "contractAssociations")
	if err != nil {
		return nil, err
	}

	var out []ContractAssociation
	if err := c.Get(ctx, path, &out); err != nil {
		return nil, wrapOpErr(opGetSecAssociations, fabricName, err)
	}
	return out, nil
}

func (c *Client) DeleteSecurityAssociation(ctx context.Context, fabricName string, vrfName string, srcGroupID, dstGroupID int, contractName string) error {
	if err := common.RequireNonEmpty("fabricName", fabricName); err != nil {
		return err
	}
	if err := common.RequireNonEmpty("vrfName", vrfName); err != nil {
		return err
	}
	if err := common.RequireNonEmpty("contractName", contractName); err != nil {
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
	path := common.AddQuery(basePath, q)

	if err := c.Delete(ctx, path); err != nil {
		return wrapOpErr(opDeleteSecAssociation, fabricName, err)
	}
	return nil
}

// ConfigDeploy deploys the fabric configuration after security changes.
// This must be called after creating/modifying security groups, contracts, or associations.
// Retries up to 5 times with 5 second delay if a deploy is already in progress.
func (c *Client) ConfigDeploy(ctx context.Context, fabricName string, opts *ConfigDeployOptions) error {
	if err := common.RequireNonEmpty("fabricName", fabricName); err != nil {
		return err
	}

	// Build path: /appcenter/cisco/ndfc/api/v1/lan-fabric/rest/control/fabrics/{fabricName}/config-deploy
	path, err := c.ndfcLanFabricPath("rest", "control", "fabrics", fabricName, "config-deploy")
	if err != nil {
		return err
	}

	// Add query parameters if options provided
	if opts != nil {
		q := url.Values{}
		if opts.ForceShowRun {
			q.Set("forceShowRun", "true")
		}
		if opts.InclAllMSDSwitches {
			q.Set("inclAllMSDSwitches", "true")
		}
		path = common.AddQuery(path, q)
	}

	// Retry with exponential backoff if deploy is already in progress
	const maxRetries = 6
	baseDelay := 2 * time.Second

	var lastErr error
	for attempt := 1; attempt <= maxRetries; attempt++ {
		err := c.Post(ctx, path, struct{}{}, nil)
		if err == nil {
			return nil
		}

		if !isDeployInProgress(err) {
			return wrapOpErr(opConfigDeploy, fabricName, err)
		}

		lastErr = err
		if attempt == maxRetries {
			return wrapOpErr(opConfigDeploy, fabricName,
				fmt.Errorf("deploy still in progress after %d attempts: %w", attempt, err))
		}

		// Exponential backoff with cap at 30s
		delay := baseDelay * time.Duration(1<<(attempt-1))
		if delay > 30*time.Second {
			delay = 30 * time.Second
		}

		// Small jitter (+/- up to 20%) to prevent thundering herd
		jitter := time.Duration(int64(delay) / 5)
		delay = delay - jitter + time.Duration(time.Now().UnixNano()%int64(2*jitter+1))

		// Debug log for retry visibility
		// logger.Debug("config-deploy already in progress; retrying",
		//   zap.String("fabric", fabricName), zap.Int("attempt", attempt), zap.Duration("delay", delay))

		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return wrapOpErr(opConfigDeploy, fabricName, ctx.Err())
		case <-timer.C:
			// retry
		}
	}
	return wrapOpErr(opConfigDeploy, fabricName, lastErr)
}

// isDeployInProgress checks if the error indicates a deploy is already in progress.
// Tolerant matching: checks multiple status codes and body patterns.
func isDeployInProgress(err error) bool {
	if err == nil {
		return false
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		return false
	}

	// Check body for deploy-in-progress patterns (case-insensitive)
	body := strings.ToLower(apiErr.BodyString(1000))

	// Primary check: exact phrase
	if strings.Contains(body, "deploy is already in progress") {
		return true
	}
	// Fallback: partial matches
	if strings.Contains(body, "already in progress") && strings.Contains(body, "deploy") {
		return true
	}
	if strings.Contains(body, "config-deploy") && strings.Contains(body, "in progress") {
		return true
	}

	return false
}

// ConfigDeployOptions contains optional parameters for config deploy
type ConfigDeployOptions struct {
	// ForceShowRun: If true, config compliance tries to fetch show run if anything changed.
	// If false, it computes diff from cached show run.
	ForceShowRun bool

	// InclAllMSDSwitches: If true and passing MSD fabric name, all child fabric changes get deployed.
	// If false, MSD's child fabric changes are not deployed.
	InclAllMSDSwitches bool
}
