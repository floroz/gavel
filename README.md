# 🔨 Gavel: High-Performance Real-Time Auction System

[![Go Version](https://img.shields.io/badge/Go-1.24+-00ADD8?style=flat-square&logo=go)](https://go.dev/)
[![Kubernetes](https://img.shields.io/badge/Kubernetes-Latest-326CE5?style=flat-square&logo=kubernetes)](https://kubernetes.io/)
[![Tilt](https://img.shields.io/badge/Tilt-Dev_Env-23C6C8?style=flat-square&logo=tilt)](https://tilt.dev/)
[![Postgres](https://img.shields.io/badge/Postgres-16-336791?style=flat-square&logo=postgresql)](https://www.postgresql.org/)
[![RabbitMQ](https://img.shields.io/badge/RabbitMQ-Latest-FF6600?style=flat-square&logo=rabbitmq)](https://www.rabbitmq.com/)
[![Redis](https://img.shields.io/badge/Redis-Latest-DC382D?style=flat-square&logo=redis)](https://redis.io/)

A distributed, production-ready auction platform engineered for high-concurrency bidding and data consistency. Built with Go, Postgres, and RabbitMQ, Gavel implements a robust event-driven architecture designed to handle thousands of bids per second with sub-millisecond precision.

---

## 🚀 Key Capabilities

Gavel is built to solve the complex challenges of modern auction systems:

*   **Zero-Loss Event Delivery**: Implements the **Transactional Outbox Pattern** to ensure absolute consistency between database state and message delivery.
*   **High-Concurrency Locking**: Utilizes advanced Postgres row-level locking (`SELECT FOR UPDATE`) to prevent race conditions during "sniping" scenarios.
*   **Massive Scalability**: Microservices-first design allows independent scaling of the Bid Engine and Analytics components.
*   **Strict Idempotency**: Guaranteed "at-least-once" delivery with deduplication at the consumer level, ensuring data integrity across the entire cluster.
*   **Full Observability**: Structured logging and transaction tracing across service boundaries.
*   **Cloud Native**: Fully containerized and orchestrated via Kubernetes (Kind) with NGINX Ingress and Helm Charts.

---

## 🏗 Architecture

The system leverages a decoupled **Ports & Adapters (Hexagonal)** architecture, ensuring business logic remains isolated from infrastructure concerns.

```mermaid
graph TD
    User((User)) -->|JSON/ConnectRPC| Ingress{NGINX Ingress}
    
    subgraph "Kubernetes Cluster"
        Ingress -->|api.auction.local/bids.v1...| BidAPI[Bid Service API]
        Ingress -->|api.auction.local/userstats.v1...| StatsAPI[User Stats API]
        
        subgraph "Bid Domain (Write Side)"
            BidAPI -->|Tx: Save Bid + Outbox Event| BidDB[(Postgres: bid_db)]
            Worker[Bid Outbox Worker]
            Worker -->|Poll Outbox Table| BidDB
        end
        
        subgraph "Analytics Domain (Read Side)"
            StatsAPI -->|Read| StatsDB[(Postgres: stats_db)]
            StatsWorker[User Stats Consumer]
            StatsWorker -->|Update User Totals| StatsDB
        end
        
        subgraph "Infrastructure"
            Worker -->|Publish Event| RMQ(RabbitMQ)
            RMQ -->|Consume: BidPlaced| StatsWorker
            BidAPI -->|Cache| Redis(Redis Cache)
        end
    end
```

---

## 🛠 Tech Stack & Patterns

-   **Language**: Go 1.24+ (Generics, Context-driven)
-   **Orchestration**: Kubernetes (Kind), Helm, Tilt, ctlptl
-   **Database**: PostgreSQL (Raw `pgx` for maximum control over transactions)
-   **Messaging**: RabbitMQ (Topic-based exchanges for decoupled scaling)
-   **Caching**: Redis (Bidding leaderboards and item metadata)
-   **Protocol**: Protobuf for high-efficiency message serialization
-   **Pattern**: Hexagonal Architecture (Clean Architecture)

---

## 🔌 API & Communication

We use **ConnectRPC** for the service-to-frontend API. This provides a "best of both worlds" approach:

1.  **Frontend**: Auto-generated, type-safe TypeScript clients (Protocol Buffers).
2.  **Testing**: Standard JSON over HTTP (curl / Postman) without needing special tools.

### Testing Endpoints (JSON)

**Prerequisite**: Add `127.0.0.1 api.auction.local` to your `/etc/hosts` file.

**Place Bid** (Write)
```bash
curl -X POST -H "Content-Type: application/json" \
  -d '{"item_id": "uuid", "user_id": "uuid", "amount": 15000}' \
  http://api.auction.local/bids.v1.BidService/PlaceBid
```

**Get User Stats** (Read)
```bash
curl -X POST -H "Content-Type: application/json" \
  -d '{"user_id": "uuid"}' \
  http://api.auction.local/userstats.v1.UserStatsService/GetUserStats
```

---

## ⚡ Quick Start (Kubernetes w/ Tilt)

### Prerequisites
*   Docker
*   Kind (`brew install kind`)
*   Tilt (`brew install tilt-dev/tap/tilt`)
*   Helm (`brew install helm`)
*   ctlptl (`brew install ctlptl`)
*   kubectl

### 1. Setup Local Cluster
We use `ctlptl` for declarative cluster management. This creates a Kind cluster and a local container registry.

```bash
make cluster
```

**Important**: Ensure you have `127.0.0.1 api.auction.local` in your `/etc/hosts`.

### 2. Start Development Environment
Run the entire stack (Infrastructure + Services) with Tilt. This will build images, apply Helm charts, and stream logs:
```bash
make dev
```
*   Press `Space` to open the Tilt UI
*   Services are accessible at `http://api.auction.local`

### 3. Verify Deployment
Run the verification script to check service health:
```bash
./scripts/verify-deployment.sh
```

---

## ⚙️ Development Toolkit

| Command | Action |
|:---|:---|
| `make dev` | **Recommended**: Start full k8s dev environment (Tilt) |
| `make clean` | Tear down k8s resources (Tilt) |
| `make cluster` | Create/Update Kind cluster + Registry |
| `make cluster-delete` | Destroy Cluster + Registry |
| `make proto-gen` | Rebuild Protobuf definitions (Go) |
| `make proto-gen-ts` | Generate TypeScript clients |
| `make lint` | Run linters |
| `make test` | Run full test suite |


## 🧪 Testing Strategy

Gavel prioritizes **real-world reliability** over theoretical unit coverage. Our testing philosophy is built on three pillars:

*   **Integration-First**: We prefer testing components against real infrastructure (Postgres, RabbitMQ) rather than using mocks. This ensures that our SQL queries, transaction boundaries, and message delivery logic are actually correct.
*   **Testcontainers**: We use [Testcontainers-go](https://golang.testcontainers.org/) to spin up ephemeral, production-identical instances of our dependencies for every test suite.
*   **No Mocks Policy**: We discourage the use of mock frameworks (like `gomock` or `testify/mock`) for infrastructure layers. If you need to test a repository, test it against a real database.

### Running Tests

Integration tests are separated using the `integration` build tag to keep the development loop fast.

| Command | Action |
|:---|:---|
| `make test-unit` | Run unit tests (no external dependencies) |
| `make test-integration` | Run integration tests (requires Docker) |
| `make test` | Run the full test suite |

---
