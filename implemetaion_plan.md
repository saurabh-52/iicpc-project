# Implementation Plan: Distributed Benchmarking Platform

## 1. Project Infrastructure
* **/backend:** Go (API & Orchestrator)
* **/bot-fleet:** Go (Load Generators)
* **/frontend:** React + Tailwind (Dashboard)
* **Databases:** PostgreSQL (Profiles/History), Redis (High-speed buffer)

## 2. Phase 1: Sandboxing Engine
* **Goal:** Securely execute contestant code.
* **Tech:** Go Docker SDK.
* **Steps:** Build binary -> Run in isolated container (`--network none`, cgroups for memory/CPU limits).
## 2. Phase 1: Sandboxing Engine (completed)
- **Goal:** Securely execute contestant code in isolated runtime environments.
- **Tech:** Kubernetes pods (Go implementation in backend/internal/sandbox).
- **What we implemented:**
	- `ExecuteCode()` that creates a Kubernetes Pod and a matching Service in the `trading-sandbox` namespace.
	- Pod mounts the submission directory as a read-only hostPath volume and runs a language-specific command.
	- Language support implemented: C++ (gcc), Go, Rust, Python — compiled or executed inside the container.
	- Pods are created with `RestartPolicyNever`; a NodePort `Service` is created to expose the runtime port.
	- The implementation is in `trading-platform/backend/internal/sandbox/runner.go`.
- **Notes / differences from original plan:**
	- We used Kubernetes pods instead of the Docker SDK to orchestrate isolated runs.
	- Network isolation is handled at the cluster/network policy level (not by `--network none`).
	- Resource/cgroup limits can be added via Pod resource requests/limits in a follow-up.

## 3. Phase 2: Bot Fleet
* **Goal:** Generate massive concurrent traffic.
* **Tech:** Go Goroutines.
* **Steps:** Spawn thousands of workers -> Send simulated buy/sell orders via Unix Sockets or TCP.

## 4. Phase 3: Telemetry & Validation
* **Goal:** Score the submissions.
* **Tech:** Redis Streams + Go Worker.
* **Steps:** Capture container output -> Verify orderbook math -> Calculate latency/TPS.

## 5. Phase 4: Live Leaderboard
* **Goal:** Display real-time results.
* **Tech:** React + WebSockets.
* **Steps:** Stream validation results via WebSockets -> Update UI tables and charts dynamically.

## 6. Milestones
1. **M1 (Upload):** Compile and run code in an isolated runtime — COMPLETED (Kubernetes-based sandbox).
2. **M2 (Flood):** Hit container with 10k req/sec using Go.
3. **M3 (Judge):** Verify trade math correctness.
4. **M4 (Stream):** Push live scores to React UI.

### Phase 1 follow-ups
- Add resource limits and network policies for stricter isolation.
- Add automatic cleanup of pods/services and logs collection into Redis/DB.
- Add tests that run multiple submissions concurrently to verify stability.