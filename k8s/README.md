# Kubernetes/Podman Manifests for go-nd

This directory contains Kubernetes manifests that can be applied using Podman Desktop or kubectl.

## Prerequisites

1. Build the go-nd container image:
   ```bash
   podman build -t go-nd:latest .
   ```

2. For Podman Desktop, ensure you have a Kubernetes cluster running (Kind, Minikube, or Podman's built-in Kubernetes).

## Applying Manifests

### Using Kustomize (Recommended)

```bash
# Preview what will be applied
kubectl kustomize k8s/

# Apply all manifests
kubectl apply -k k8s/
```

### Using Podman Desktop

1. Open Podman Desktop
2. Navigate to **Kubernetes** > **Apply YAML**
3. Select the `k8s/` directory or individual YAML files
4. Click **Apply**

### Manual Application Order

If applying manually without Kustomize:

```bash
kubectl apply -f k8s/namespace.yaml
kubectl apply -f k8s/configmap.yaml
kubectl apply -f k8s/secrets.yaml
kubectl apply -f k8s/postgres.yaml
kubectl apply -f k8s/valkey.yaml
kubectl apply -f k8s/app.yaml
kubectl apply -f k8s/nginx.yaml
```

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                        Namespace: go-nd                      │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  ┌─────────────┐                                            │
│  │   Nginx     │ NodePort: 30080 (HTTP), 30051 (gRPC)       │
│  │ Load Balancer│                                            │
│  └──────┬──────┘                                            │
│         │                                                    │
│         ▼                                                    │
│  ┌──────────────────────────────────────────┐               │
│  │         StatefulSet: go-nd-app           │               │
│  │  ┌─────────┐ ┌─────────┐ ┌─────────┐    │               │
│  │  │ app-0   │ │ app-1   │ │ app-2   │    │               │
│  │  │ :8080   │ │ :8080   │ │ :8080   │    │               │
│  │  │ :50051  │ │ :50051  │ │ :50051  │    │               │
│  │  └────┬────┘ └────┬────┘ └────┬────┘    │               │
│  └───────┼───────────┼───────────┼──────────┘               │
│          │           │           │                           │
│          ▼           ▼           ▼                           │
│  ┌───────────────┐  ┌───────────────┐                       │
│  │   PostgreSQL  │  │    Valkey     │                       │
│  │   (postgres)  │  │   (valkey)    │                       │
│  │    :5432      │  │    :6379      │                       │
│  └───────────────┘  └───────────────┘                       │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

## Components

| Component | Type | Replicas | Ports | Description |
|-----------|------|----------|-------|-------------|
| nginx | Deployment | 1 | 80, 50051 | Load balancer for HTTP and gRPC |
| go-nd-app | StatefulSet | 3 | 8080, 50051 | Application instances |
| postgres | Deployment | 1 | 5432 | PostgreSQL database |
| valkey | Deployment | 1 | 6379 | Valkey (Redis-compatible) cache |

## Accessing the Application

After deployment:

- **HTTP API**: `http://localhost:30080` (or your node IP)
- **gRPC API**: `localhost:30051`

## Configuration

### Secrets (k8s/secrets.yaml)

Update these values before deploying to production:

- `DB_PASSWORD`: PostgreSQL password
- `VALKEY_PASSWORD`: Valkey password
- `GRPC_AUTH_TOKEN`: gRPC authentication token
- `ND_USERNAME`, `ND_PASSWORD`, `ND_API_KEY`: Nexus Dashboard credentials

### ConfigMap (k8s/configmap.yaml)

Application configuration including:

- Feature flags (`ENABLE_HTTP`, `ENABLE_GRPC`, `ENABLE_SYNC`)
- Database and Valkey connection settings
- Nginx load balancer configuration

## Scaling

To scale the application:

```bash
kubectl scale statefulset go-nd-app -n go-nd --replicas=5
```

Note: Update the nginx ConfigMap to include the new pod hostnames in the upstream configuration.

## Cleanup

```bash
kubectl delete -k k8s/
```

Or to delete everything including PVCs:

```bash
kubectl delete namespace go-nd
```
