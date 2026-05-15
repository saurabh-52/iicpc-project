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
1. **M1 (Upload):** Compile and run code in Docker.
2. **M2 (Flood):** Hit container with 10k req/sec using Go.
3. **M3 (Judge):** Verify trade math correctness.
4. **M4 (Stream):** Push live scores to React UI.