# Distributed Benchmarking Platform

A high-performance, distributed benchmarking system designed to stress-test and evaluate contestant-submitted trading infrastructure (Matching Engines/Orderbooks). 

The platform securely hosts contestant code in isolated environments and uses a highly concurrent Go-based architecture to simulate massive market volatility (peak traffic) to grade systems on latency, throughput, and correctness.

---

## 🏗️ Architecture (Phase 1: Complete)

We have built the **Security & Execution Core**. Our system is now capable of taking untrusted code and running it safely without risking our host machine.

### Core Technologies
* **Language:** Go (Golang)
* **API Framework:** Go Fiber
* **Containerization:** Docker (via Go Docker SDK)

### Data Flow Breakdown

**1. The Entry Point (API)**
* **Action:** Our Go server (`main.go`) listens for a `POST` request on `/submit`.
* **Result:** It extracts the `.cpp` file from the request and saves it to a local `workspace/` folder.

**2. The Sandbox Preparation**
* **Pathing:** The engine converts the relative file path to an Absolute Path so the Docker daemon can find it on our Windows/Linux drive.
* **Resource Budgeting:** It defines a "contract" for the code:
  * **Memory:** Max 512 MB.
  * **CPU:** Max 1.0 core.
  * **Network:** 0% (Completely disabled).

**3. The Execution (Docker)**
* **Compilation:** Inside a `gcc` container, our engine runs a shell command to compile the `.cpp` file into a binary named `run`.
* **Run:** It executes that binary.
* **Isolation:** Because of the sandbox settings, even if the contestant's code had a `while(true)` loop or tried to scan our network, it would be killed or blocked.

**4. The Feedback Loop**
* **Log Capture:** Our Go code waits for the container to finish.
* **Streaming:** It reaches into the container, grabs everything printed to the console (`stdout`), and pipes it back to our terminal.
* **Cleanup:** It deletes the container immediately after execution to save disk space and memory.

---

## 📂 Folder Structure

```text
trading-platform/
├── backend/
│   ├── cmd/
│   │   └── main.go              # API Gateway & Entry Point
│   ├── internal/
│   │   └── sandbox/
│   │       └── runner.go        # Docker SDK Integration & Security Policies
│   ├── workspace/               # Temporary isolated directory for uploads
│   ├── go.mod
│   └── go.sum
└── README.md