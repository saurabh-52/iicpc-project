# Kubernetes Setup Guide

The trading platform backend has been migrated from Docker to Kubernetes orchestration for improved container management and scalability.

## Architecture Overview

- **Orchestration**: Kubernetes pods created per submission
- **Pod Naming**: `sandbox-{port}-{unix-timestamp}`
- **Namespace**: `trading-sandbox`
- **Service Exposure**: NodePort service for port binding
- **Volume Management**: HostPath volumes for source code (read-only)

## Prerequisites

- Kubernetes cluster running (v1.27+)
- `kubectl` configured to access your cluster
- Backend service running outside the cluster with access to a kubeconfig file

## Deployment Steps

### 1. Create Namespace and RBAC Configuration

Apply the namespace, ServiceAccount, Role, and RoleBinding:

```bash
kubectl apply -f infrastructure/k8s-namespace.yaml
```

This creates:
- `trading-sandbox` namespace
- `trading-backend` ServiceAccount
- `trading-backend-executor` Role with permissions to:
  - Create/delete Pods and Services
  - View pod logs
- RoleBinding connecting the ServiceAccount to the Role

### 2. Deploy Backend Service

Run the backend as a local process or on a separate machine with kubeconfig access:

```bash
kubectl apply -f infrastructure/backend-deployment.yaml
```

The backend process should have:
- `KUBECONFIG` pointing to a valid kubeconfig, or `~/.kube/config` available
- Access to the `trading-sandbox` namespace
- Port `3000` exposed for the API server if needed

### 3. Verify Setup

Check that the namespace and RBAC are configured:

```bash
# List namespace
kubectl get namespace trading-sandbox

# List ServiceAccount
kubectl get serviceaccount -n trading-sandbox

# Verify Role
kubectl get role -n trading-sandbox

# Verify RoleBinding
kubectl get rolebinding -n trading-sandbox
```

## How It Works

1. **Submission**: Frontend sends code via `/api/submit` → Backend receives on `:3000`
2. **Pod Creation**: Backend uses kubeconfig to authenticate to Kubernetes API
3. **Pod Lifecycle**: 
   - Pod created with language-specific image (gcc, golang, rust, python)
   - Source code mounted as read-only HostPath volume at `/app`
   - Container executes code
   - Service created for port exposure (NodePort)
   - Pod name returned to frontend as submission ID
4. **Resource Limits**: Pods limited to 512MB memory and 1000m CPU

## Language Support

- **C++**: `gcc:latest` — Compiles with `g++ /app/file.cpp -o /tmp/run && /tmp/run`
- **Go**: `golang:1.25` — Executes with `go run /app/file.go`
- **Rust**: `rust:latest` — Compiles with `rustc /app/file.rs -o /tmp/run && /tmp/run`
- **Python**: `python:3.12-slim` — Executes with `python /app/file.py`

## Troubleshooting

### Backend can't create pods

Check:
1. ServiceAccount has correct permissions: `kubectl describe role trading-backend-executor -n trading-sandbox`
2. Backend pod is using the ServiceAccount: Check pod manifest
3. Verify RBAC RoleBinding: `kubectl get rolebinding -n trading-sandbox`

### Pods not starting

Check logs:
```bash
kubectl logs -n trading-sandbox <pod-name>
```

Describe pod for events:
```bash
kubectl describe pod -n trading-sandbox <pod-name>
```

### Image pull failures

Ensure image registry is accessible from cluster nodes:
```bash
kubectl get nodes
# Run on node: docker pull gcc:latest (or specific image)
```

## Environment Variables

When running backend locally or on a separate host, ensure:
- `KUBECONFIG` is set, or `~/.kube/config` exists on that host
- Backend has network connectivity to the Kubernetes API server

## Next Steps

1. Create `infrastructure/backend-deployment.yaml` to deploy the backend service
2. Configure ingress/load balancer for frontend access
3. Set up logging and monitoring (kubectl logs, ELK stack, Prometheus, etc.)
4. Configure persistent storage if needed for long-term submission records
