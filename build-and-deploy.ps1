# Build and Deploy Backend to Kubernetes
# This script builds the Docker image and deploys it to Kubernetes

$ErrorActionPreference = "Stop"

# Configuration
$BACKEND_DIR = ".\trading-platform\backend"
$IMAGE_NAME = "trading-platform/backend"
$IMAGE_TAG = "minikube"
$FULL_IMAGE = "$IMAGE_NAME`:$IMAGE_TAG"
$NAMESPACE = "trading-sandbox"

Write-Host "🔨 Building Docker image: $FULL_IMAGE" -ForegroundColor Cyan

# Build the Docker image
if (Test-Path $BACKEND_DIR) {
    Push-Location $BACKEND_DIR
    docker build -t $FULL_IMAGE -f Dockerfile .
    if ($LASTEXITCODE -ne 0) {
        Write-Host "❌ Docker build failed!" -ForegroundColor Red
        exit 1
    }
    Pop-Location
} else {
    Write-Host "❌ Backend directory not found: $BACKEND_DIR" -ForegroundColor Red
    exit 1
}

Write-Host "✅ Docker image built successfully" -ForegroundColor Green

Write-Host "🚀 Applying Kubernetes namespace and RBAC..." -ForegroundColor Cyan
kubectl apply -f infrastructure/k8s-namespace.yaml
if ($LASTEXITCODE -ne 0) {
    Write-Host "❌ Failed to apply namespace configuration!" -ForegroundColor Red
    exit 1
}

Write-Host "🚀 Deploying backend to Kubernetes..." -ForegroundColor Cyan
kubectl apply -f infrastructure/backend-deployment.yaml
if ($LASTEXITCODE -ne 0) {
    Write-Host "❌ Failed to deploy backend!" -ForegroundColor Red
    exit 1
}

Write-Host "✅ Backend deployed successfully" -ForegroundColor Green

Write-Host "📊 Checking deployment status..." -ForegroundColor Cyan
kubectl rollout status deployment/trading-backend -n $NAMESPACE --timeout=2m

Write-Host "✅ All done! Backend is running in Kubernetes" -ForegroundColor Green
Write-Host ""
Write-Host "Useful commands:" -ForegroundColor Yellow
Write-Host "  View logs:          kubectl logs -n $NAMESPACE -l app=trading-platform,component=backend -f"
Write-Host "  Describe pods:      kubectl describe pods -n $NAMESPACE"
Write-Host "  Port forward:       kubectl port-forward -n $NAMESPACE svc/trading-backend 3000:3000"
Write-Host "  Delete deployment:  kubectl delete deployment trading-backend -n $NAMESPACE"
