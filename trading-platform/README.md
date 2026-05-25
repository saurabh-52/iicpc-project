 Trading Platform - Distributed Benchmarking System

A production-ready distributed benchmarking platform for stress-testing and evaluating trader-submitted code (Matching Engines, Order Books, etc.). Built with modern infrastructure: **Kubernetes orchestration, React frontend, Go backend**.

## 🎯 Quick Links

- **Phase 1 Details:** [phase1.md](../phase1.md) — Complete architecture and security model
- **Implementation Plan:** [implementation_plan.md](../implemetaion_plan.md) — Roadmap to Phase 4
- **Kubernetes Setup:** [infrastructure/KUBERNETES_SETUP.md](infrastructure/KUBERNETES_SETUP.md) — Deployment guide

---

## 🏗️ Project Structure

```
trading-platform/
├── backend/              # Go API server & Kubernetes orchestrator (port 3000)
│   ├── cmd/main.go       # HTTP server, routes, file handling
│   ├── internal/sandbox/runner.go  # K8s pod orchestration
│   ├── workspace/        # Temporary upload directory
│   ├── go.mod
│   └── go.sum
├── frontend/             # React + Vite UI (dev server port 5174)
│   ├── src/
│   │   ├── App.jsx       # Root component with routing
│   │   ├── App.css       # Glassmorphic design system
│   │   ├── index.css     # Global styles
│   │   └── pages/
│   │       ├── Dashboard.jsx    # System overview
│   │       └── SubmitPage.jsx   # Code submission form
│   ├── package.json
│   ├── vite.config.js
│   └── README.md
├── infrastructure/       # Kubernetes deployment files
│   ├── k8s-namespace.yaml        # Namespace + RBAC
│   ├── backend-deployment.yaml   # Deployment manifest
│   └── KUBERNETES_SETUP.md       # Full setup guide
├── docker-compose.yml    # Local dev orchestration
└── README.md             # This file
```

---

## 🚀 Getting Started

### Prerequisites

- **Go:** v1.25.0 or higher
- **Node.js:** v18 or higher
- **Docker:** (for local development)
- **Kubernetes:** v1.27+ (for production deployment)

### Local Development

#### 1. Backend (Go)

```bash
cd backend
go mod tidy
go run ./cmd
```

Server runs on `http://localhost:3000`

#### 2. Frontend (React + Vite)

```bash
cd frontend
npm install
npm run dev
```

Dev server runs on `http://localhost:5174` with `/api` proxy to backend.

#### 3. Test Submission

1. Open `http://localhost:5174`
2. Navigate to **Submit** page
3. Select language (C++, Go, Rust, Python)
4. Upload matching file (e.g., `.cpp` for C++)
5. Click Submit
6. See pod ID in response

### Production: Kubernetes Deployment

#### 1. Apply Kubernetes Configuration

```bash
# Create namespace and RBAC
kubectl apply -f infrastructure/k8s-namespace.yaml

# Deploy backend
kubectl apply -f infrastructure/backend-deployment.yaml
```

#### 2. Verify Setup

```bash
# Check namespace
kubectl get namespace trading-sandbox

# Check deployment
kubectl get deployment -n trading-sandbox

# Check pods
kubectl get pods -n trading-sandbox

# View logs
kubectl logs -n trading-sandbox -l app=trading-platform,component=backend
```

#### 3. Access Backend

The backend Service is exposed as `LoadBalancer` (or `NodePort` if LoadBalancer unavailable). To find the endpoint:

```bash
kubectl get svc -n trading-sandbox trading-backend
```

---

## 📡 API Reference

### POST /submit

Submit code for execution.

**Request:**
```
FormData:
  - source_code (file)    : The code file (.cpp, .go, .rs, .py)
  - language (string)     : Language (cpp, go, rust, python)
  - port (int)            : Container port (1-65535)
  - systemName (string)   : System identifier
  - protocol (string)     : Protocol (e.g., "TCP")
  - strategy (string)     : Strategy description (optional)
```

**Response (200):**
```json
{
  "message": "Sandbox started successfully",
  "containerId": "sandbox-8080-1716123456",
  "language": "cpp",
  "port": 8080,
  "file": "main.cpp"
}
```

**Response (400):**
```json
{
  "error": "file extension .txt does not match selected language cpp"
}
```

**Response (500):**
```json
{
  "error": "failed to create pod: <details>"
}
```

---

## 🔐 Security Features

### File Validation
- **Dual-layer:** Client-side + server-side extension checking
- **Whitelisted Extensions:**
  - C++: `.cpp`, `.cc`, `.cxx`
  - Go: `.go`
  - Rust: `.rs`
  - Python: `.py`

### Pod Isolation
- **Resource Limits:** 512MB memory, 1000m CPU per pod
- **Volume Mounts:** Read-only source code (cannot be modified)
- **Network:** Isolated to cluster, no external egress by default
- **Restart Policy:** Never (pod terminates after execution)

### Authentication
- Kubernetes RBAC via ServiceAccount
- Backend uses kubeconfig from the machine running the backend
- Role permissions limited to pod/service CRUD in `trading-sandbox` namespace

---

## 🛠️ Supported Languages

| Language | Base Image | Command |
|----------|-----------|---------|
| C++ | `gcc:latest` | `g++ /app/file.cpp -o /tmp/run && /tmp/run` |
| Go | `golang:1.25` | `go run /app/file.go` |
| Rust | `rust:latest` | `rustc /app/file.rs -o /tmp/run && /tmp/run` |
| Python | `python:3.12-slim` | `python /app/file.py` |

---

## 📊 UI Features

### Dashboard
- Hero section with CTA button ("Submit an engine")
- Stats grid: Active engines, runs today, avg. latency, sandbox status
- Timeline of recent operations
- System health indicator

### Submit Form
- Language dropdown with dynamic file validation
- Port selection (custom or default 8080)
- File upload with extension matching
- Protocol and strategy fields
- Real-time validation feedback
- Success/error messages

---

## 🔧 Development

### Frontend Development Commands
```bash
cd frontend
npm run dev       # Start Vite dev server
npm run build     # Build for production
npm run lint      # Run ESLint
```

### Backend Development Commands
```bash
cd backend
go mod tidy       # Sync dependencies
go build ./...    # Build all packages
go run ./cmd      # Run API server
go test ./...     # Run tests
```

### Kubernetes Debugging
```bash
# View pod logs
kubectl logs -n trading-sandbox <pod-name>

# Describe pod
kubectl describe pod -n trading-sandbox <pod-name>

# Access pod shell (if debugging image has shell)
kubectl exec -it -n trading-sandbox <pod-name> -- /bin/bash

# Get pod events
kubectl get events -n trading-sandbox
```

---

## 📝 Configuration

### Backend Environment Variables
- `PORT` (default `3000`): HTTP server port
- Inside K8s, pods created in `trading-sandbox` namespace with labels: `app=trading-sandbox`, `port={port}`

### Frontend Configuration
- **Vite proxy:** `/api` → `http://localhost:3000` (removes `/api` prefix)
- **Build output:** `frontend/dist/`

---

## 🚢 Deployment Checklist

- [ ] Kubernetes cluster configured and accessible
- [ ] `kubectl` authenticated to cluster
- [ ] Apply namespace + RBAC: `kubectl apply -f infrastructure/k8s-namespace.yaml`
- [ ] Apply backend deployment: `kubectl apply -f infrastructure/backend-deployment.yaml`
- [ ] Verify pods running: `kubectl get pods -n trading-sandbox`
- [ ] Frontend built: `npm run build` in `frontend/`
- [ ] Frontend deployed to ingress/load balancer
- [ ] API endpoint accessible from frontend

---

## 📚 Next Phases (Planned)

### Phase 2: Bot Fleet
- Concurrent load generation with Go goroutines
- Simulate market volatility (10k+ req/sec)
- Unix socket or TCP communication to test containers

### Phase 3: Telemetry & Validation
- Redis Streams for result capture
- Order book math verification
- Latency and throughput scoring

### Phase 4: Live Leaderboard
- WebSocket streaming of results
- Real-time React dashboard updates
- Historical submission analytics

---

## 📄 License

(License TBD)

---

## 🤝 Support

For detailed architecture and security information, see [phase1.md](../phase1.md).  
For deployment troubleshooting, see [infrastructure/KUBERNETES_SETUP.md](infrastructure/KUBERNETES_SETUP.md).
