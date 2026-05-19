#!/bin/bash

# Build and Deploy Backend to Kubernetes
# This script builds the Docker image and deploys it to Kubernetes

set -e

# Configuration
BACKEND_DIR="./trading-platform/backend"
IMAGE_NAME="trading-platform/backend"
IMAGE_TAG="latest"
FULL_IMAGE="$IMAGE_NAME:$IMAGE_TAG"
NAMESPACE="trading-sandbox"

echo "🔨 Building Docker image: $FULL_IMAGE"

# Build the Docker image
if [ -d "$BACKEND_DIR" ]; then
    pushd "$BACKEND_DIR" > /dev/null
    docker build -t "$FULL_IMAGE" -f Dockerfile .
    if [ $? -ne 0 ]; then
        echo "❌ Docker build failed!"
        exit 1
    fi
    popd > /dev/null
else
    echo "❌ Backend directory not found: $BACKEND_DIR"
    exit 1
fi

echo "✅ Docker image built successfully"

echo "🚀 Applying Kubernetes namespace and RBAC..."
kubectl apply -f infrastructure/k8s-namespace.yaml
if [ $? -ne 0 ]; then
    echo "❌ Failed to apply namespace configuration!"
    exit 1
fi

echo "🚀 Deploying backend to Kubernetes..."
kubectl apply -f infrastructure/backend-deployment.yaml
if [ $? -ne 0 ]; then
    echo "❌ Failed to deploy backend!"
    exit 1
fi

echo "✅ Backend deployed successfully"

echo "📊 Checking deployment status..."
kubectl rollout status deployment/trading-backend -n "$NAMESPACE" --timeout=2m

echo "✅ All done! Backend is running in Kubernetes"
echo ""
echo "Useful commands:"
echo "  View logs:          kubectl logs -n $NAMESPACE -l app=trading-platform,component=backend -f"
echo "  Describe pods:      kubectl describe pods -n $NAMESPACE"
echo "  Port forward:       kubectl port-forward -n $NAMESPACE svc/trading-backend 3000:3000"
echo "  Delete deployment:  kubectl delete deployment trading-backend -n $NAMESPACE"
