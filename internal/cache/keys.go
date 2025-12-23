package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

// Key prefix - simple app-level prefix
const keyPrefix = "nd"

// Cache domain prefixes
const (
	domainAuth   = "auth"
	domainLAN    = "lan"
	domainSec    = "sec"
	domainJob    = "job"
	domainQueue  = "queue"
	domainLock   = "lock"
	domainLease  = "lease"
	domainWorker = "worker"
	domainRL     = "rl"
	domainDB     = "db"
	domainIdempo = "idempo"
	domainCache  = "cachekeys"
)

// Default TTLs
const (
	TTLAuthToken      = 55 * time.Minute // assuming 1hr token, minus buffer
	TTLAuthLoginLock  = time.Minute
	TTLFabrics        = 5 * time.Minute
	TTLSwitches       = 2 * time.Minute
	TTLPorts          = time.Minute
	TTLSecurityGroups = time.Minute
	TTLContracts      = time.Minute
	TTLProtocols      = 5 * time.Minute
	TTLAssociations   = 30 * time.Second
	TTLIdempotency    = 30 * time.Minute
	TTLLock           = 2 * time.Minute
	TTLLease          = time.Minute
	TTLJobStatus      = 5 * time.Minute
	TTLDBLookup       = 5 * time.Minute
)

// Auth keys

// AuthToken returns the key for cached ND auth token
func AuthToken() string {
	return fmt.Sprintf("%s:%s:token", keyPrefix, domainAuth)
}

// AuthLoginLock returns the key for login stampede prevention
func AuthLoginLock() string {
	return fmt.Sprintf("%s:%s:loginLock", keyPrefix, domainAuth)
}

// LAN Fabric keys

// Fabrics returns the key for cached fabrics list
func Fabrics() string {
	return fmt.Sprintf("%s:%s:fabrics", keyPrefix, domainLAN)
}

// FabricByName returns the key for fabric name -> ID lookup
func FabricByName(name string) string {
	return fmt.Sprintf("%s:%s:fabricByName:%s", keyPrefix, domainLAN, name)
}

// Switches returns the key for switches in a fabric
func Switches(fabricName string) string {
	return fmt.Sprintf("%s:%s:switches:%s", keyPrefix, domainLAN, fabricName)
}

// Switch returns the key for a single switch
func Switch(fabricName, switchID string) string {
	return fmt.Sprintf("%s:%s:switch:%s:%s", keyPrefix, domainLAN, fabricName, switchID)
}

// Ports returns the key for ports on a switch
func Ports(fabricName, switchID string) string {
	return fmt.Sprintf("%s:%s:ports:%s:%s", keyPrefix, domainLAN, fabricName, switchID)
}

// Port returns the key for a single port
func Port(fabricName, switchID, portID string) string {
	return fmt.Sprintf("%s:%s:port:%s:%s:%s", keyPrefix, domainLAN, fabricName, switchID, portID)
}

// Security keys

// SecurityGroups returns the key for security groups in a fabric
func SecurityGroups(fabric string) string {
	return fmt.Sprintf("%s:%s:groups:%s", keyPrefix, domainSec, fabric)
}

// SecurityGroup returns the key for a single security group by name
func SecurityGroup(fabric, groupName string) string {
	return fmt.Sprintf("%s:%s:group:%s:%s", keyPrefix, domainSec, fabric, groupName)
}

// Contracts returns the key for contracts in a fabric
func Contracts(fabric string) string {
	return fmt.Sprintf("%s:%s:contracts:%s", keyPrefix, domainSec, fabric)
}

// Contract returns the key for a single contract by name
func Contract(fabric, contractName string) string {
	return fmt.Sprintf("%s:%s:contract:%s:%s", keyPrefix, domainSec, fabric, contractName)
}

// Protocols returns the key for protocols in a fabric
func Protocols(fabric string) string {
	return fmt.Sprintf("%s:%s:protocols:%s", keyPrefix, domainSec, fabric)
}

// Associations returns the key for contract associations in a fabric
func Associations(fabric string) string {
	return fmt.Sprintf("%s:%s:associations:%s", keyPrefix, domainSec, fabric)
}

// Cache invalidation keys

// FabricCacheKeys returns the key for the set of cache keys to invalidate for a fabric
func FabricCacheKeys(fabric string) string {
	return fmt.Sprintf("%s:%s:%s", keyPrefix, domainCache, fabric)
}

// Idempotency keys

// IdempotencyKey returns the key for idempotency check
func IdempotencyKey(operation string, payloadHash string) string {
	return fmt.Sprintf("%s:%s:%s:%s", keyPrefix, domainIdempo, operation, payloadHash)
}

// Lock keys

// LockSecurityGroup returns the lock key for a security group
func LockSecurityGroup(fabric, groupName string) string {
	return fmt.Sprintf("%s:%s:%s:group:%s:%s", keyPrefix, domainLock, domainSec, fabric, groupName)
}

// LockContract returns the lock key for a contract
func LockContract(fabric, contractName string) string {
	return fmt.Sprintf("%s:%s:%s:contract:%s:%s", keyPrefix, domainLock, domainSec, fabric, contractName)
}

// LockAssociation returns the lock key for a contract association
func LockAssociation(fabric, vrf, srcGroup, dstGroup, contract string) string {
	return fmt.Sprintf("%s:%s:%s:assoc:%s:%s:%s:%s:%s", keyPrefix, domainLock, domainSec, fabric, vrf, srcGroup, dstGroup, contract)
}

// LockFabric returns a global lock for a fabric
func LockFabric(fabric string) string {
	return fmt.Sprintf("%s:%s:fabric:%s", keyPrefix, domainLock, fabric)
}

// Job keys

// JobQueue returns the key for the job queue
func JobQueue() string {
	return fmt.Sprintf("%s:%s:jobs", keyPrefix, domainQueue)
}

// JobQueueFabric returns the key for a fabric-specific job queue
func JobQueueFabric(fabric string) string {
	return fmt.Sprintf("%s:%s:jobs:%s", keyPrefix, domainQueue, fabric)
}

// JobStatus returns the key for job status (hot state for UI)
func JobStatus(jobID string) string {
	return fmt.Sprintf("%s:%s:%s:status", keyPrefix, domainJob, jobID)
}

// JobProgress returns the key for job progress
func JobProgress(jobID string) string {
	return fmt.Sprintf("%s:%s:%s:progress", keyPrefix, domainJob, jobID)
}

// JobLastEvent returns the key for job's last event
func JobLastEvent(jobID string) string {
	return fmt.Sprintf("%s:%s:%s:lastEvent", keyPrefix, domainJob, jobID)
}

// Lease/worker keys

// JobLease returns the key for a job lease
func JobLease(jobID string) string {
	return fmt.Sprintf("%s:%s:%s:%s", keyPrefix, domainLease, domainJob, jobID)
}

// WorkerHeartbeat returns the key for a worker heartbeat
func WorkerHeartbeat(workerID string) string {
	return fmt.Sprintf("%s:%s:%s:hb", keyPrefix, domainWorker, workerID)
}

// Rate limiting keys

// RateLimit returns the key for rate limiting an endpoint
func RateLimit(endpoint, fabric string) string {
	return fmt.Sprintf("%s:%s:%s:%s", keyPrefix, domainRL, endpoint, fabric)
}

// Backoff returns the key for backoff tracking
func Backoff(endpoint, fabric string) string {
	return fmt.Sprintf("%s:backoff:%s:%s", keyPrefix, endpoint, fabric)
}

// DB lookup cache keys

// DBGroupByName returns the key for local DB group lookup cache
func DBGroupByName(fabric, name string) string {
	return fmt.Sprintf("%s:%s:groupByName:%s:%s", keyPrefix, domainDB, fabric, name)
}

// DBContractByName returns the key for local DB contract lookup cache
func DBContractByName(fabric, name string) string {
	return fmt.Sprintf("%s:%s:contractByName:%s:%s", keyPrefix, domainDB, fabric, name)
}

// Helper functions

// HashPayload creates a SHA256 hash of a payload for idempotency keys
func HashPayload(payload []byte) string {
	h := sha256.Sum256(payload)
	return hex.EncodeToString(h[:16]) // Use first 16 bytes (32 hex chars)
}

// ParseKey extracts components from a cache key (for debugging)
func ParseKey(key string) map[string]string {
	parts := strings.Split(key, ":")
	result := make(map[string]string)
	result["raw"] = key
	result["parts"] = fmt.Sprintf("%d", len(parts))
	if len(parts) >= 2 && parts[0] == keyPrefix {
		result["domain"] = parts[1]
	}
	return result
}
