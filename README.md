# Go Nexus Dashboard

A Go application for interacting with Cisco Nexus Dashboard to manage Security Associations, Security Contracts, and Security Groups using Port Selectors.

## Features

- **Fabric Management**: Query fabrics, switches, networks, and switch ports from Nexus Dashboard
- **Security Management**: Create Security Groups, Contracts, and Associations (Legacy 3.x API)
- **Compute Node Mapping**: Track which compute nodes (servers) are connected to which switch ports
- **Job Management**: Slurm job integration for batch security policy deployment
- **gRPC API**: Pure gRPC microservice interface with token-based authentication
- **Caching**: Valkey (Redis-compatible) caching layer with rate limiting
- **Database**: PostgreSQL with GORM ORM
- **Structured Logging**: Zap-based logging

## Prerequisites

- Go 1.25+
- PostgreSQL
- Valkey (or Redis-compatible server)
- Cisco Nexus Dashboard access
- Docker & Docker Compose (optional)
- [buf](https://buf.build/docs/installation) (for proto code generation)

## Installation

1. Clone the repository:
```bash
git clone https://github.com/banglin/go-nd.git
cd go-nd
```

2. Install dependencies:
```bash
go mod tidy
```

3. Copy the environment file and configure:
```bash
cp .env.example .env
# Edit .env with your configuration
```

4. Build and run the application:
```bash
make build
make run
```

Or run directly:
```bash
go run ./cmd/server
```

### gRPC Server

Build and run the gRPC server (microservice mode):
```bash
make build-grpc
GRPC_AUTH_TOKEN=your-secret-token make run-grpc
```

Or run directly:
```bash
GRPC_AUTH_TOKEN=your-secret-token go run ./cmd/grpc_server
```

The gRPC server listens on port `9090` by default (configurable via `GRPC_PORT`).

### Docker

Start dependencies (PostgreSQL, Valkey):
```bash
make deps-up
```

Build and run with Docker:
```bash
docker-compose up
```

## Configuration

Set the following environment variables (or use `.env` file):

| Variable | Description | Default |
|----------|-------------|---------|
| `SERVER_PORT` | HTTP server port | `8080` |
| `GIN_MODE` | Gin mode (debug/release) | `debug` |
| `DB_HOST` | PostgreSQL host | `localhost` |
| `DB_PORT` | PostgreSQL port | `5432` |
| `DB_USER` | PostgreSQL user | `postgres` |
| `DB_PASSWORD` | PostgreSQL password | `postgres` |
| `DB_NAME` | Database name | `nexus_dashboard` |
| `DB_SSLMODE` | SSL mode | `disable` |
| `VALKEY_ADDRESS` | Valkey server address | `localhost:6379` |
| `VALKEY_PASSWORD` | Valkey password | `` |
| `VALKEY_DB` | Valkey database number | `0` |
| `ND_BASE_URL` | Nexus Dashboard URL | - |
| `ND_USERNAME` | Nexus Dashboard username | `admin` |
| `ND_PASSWORD` | Nexus Dashboard password | - |
| `ND_INSECURE` | Skip TLS verification | `true` |
| `GRPC_PORT` | gRPC server port | `9090` |
| `GRPC_AUTH_TOKEN` | gRPC authentication token (required) | - |
| `GRPC_REFLECTION` | Enable gRPC reflection | `true` |

## Nexus Dashboard API Base Paths

The Nexus Dashboard client (`internal/ndclient`) builds request paths using configurable base paths defined in `internal/ndclient/endpoints.go`. This allows the code to target:

- Legacy NDFC APIs (under `/appcenter/cisco/ndfc/api/v1/`)
- New ND APIs (under `/api/v1/`)

Default base paths:

| Namespace | Default Base Path |
|----------|-------------------|
| `ndfc.security.v1` | `/appcenter/cisco/ndfc/api/v1/security` |
| `ndfc.lan-fabric.v1` | `/appcenter/cisco/ndfc/api/v1/lan-fabric` |
| `ndfc.imagemanagement.v1` | `/appcenter/cisco/ndfc/api/v1/imagemanagement` |
| `nd.root.v1` | `/api/v1` |
| `nd.manage.v1` | `/api/v1/manage` |

If your deployment uses non-standard paths, you can override them in code:

```go
client, err := ndclient.NewClient(cfg)
if err != nil {
    // handle error
}

client = client.WithEndpoints(ndclient.Endpoints{
    Base: map[ndclient.APINamespace]string{
        ndclient.APINDFCSecurityV1:  "/appcenter/cisco/ndfc/api/v1/security",
        ndclient.APINDFCLANFabricV1: "/appcenter/cisco/ndfc/api/v1/lan-fabric",
        ndclient.APINDRootV1:        "/api/v1",
    },
})
```

## gRPC API

The gRPC server provides a pure gRPC interface for microservice communication. Proto definitions are in `proto/go_nd/v1/`.

### Authentication

All gRPC calls (except health checks) require a Bearer token in the `authorization` metadata:

```go
// Go client example
md := metadata.Pairs("authorization", "Bearer "+token)
ctx := metadata.NewOutgoingContext(context.Background(), md)
resp, err := client.SubmitJob(ctx, req)
```

```bash
# grpcurl example
grpcurl -H 'authorization: Bearer your-token' \
  -d '{"slurm_job_id": "job-123", "compute_nodes": ["node1", "node2"]}' \
  localhost:9090 go_nd.v1.JobsService/SubmitJob
```

### JobsService

| RPC | Description |
|-----|-------------|
| `SubmitJob` | Create a job and provision security groups |
| `GetJob` | Get job by Slurm job ID |
| `ListJobs` | List jobs with optional status/fabric filters |
| `CompleteJob` | Mark job as completed and deprovision |
| `CleanupExpiredJobs` | Remove expired jobs |

### ComputeNodesService

| RPC | Description |
|-----|-------------|
| `ListComputeNodes` | List all compute nodes |
| `GetComputeNode` | Get compute node by ID |
| `CreateComputeNode` | Create a new compute node |
| `UpdateComputeNode` | Update an existing compute node |
| `DeleteComputeNode` | Delete a compute node |
| `ListPortMappings` | List port mappings for a compute node |
| `AddPortMapping` | Add a port mapping to a compute node |
| `DeletePortMapping` | Remove a port mapping |

### FabricsService

| RPC | Description |
|-----|-------------|
| `ListFabrics` | List all fabrics |
| `GetFabric` | Get fabric by ID |
| `CreateFabric` | Create a new fabric |
| `SyncFabrics` | Sync fabrics from Nexus Dashboard |
| `ListSwitches` | List switches in a fabric |
| `GetSwitch` | Get switch by ID |
| `CreateSwitch` | Create a new switch |
| `SyncSwitches` | Sync switches from Nexus Dashboard |
| `ListNetworks` | List networks in a fabric (from ND) |
| `ListPorts` | List ports on a switch |
| `GetPort` | Get port by ID |
| `CreatePort` | Create a new port |
| `SyncPorts` | Sync ports from Nexus Dashboard |
| `DeletePorts` | Delete ports from a switch |

### Health Check

```bash
grpcurl localhost:9090 grpc.health.v1.Health/Check
```

### Proto Generation

Regenerate proto code after modifying `.proto` files:

```bash
make proto
```

## REST API Endpoints

### Health Check

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/health` | Health check endpoint |

### Fabrics

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/v1/fabrics` | List all fabrics |
| `GET` | `/api/v1/fabrics/:id` | Get fabric by ID |
| `POST` | `/api/v1/fabrics` | Create fabric |
| `POST` | `/api/v1/fabrics/sync` | Sync fabrics from ND |
| `GET` | `/api/v1/fabrics/:id/switches` | List switches in fabric |
| `GET` | `/api/v1/fabrics/:id/switches/:switchId` | Get switch by ID |
| `POST` | `/api/v1/fabrics/:id/switches` | Create switch |
| `POST` | `/api/v1/fabrics/:id/switches/sync` | Sync switches from ND |
| `GET` | `/api/v1/fabrics/:id/networks` | List networks in fabric |
| `POST` | `/api/v1/fabrics/:id/ports/sync` | Sync all ports in fabric |
| `GET` | `/api/v1/fabrics/:id/switches/:switchId/ports` | List switch ports |
| `GET` | `/api/v1/fabrics/:id/switches/:switchId/ports/:portId` | Get switch port by ID |
| `POST` | `/api/v1/fabrics/:id/switches/:switchId/ports` | Create switch port |
| `POST` | `/api/v1/fabrics/:id/switches/:switchId/ports/sync` | Sync ports from ND |
| `DELETE` | `/api/v1/fabrics/:id/switches/:switchId/ports` | Delete switch ports |

### Compute Nodes

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/v1/compute-nodes` | List all compute nodes |
| `GET` | `/api/v1/compute-nodes/:id` | Get compute node by ID |
| `POST` | `/api/v1/compute-nodes` | Create compute node |
| `PUT` | `/api/v1/compute-nodes/:id` | Update compute node |
| `DELETE` | `/api/v1/compute-nodes/:id` | Delete compute node |
| `GET` | `/api/v1/compute-nodes/:id/port-mappings` | Get port mappings |
| `POST` | `/api/v1/compute-nodes/:id/port-mappings` | Add port mapping |
| `DELETE` | `/api/v1/compute-nodes/:id/port-mappings/:mappingId` | Delete port mapping |
| `GET` | `/api/v1/switches/:switchId/compute-nodes` | Get nodes by switch |
| `GET` | `/api/v1/ports/:portId/compute-nodes` | Get nodes by port |

### Security (Legacy 3.x API)

#### Security Groups (with Port Selectors)

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/v1/security/groups` | List security groups |
| `GET` | `/api/v1/security/groups/ndfc` | List NDFC security groups |
| `GET` | `/api/v1/security/groups/:id` | Get security group |
| `POST` | `/api/v1/security/groups` | Create security group |
| `DELETE` | `/api/v1/security/groups/:id` | Delete security group |
| `DELETE` | `/api/v1/security/groups/ndfc/:groupId` | Delete NDFC security group |

#### Security Contracts

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/v1/security/contracts` | List security contracts |
| `GET` | `/api/v1/security/contracts/:id` | Get security contract |
| `POST` | `/api/v1/security/contracts` | Create security contract |
| `DELETE` | `/api/v1/security/contracts/:id` | Delete security contract |

#### Security Associations

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/v1/security/associations` | List security associations |
| `GET` | `/api/v1/security/associations/:id` | Get security association |
| `POST` | `/api/v1/security/associations` | Create security association |
| `DELETE` | `/api/v1/security/associations/:id` | Delete security association |

### Jobs (Slurm Integration)

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/v1/jobs` | List all jobs |
| `POST` | `/api/v1/jobs` | Submit a new job |
| `GET` | `/api/v1/jobs/:slurm_job_id` | Get job by Slurm job ID |
| `POST` | `/api/v1/jobs/:slurm_job_id/complete` | Mark job as complete |
| `POST` | `/api/v1/jobs/cleanup` | Cleanup expired jobs |

## Example Usage

The following examples show a typical workflow in order of operations.

### 1. Sync Fabric Data from Nexus Dashboard

First, sync your fabric infrastructure from Nexus Dashboard:

```bash
# Sync all fabrics
curl -X POST http://localhost:8080/api/v1/fabrics/sync

# List synced fabrics
curl http://localhost:8080/api/v1/fabrics

# Sync switches for a specific fabric
curl -X POST http://localhost:8080/api/v1/fabrics/DevNet_VxLAN_Fabric/switches/sync

# List switches in fabric
curl http://localhost:8080/api/v1/fabrics/DevNet_VxLAN_Fabric/switches

# Sync all ports for all switches in a fabric
curl -X POST http://localhost:8080/api/v1/fabrics/DevNet_VxLAN_Fabric/ports/sync

# List ports on a specific switch
curl "http://localhost:8080/api/v1/fabrics/DevNet_VxLAN_Fabric/switches/{switch_id}/ports"

# List networks in fabric (from NDFC)
curl http://localhost:8080/api/v1/fabrics/DevNet_VxLAN_Fabric/networks
```

### 2. Register Compute Nodes

Register your compute nodes (servers/HPC nodes):

```bash
# Create a compute node
curl -X POST http://localhost:8080/api/v1/compute-nodes \
  -H "Content-Type: application/json" \
  -d '{
    "name": "node-01",
    "hostname": "node-01.hpc.local",
    "ip_address": "10.0.0.10"
  }'

# List all compute nodes
curl http://localhost:8080/api/v1/compute-nodes

# Get a specific compute node
curl http://localhost:8080/api/v1/compute-nodes/{node_id}
```

### 3. Map Compute Nodes to Switch Ports

Map each compute node's NIC to its connected switch port:

```bash
# Add port mapping (node NIC -> switch port)
curl -X POST http://localhost:8080/api/v1/compute-nodes/{node_id}/port-mappings \
  -H "Content-Type: application/json" \
  -d '{
    "switch_port_id": "{port_id}",
    "nic_name": "eth0",
    "vlan": 100
  }'

# List port mappings for a node
curl http://localhost:8080/api/v1/compute-nodes/{node_id}/port-mappings

# Find compute nodes connected to a switch
curl http://localhost:8080/api/v1/switches/{switch_id}/compute-nodes
```

### 4. Create Security Groups

Create security groups with port selectors:

```bash
curl -X POST http://localhost:8080/api/v1/security/groups \
  -H "Content-Type: application/json" \
  -d '{
    "group_name": "hpc-job-12345",
    "group_id": 12345,
    "fabric_name": "DevNet_VxLAN_Fabric",
    "attach": true,
    "network_port_selectors": [
      {
        "network": "HPC_Network",
        "switch_id": "99433ZAWNB5",
        "interface_name": "Ethernet1/5"
      }
    ]
  }'

# List security groups
curl http://localhost:8080/api/v1/security/groups

# List security groups from NDFC
curl "http://localhost:8080/api/v1/security/groups/ndfc?fabric_name=DevNet_VxLAN_Fabric"
```

### 5. Create Security Contracts

Define traffic rules between security groups:

```bash
curl -X POST http://localhost:8080/api/v1/security/contracts \
  -H "Content-Type: application/json" \
  -d '{
    "contract_name": "allow-all",
    "fabric_name": "DevNet_VxLAN_Fabric",
    "rules": [
      {
        "direction": "in-out",
        "action": "permit",
        "protocol_name": "ip"
      }
    ]
  }'

# List security contracts
curl http://localhost:8080/api/v1/security/contracts
```

### 6. Create Security Associations

Associate security groups with contracts:

```bash
curl -X POST http://localhost:8080/api/v1/security/associations \
  -H "Content-Type: application/json" \
  -d '{
    "fabric_name": "DevNet_VxLAN_Fabric",
    "vrf_name": "HPC_VRF",
    "src_group_id": 12345,
    "dst_group_id": 12345,
    "src_group_name": "hpc-job-12345",
    "dst_group_name": "hpc-job-12345",
    "contract_name": "allow-all",
    "attach": true
  }'

# List security associations
curl http://localhost:8080/api/v1/security/associations
```

### 7. Job Management (Slurm Integration)

For HPC environments, use the jobs API to automate security policy lifecycle:

```bash
# Submit a job (creates security groups automatically)
curl -X POST http://localhost:8080/api/v1/jobs \
  -H "Content-Type: application/json" \
  -d '{
    "slurm_job_id": "12345",
    "name": "my-hpc-job",
    "compute_nodes": ["node-01", "node-02", "node-03"]
  }'

# List all jobs
curl http://localhost:8080/api/v1/jobs

# List active jobs only
curl "http://localhost:8080/api/v1/jobs?status=active"

# Get job details
curl http://localhost:8080/api/v1/jobs/12345

# Complete a job (removes security policies)
curl -X POST http://localhost:8080/api/v1/jobs/12345/complete

# Cleanup expired jobs
curl -X POST http://localhost:8080/api/v1/jobs/cleanup
```

## Development

### Make Targets

```bash
make test           # Run all tests
make test-v         # Run tests with verbose output
make test-cover     # Run tests with coverage report
make test-race      # Run tests with race detector
make test-short     # Run short tests only (skip slow tests)
make test-ndclient  # Run NDFC client tests only
make test-lanfabric # Run LAN fabric tests only
make test-security  # Run security client tests only
make test-integration # Run integration test (requires NDFC)
make build          # Build the application
make run            # Build and run the application
make fmt            # Format code
make vet            # Run go vet
make lint           # Run golangci-lint
make tidy           # Tidy go modules
make clean          # Clean build artifacts
make deps-up        # Start docker dependencies
make deps-down      # Stop docker dependencies
```

## Project Structure

```
go-nd/
├── cmd/
│   ├── server/             # Application entry point
│   ├── batch_test/         # Batch testing utility
│   └── integration_test/   # Integration tests
├── internal/
│   ├── cache/              # Valkey cache client & rate limiting
│   ├── config/             # Configuration management
│   ├── database/           # PostgreSQL connection and migrations
│   ├── handlers/           # HTTP handlers (fabric, compute, security, jobs)
│   ├── logger/             # Zap-based structured logging
│   ├── models/             # GORM models
│   ├── ndclient/           # Nexus Dashboard client
│   │   ├── common/         # Shared utilities
│   │   └── lanfabric/      # LAN fabric API client
│   ├── router/             # Gin router setup
│   ├── services/           # Business logic (job service, deploy batcher)
│   └── sync/               # Background sync workers
├── Dockerfile              # Container build
├── docker-compose.yml      # Local development stack
├── Makefile                # Build and test automation
└── go.mod                  # Go module definition
```

## License

MIT
