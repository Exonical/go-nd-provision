# Go Nexus Dashboard

A Go application for interacting with Cisco Nexus Dashboard to manage Security Associations, Security Contracts, and Security Groups using Port Selectors.

## Features

- **Fabric Management**: Query fabrics, switches, and switch ports from Nexus Dashboard (new API)
- **Security Management**: Create Security Groups, Contracts, and Associations (Legacy 3.x API)
- **Compute Node Mapping**: Track which compute nodes (servers) are connected to which switch ports
- **Caching**: Valkey (Redis-compatible) caching layer
- **Database**: PostgreSQL 18 with GORM ORM

## Prerequisites

- Go 1.21+
- PostgreSQL 18
- Valkey (or Redis-compatible server)
- Cisco Nexus Dashboard access

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

4. Run the application:
```bash
go run main.go
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

## API Endpoints

### Fabrics (New API)

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/v1/fabrics` | List all fabrics |
| `GET` | `/api/v1/fabrics/:id` | Get fabric by ID |
| `POST` | `/api/v1/fabrics/sync` | Sync fabrics from ND |
| `GET` | `/api/v1/fabrics/:id/switches` | List switches in fabric |
| `GET` | `/api/v1/fabrics/:id/switches/:switchId` | Get switch by ID |
| `POST` | `/api/v1/fabrics/:id/switches/sync` | Sync switches from ND |
| `GET` | `/api/v1/fabrics/:id/switches/:switchId/ports` | List switch ports |
| `POST` | `/api/v1/fabrics/:id/switches/:switchId/ports` | Create switch port |
| `POST` | `/api/v1/fabrics/:id/switches/:switchId/ports/sync` | Sync ports from ND |

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
| `GET` | `/api/v1/security/groups/:id` | Get security group |
| `POST` | `/api/v1/security/groups` | Create security group |
| `DELETE` | `/api/v1/security/groups/:id` | Delete security group |

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

## Example Usage

### Create a Compute Node

```bash
curl -X POST http://localhost:8080/api/v1/compute-nodes \
  -H "Content-Type: application/json" \
  -d '{
    "name": "server-01",
    "hostname": "server-01.example.com",
    "ip_address": "10.0.0.10",
    "mac_address": "00:11:22:33:44:55"
  }'
```

### Map Compute Node to Switch Port

```bash
curl -X POST http://localhost:8080/api/v1/compute-nodes/{node_id}/port-mappings \
  -H "Content-Type: application/json" \
  -d '{
    "switch_port_id": "port-uuid",
    "nic_name": "eth0",
    "vlan": 100
  }'
```

### Create Security Group with Port Selector

```bash
curl -X POST http://localhost:8080/api/v1/security/groups \
  -H "Content-Type: application/json" \
  -d '{
    "name": "web-servers",
    "description": "Web server security group",
    "fabric_id": "fabric-uuid",
    "selectors": [
      {
        "switch_port_id": "port-uuid",
        "expression": ""
      }
    ]
  }'
```

### Create Security Contract

```bash
curl -X POST http://localhost:8080/api/v1/security/contracts \
  -H "Content-Type: application/json" \
  -d '{
    "name": "allow-http",
    "description": "Allow HTTP traffic",
    "fabric_id": "fabric-uuid",
    "rules": [
      {
        "name": "http",
        "action": "permit",
        "protocol": "tcp",
        "dst_port": "80",
        "priority": 100
      }
    ]
  }'
```

### Create Security Association

```bash
curl -X POST http://localhost:8080/api/v1/security/associations \
  -H "Content-Type: application/json" \
  -d '{
    "name": "web-to-db",
    "description": "Web servers to database",
    "fabric_id": "fabric-uuid",
    "provider_group_id": "provider-group-uuid",
    "consumer_group_id": "consumer-group-uuid",
    "security_contract_id": "contract-uuid"
  }'
```

## Project Structure

```
go-nd/
├── main.go                 # Application entry point
├── go.mod                  # Go module definition
└── internal/
    ├── config/
    │   └── config.go       # Configuration management
    ├── database/
    │   └── database.go     # PostgreSQL connection and migrations
    ├── cache/
    │   └── valkey.go       # Valkey cache client
    ├── models/
    │   └── models.go       # GORM models
    ├── ndclient/
    │   ├── client.go       # Nexus Dashboard HTTP client
    │   ├── fabric.go       # Fabric/Switch/Port API (new)
    │   └── security.go     # Security API (Legacy 3.x)
    ├── handlers/
    │   ├── fabric.go       # Fabric HTTP handlers
    │   ├── compute.go      # Compute node handlers
    │   └── security.go     # Security handlers
    └── router/
        └── router.go       # Gin router setup
```

## License

MIT
