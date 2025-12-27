# 🔨 Gavel: High-Performance Real-Time Auction System

![Auction Header](https://images.unsplash.com/photo-1584281723320-cd3c896317d3?auto=format&fit=crop&q=80&w=2000&h=600)

[![Go Version](https://img.shields.io/badge/Go-1.24+-00ADD8?style=flat-square&logo=go)](https://go.dev/)
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

---

## 🏗 Architecture

The system leverages a decoupled **Ports & Adapters (Hexagonal)** architecture, ensuring business logic remains isolated from infrastructure concerns.

```mermaid
graph TD
    subgraph "Bid Domain (Write Side)"
        API[Bid Service API]
        Worker[Bid Outbox Worker]
        BidDB[(Postgres: bid_db)]
    end

    subgraph "Analytics Domain (Read Side)"
        StatsWorker[User Stats Consumer]
        StatsDB[(Postgres: stats_db)]
    end

    subgraph "Infrastructure"
        RMQ(RabbitMQ)
        Redis(Redis Cache)
    end

    %% Flows
    User((User)) -->|Place Bid| API
    API -->|Tx: Save Bid + Outbox Event| BidDB
    
    Worker -->|Poll Outbox Table| BidDB
    Worker -->|Publish Event| RMQ
    
    RMQ -->|Consume: BidPlaced| StatsWorker
    StatsWorker -->|Update User Totals| StatsDB
```

---

## 🛠 Tech Stack & Patterns

-   **Language**: Go 1.24+ (Generics, Context-driven)
-   **Database**: PostgreSQL (Raw `pgx` for maximum control over transactions)
-   **Messaging**: RabbitMQ (Topic-based exchanges for decoupled scaling)
-   **Caching**: Redis (Bidding leaderboards and item metadata)
-   **Protocol**: Protobuf for high-efficiency message serialization
-   **Pattern**: Hexagonal Architecture (Clean Architecture)

---

## ⚡ Quick Start

### 1. Initialize Infrastructure
Spin up the core services (Postgres, RabbitMQ, and Redis):
```bash
make up
```

### 2. Apply Migrations
Prepare the schemas for both the Bid and Statistics databases:
```bash
make migrate-up-all
```

### 3. Launch Services
Run the API and background workers:
```bash
make run-all
```

---

## ⚙️ Development Toolkit

| Command | Action |
|:---|:---|
| `make up / down` | Control local infrastructure |
| `make test` | Run full test suite (Unit + Integration) |
| `make build-all` | Compile production binaries / Docker images |
| `make proto-gen` | Rebuild Protobuf definitions |

---

> **Note**: This system is architected for deployment in Kubernetes environments. Check the `docs/PLAN.md` for the upcoming roadmap.
