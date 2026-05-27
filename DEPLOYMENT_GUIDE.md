# Backend Deployment Guide

## Quick Start

### Prerequisites

Ensure you have:

- Docker Desktop (or similar) with Kubernetes enabled
- `kubectl` configured to access your Kubernetes cluster
- Go 1.25+ (for local development)
- PowerShell or Bash shell

### Build and Deploy in One Command

**Windows (PowerShell):**

```powershell
.\build-and-deploy.ps1
```

**macOS/Linux (Bash):**

```bash
./build-and-deploy.sh
```

## Step-by-Step Deployment

### 1. Build the Docker Image

Navigate to the trading-platform directory and build the backend image:

```bash
cd trading-platform/backend
docker build -t trading-platform/backend:minikube -f Dockerfile .
cd ../..
```

### 2. Create Kubernetes Namespace and RBAC

Apply the namespace, ServiceAccount, Role, and RoleBinding:

```bash
kubectl apply -f infrastructure/k8s-namespace.yaml
```

This creates:

- `trading-sandbox` namespace
- `trading-backend` ServiceAccount with permissions to create/delete pods and services
- Required RBAC roles and bindings

### 3. Deploy the Backend

Apply the backend deployment:

```bash
kubectl apply -f infrastructure/backend-deployment.yaml
```

This creates:

- A Kubernetes Deployment for the backend
- A LoadBalancer Service exposing the API on port 3000

### 4. Verify Deployment

Check the deployment status:

```bash
kubectl rollout status deployment/trading-backend -n trading-sandbox
```

Check pod status:

```bash
kubectl get pods -n trading-sandbox
```

View logs:

```bash
kubectl logs -n trading-sandbox -l app=trading-platform,component=backend -f
```

## Architecture

The backend is now deployed as a Kubernetes Deployment with the following configuration:

- **Image**: `trading-platform/backend:minikube` (built from Dockerfile)
- **Replicas**: 1
- **Namespace**: `trading-sandbox`
- **ServiceAccount**: `trading-backend` (has permissions to create sandbox pods)
- **Resource Limits**:
  - Memory: 512Mi limit, 256Mi request
  - CPU: 500m limit, 250m request
- **Health Checks**:
  - Liveness probe: `/health` endpoint every 20 seconds
  - Readiness probe: `/health` endpoint every 10 seconds
- **Port**: 3000 (exposed via LoadBalancer service)

## How It Works

1. **Submission**: Frontend sends code via POST to `/submit`
2. **Backend Processing**: Backend receives the file and creates a Kubernetes Pod
3. **Pod Creation**: A new sandbox pod is created with:
   - Language-specific image (gcc, golang, rust, python)
   - Code mounted as read-only volume
   - Isolated network and resources
4. **Execution**: Code runs inside the pod
5. **Result**: Pod name returned as submission ID

## Kubernetes Pod Isolation

Each submission creates an isolated Kubernetes Pod with:

- Read-only volume mount for source code
- Resource limits (memory and CPU)
- Network policies (configurable)
- Automatic cleanup (via pod cleanup job, if configured)

## Health Check

The backend exposes a health check endpoint at `/health` that returns:

```json
{
  "status": "healthy"
}
```

This is used by Kubernetes probes to determine if the pod is:

- **Alive** (liveness probe)
- **Ready** to serve traffic (readiness probe)

## Troubleshooting

### Backend Pod Not Starting

Check logs:

```bash
kubectl logs -n trading-sandbox deploy/trading-backend
```

Describe deployment:

```bash
kubectl describe deployment trading-backend -n trading-sandbox
```

### Image Pull Failures

If using a registry other than Docker Hub, ensure:

1. Image is properly tagged
2. Image exists in the registry
3. Kubernetes has imagePullSecrets configured (if using private registry)

### Port Already in Use

If port 3000 is already in use:

```bash
# Find and kill process on port 3000
# Windows:
Get-NetTCPConnection -LocalPort 3000 | Stop-Process -Force

# macOS/Linux:
lsof -i :3000 | grep LISTEN | awk '{print $2}' | xargs kill -9
```

### In-Cluster Configuration Error

If you see: `unable to load in-cluster configuration, KUBERNETES_SERVICE_HOST and KUBERNETES_SERVICE_PORT must be defined`

This means:

1. The backend pod is not running in the Kubernetes cluster
2. Or the ServiceAccount is not properly configured
3. Or the pod is running in a different namespace

**Solution**: Ensure the pod is running in the `trading-sandbox` namespace with the `trading-backend` ServiceAccount:

```bash
kubectl get pods -n trading-sandbox -o wide
kubectl describe pod -n trading-sandbox <pod-name>
```

## Local Development (Without Kubernetes)

To run the backend locally (not in Kubernetes):

```bash
cd trading-platform/backend

# Install dependencies
go mod download

# Run directly
go run ./cmd/main.go

# Or build and run
go build -o trading-backend ./cmd/main.go
./trading-backend
```

The backend will run on `http://localhost:3000` and sandboxing will fail (since Docker SDK isn't set up for local runs).

## Next Steps

1. Deploy the frontend
2. Set up logging and monitoring
3. Configure ingress for external access
4. Set up persistent storage for submission records
5. Configure automatic pod cleanup and log collection

See [KUBERNETES_SETUP.md](trading-platform/infrastructure/KUBERNETES_SETUP.md) for more details.
