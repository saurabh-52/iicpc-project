# 🚀 Distributed Benchmarking Platform

> A highly concurrent, resilient, and distributed platform designed to stress-test and evaluate contestant-submitted trading infrastructure.

## 📖 Overview

Building a matching engine or orderbook is a complex engineering challenge that requires mastery of low-level data structures, process synchronization, and concurrency. The **Distributed Benchmarking Platform** acts as the ultimate proving ground for these systems. 

Instead of merely checking if the code compiles or passes static test cases, this platform dynamically spawns a massive fleet of simulated "trading bots." These bots bombard the contestant's hosted code with concurrent orders to simulate peak market volatility. The system then captures granular telemetry to assess the submitted code on latency, throughput, and correctness, streaming the results to a live, dynamic leaderboard.

---

## ✨ Key Features

* **🛡️ Secure Code Sandboxing:** Safely executes untrusted C++, Go, or Rust code using the Docker SDK, enforcing strict resource limitations (cgroups) and network isolation.
* **🌐 Distributed Load Generation:** Utilizes Go's lightweight goroutines to spawn thousands of concurrent simulated market participants (bots) that hammer the target engine with high-velocity requests.
* **⚡ Ultra-Low Latency Telemetry:** Captures and buffers order acknowledgments using Redis Streams to ensure the benchmarking engine itself does not become the bottleneck.
* **📊 Real-Time Analytics:** Pushes live $p50$, $p90$, and $p99$ latency metrics and Transactions Per Second (TPS) to a React frontend via WebSockets.
* **⚖️ Deterministic Verification:** Acts as a "Special Judge" to ensure price-time priority and trade execution accuracy are mathematically flawless.

---

## 🏗️ System Architecture

Our platform is decoupled into four highly specialized micro-components to ensure maximum performance and scalability.

### 1. The Submission & Sandboxing Engine (The "Safe Room")
* **Role:** Securely ingests, compiles, and runs contestant binaries.
* **Mechanics:** Uses the Go Docker SDK to spin up ephemeral, locked-down containers. It completely disables external networking (`--network="none"`) and strictly pins CPU ($1.0$ core) and Memory ($512$ MB) to ensure a standardized, un-cheatable benchmarking environment.

### 2. The Bot Fleet (The Load Generator)
* **Role:** Simulates the chaotic environment of a real-world financial exchange.
* **Mechanics:** A horizontally scalable Go service that generates high-throughput traffic. It simulates various order types (Limit, Market, Cancel) and connects to the sandboxed engine via Unix Domain Sockets or optimized TCP bridges.

### 3. Telemetry & Validation Ingester (The Referee)
* **Role:** Scores the submission based on speed and mathematical accuracy.
* **Mechanics:** Reads the output stream from the contestant's engine into a Redis buffer. A dedicated Go worker verifies the orderbook state (ensuring no race conditions occurred) and calculates latency percentiles and peak TPS.

### 4. Real-Time Leaderboard (The Scoreboard)
* **Role:** The public-facing dashboard for the competition.
* **Mechanics:** A React/Vite frontend that listens to WebSocket events pushed from the Telemetry engine, dynamically re-ranking contestants based on a composite score of speed, stability, and algorithmic correctness.

---

## 🛠️ Technology Stack

**Backend & Infrastructure**
* **Language:** Go (Golang) - *Chosen for native concurrency and Docker SDK integration.*
* **API Framework:** Go Fiber
* **Containerization:** Docker
* **Message Buffer:** Redis
* **Persistent Storage:** PostgreSQL

**Frontend**
* **Framework:** React + TypeScript (via Vite)
* **Styling:** Tailwind CSS
* **Live Data:** WebSockets + Recharts

---

## 🔄 Data Flow: The Benchmarking Lifecycle

1. **Ingestion:** A contestant submits their source code via the REST API.
2. **Isolation:** The platform mounts the code into an ephemeral Docker volume, compiles it, and starts the process under strict resource constraints.
3. **Bombardment:** The Bot Fleet initiates thousands of concurrent TCP/WebSocket connections, flooding the process with market data.
4. **Capture:** Every response from the contestant's code is time-stamped and piped into Redis.
5. **Validation:** The Telemetry engine analyzes the Redis stream, checking the math against a known-good ledger and calculating the processing speed.
6. **Broadcast:** The final composite score is saved to PostgreSQL and broadcasted to the frontend UI via WebSockets.

---

## 🚀 Getting Started

### Prerequisites
* Go (v1.20+)
* Docker Desktop (Running)
* Node.js (v18+)
* Redis & PostgreSQL (Can be spun up via Docker Compose)

### Installation

1. **Clone the repository:**
   ```bash
   git clone https://github.com/yourusername/trading-platform.git
   cd trading-platform

2. **Start the Infrastructure (Redis/DB):**
   ```bash
   docker-compose up -d

3. **Start the Backend:**
   ```bash
   cd backend
   go mod tidy
   go run cmd/main.go

4. **Test the Sandbox API:**
    ```bash
    curl -X POST http://localhost:3000/submit -F "source_code=@examples/test_orderbook.cpp"


# 🗺️ Project Roadmap

* [x] **Phase 1: Sandboxing Engine** - Implemented secure code execution, cgroups limitations, and log streaming via the Go Docker SDK.
* [ ] **Phase 2: Bot Fleet** - Build the distributed load generator using Go goroutines to simulate concurrent market participants.
* [ ] **Phase 3: Telemetry Engine** - Integrate Redis streams and build the deterministic validation worker to calculate TPS and accuracy.
* [ ] **Phase 4: Live Leaderboard** - Develop the React dashboard and wire up WebSocket broadcasting for real-time ranking.
