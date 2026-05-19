# Distributed Benchmarking Platform

A high-performance, distributed benchmarking system designed to stress-test and evaluate contestant-submitted trading infrastructure (Matching Engines/Orderbooks). 

The platform securely hosts contestant code in isolated, containerized environments within a Kubernetes cluster and uses a highly concurrent Go-based architecture to simulate massive market volatility (peak traffic) to grade systems on latency, throughput, and correctness.

---

## 🏗️ Architecture (Phase 1: Complete)

We have built the **Security & Execution Core** with a modern UI, file validation, and enterprise-grade Kubernetes orchestration. Our system is now capable of taking untrusted code, validating it, and running it safely in isolated Kubernetes pods without risking our infrastructure.

### Core Technologies
* **Backend:** Go 1.25 (Golang) with Go Fiber HTTP framework
* **Frontend:** React 19.2 with Vite 8.0 and Tailwind CSS
* **Orchestration:** Kubernetes (migrated from Docker) with in-cluster pod creation
* **Languages Supported:** C++, Go, Rust, Python
* **API:** RESTful HTTP server with FormData file uploads

### Data Flow Breakdown

**1. The Entry Point (UI & API)**
* **Frontend:** React-based dashboard with submit form featuring:
  * System name input
  * Language selector dropdown (C++, Go, Rust, Python)
  * Custom port selection (default 8080)
  * File upload with accept filter
  * Protocol and strategy specification
* **Backend:** Go server (`main.go`) on port 3000 listens for `POST /submit` requests
* **Result:** Extracts source code file and saves to `workspace/` folder

**2. Validation Layer (Dual-Stage)**
* **Client-Side:** JavaScript validates file extension matches selected language before submission
  * C++ accepts: `.cpp`, `.cc`, `.cxx`
  * Go accepts: `.go`
  * Rust accepts: `.rs`
  * Python accepts: `.py`
* **Server-Side:** Go backend re-validates file extension server-side for security
  * Returns 400 Bad Request if mismatch detected
  * Error message clearly identifies the problem

**3. The Sandbox Preparation**
* **Pathing:** Engine converts relative file path to Absolute Path for Kubernetes volume mounting
* **Resource Budgeting:** Pod defines resource contract:
  * **Memory:** Max 512 MB (limit enforced by K8s)
  * **CPU:** Max 1000 millicores (1.0 core)
  * **Network:** Isolated within cluster with controlled exposure via Service
* **Volume Setup:** Source code mounted as read-only HostPath volume at `/app`

**4. The Execution (Kubernetes)**
* **Pod Creation:** Engine creates Kubernetes pod named `sandbox-{port}-{unix-timestamp}` in `trading-sandbox` namespace
* **Image Selection:** Language-specific base image pulled:
  * C++: `gcc:latest` - Compiles with `g++ /app/file.cpp -o /tmp/run && /tmp/run`
  * Go: `golang:1.25` - Executes with `go run /app/file.go`
  * Rust: `rust:latest` - Compiles with `rustc /app/file.rs -o /tmp/run && /tmp/run`
  * Python: `python:3.12-slim` - Executes with `python /app/file.py`
* **Service Exposure:** NodePort Service created for container port binding
* **Isolation:** Pod runs with RestartPolicy=Never, no privileged access, network isolated to cluster

**5. The Feedback Loop**
* **Pod ID Tracking:** Kubernetes pod name returned to frontend as submission identifier
* **Log Capture:** Kubernetes logs accessible via `kubectl logs` for debugging
* **Cleanup:** Pods and Services can be deleted via kubectl or automatically via TTL
* **Response:** Frontend receives JSON with pod ID, language, port, and submission details

---

## 📂 Folder Structure

```text
trading-platform/
├── backend/
│   ├── cmd/
│   │   └── main.go                   # API Gateway & HTTP Server (port 3000)
│   ├── internal/
│   │   └── sandbox/
│   │       └── runner.go             # Kubernetes Pod Orchestration & Security Policies
│   ├── workspace/                    # Temporary isolated directory for uploads
│   ├── go.mod                        # Go dependencies (includes k8s.io packages)
│   └── go.sum
├── frontend/
│   ├── src/
│   │   ├── App.jsx                   # Root component with navigation
│   │   ├── App.css                   # Design system & glassmorphic styles
│   │   ├── index.css                 # Global styles and CSS variables
│   │   ├── main.jsx                  # Vite entry point
│   │   └── pages/
│   │       ├── Dashboard.jsx         # System overview with stats and CTA
│   │       └── SubmitPage.jsx        # Form for code submission with validation
│   ├── public/                       # Static assets
│   ├── package.json                  # Frontend dependencies
│   ├── vite.config.js                # Vite config with /api proxy to backend
│   └── README.md
├── infrastructure/
│   ├── k8s-namespace.yaml            # Kubernetes namespace + RBAC setup
│   ├── backend-deployment.yaml       # Kubernetes Deployment manifest
│   └── KUBERNETES_SETUP.md           # Deployment guide
├── docker-compose.yml                # Optional: local development orchestration
└── README.md
```

---

## ✨ Completed Phase 1 Features

### Backend (Go/Kubernetes)
- ✅ HTTP API server with multi-route support
- ✅ Kubernetes integration (replaces Docker)
- ✅ Multi-language support (C++, Go, Rust, Python)
- ✅ File validation (server-side extension checking)
- ✅ Pod creation with resource limits and isolation
- ✅ Service exposure via NodePort for port binding
- ✅ Error handling and status codes (400 for validation, 500 for execution)

### Frontend (React/Vite)
- ✅ Modern dashboard with hero section and stats
- ✅ Glassmorphic design system with blur effects
- ✅ Multi-page routing (Dashboard + Submit)
- ✅ Code submission form with:
  - Language selector dropdown
  - Custom port selection
  - File upload with client-side validation
  - Protocol and strategy fields
- ✅ File extension validation (matches language)
- ✅ Responsive design (mobile/tablet/desktop)
- ✅ Error and success feedback messages

### Infrastructure
- ✅ Kubernetes namespace creation script
- ✅ RBAC configuration (ServiceAccount, Role, RoleBinding)
- ✅ Backend Deployment manifest
- ✅ Comprehensive deployment documentation

---

## 🚀 Quick Start

### Prerequisites
- Go 1.25+
- Node.js 18+
- Kubernetes cluster (v1.27+)
- `kubectl` configured

### Local Development

```bash
# Terminal 1: Backend
cd trading-platform/backend
go mod tidy
go run ./cmd

# Terminal 2: Frontend (dev server on port 5174)
cd trading-platform/frontend
npm install
npm run dev
```

### Kubernetes Deployment

```bash
# Apply Kubernetes manifests
kubectl apply -f trading-platform/infrastructure/k8s-namespace.yaml
kubectl apply -f trading-platform/infrastructure/backend-deployment.yaml

# Verify
kubectl get pods -n trading-sandbox
```

---

## 🔐 Security Model

- **Untrusted Code Execution:** Runs in isolated Kubernetes pods with no host access
- **Resource Limits:** Memory capped at 512MB, CPU at 1000m per pod
- **Network Isolation:** Pods networked within cluster, no external egress by default
- **File Permissions:** Source code mounted read-only, cannot be modified by container
- **Language Validation:** Dual-layer (client + server) prevents execution mismatches