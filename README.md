# 🔨 Gavel: High-Performance Distributed Auction System

[![Go Version](https://img.shields.io/badge/Go-1.25+-00ADD8?style=flat-square&logo=go)](https://go.dev/)
[![React](https://img.shields.io/badge/React-19-61DAFB?style=flat-square&logo=react)](https://react.dev/)
[![Next.js](https://img.shields.io/badge/Next.js-15-000000?style=flat-square&logo=next.js)](https://nextjs.org/)
[![Kubernetes](https://img.shields.io/badge/Kubernetes-Latest-326CE5?style=flat-square&logo=kubernetes)](https://kubernetes.io/)
[![Tilt](https://img.shields.io/badge/Tilt-Dev_Env-23C6C8?style=flat-square&logo=tilt)](https://tilt.dev/)
[![Postgres](https://img.shields.io/badge/Postgres-16-336791?style=flat-square&logo=postgresql)](https://www.postgresql.org/)
[![RabbitMQ](https://img.shields.io/badge/RabbitMQ-Latest-FF6600?style=flat-square&logo=rabbitmq)](https://www.rabbitmq.com/)
[![Redis](https://img.shields.io/badge/Redis-Latest-DC382D?style=flat-square&logo=redis)](https://redis.io/)

A distributed auction platform designed for data consistency and architectural resilience. Built with Go, Postgres, and RabbitMQ, Gavel leverages ConnectRPC and Protobuf for high-performance, type-safe API communication, alongside a robust event-driven architecture (Transactional Outbox) to decouple services and ensure reliable state management across the system.

---

## 🚀 Key Capabilities

Gavel is designed to demonstrate robust distributed system patterns:

*   **Reliable Event Delivery**: Implements the **Transactional Outbox Pattern** to ensure consistency between database state and message publishing.
*   **Data Integrity**: Utilizes Postgres row-level locking (`SELECT FOR UPDATE`) to manage concurrency and prevent race conditions.
*   **Independent Scalability**: Decoupled microservices design allowing the Bid Engine and Analytics components to scale independently.
*   **Idempotency**: "At-least-once" delivery with consumer-side deduplication to ensure correct data processing despite retries.
*   **Observability**: Centralized structured logging (`log/slog`) across all services.
*   **Cloud Native**: Fully containerized and orchestrated via Kubernetes (Kind) with NGINX Ingress and Helm Charts.

---

## 🏗 Architecture

The system leverages a decoupled **Ports & Adapters (Hexagonal)** architecture, ensuring business logic remains isolated from infrastructure concerns.

```mermaid
graph TD
    subgraph ClientSide ["Client Side"]
        Client[User / React Client]
    end

    Client -->|1. Server Actions / RSC| Ingress{NGINX Ingress}

    subgraph K8s ["Kubernetes Cluster"]
        %% Ingress Handling
        subgraph BFF_Domain ["Backend For Frontend Domain"]
            BFF[Next.js App Router<br/>SSR + Server Actions]
        end

        Ingress -->|Proxy| BFF

        %% Shared Infrastructure
        RMQ(RabbitMQ)
        Redis(Redis Cache)

        subgraph Identity ["Identity Domain"]
            AuthAPI[Auth Service] -->|Tx: Save User + Outbox Event| AuthDB[(Postgres: auth_db)]
            AuthAPI -->|Poll Outbox - Background| AuthDB
        end

        subgraph Bids ["Bid Domain"]
            BidAPI[Bid Service API] -->|Tx: Save Bid + Outbox Event| BidDB[(Postgres: bid_db)]
            BidWorker[Bid Outbox Worker] -->|Poll Outbox Table| BidDB
        end
        
        subgraph Analytics ["Analytics Domain"]
            StatsAPI[User Stats API] -->|Read| StatsDB[(Postgres: stats_db)]
            StatsWorker[User Stats Consumer] -->|Update User Totals| StatsDB
        end

        %% Service Communication (gRPC)
        BFF -->|ConnectRPC| AuthAPI
        BFF -->|ConnectRPC| BidAPI
        BFF -->|ConnectRPC| StatsAPI

        %% Event Flow & Caching
        AuthAPI -- Publish UserCreated --> RMQ
        BidWorker -- Publish BidPlaced --> RMQ
        
        RMQ -- Consume: BidPlaced --> StatsWorker
        RMQ -- Consume: UserCreated --> StatsWorker
        
        BidAPI -.->|Cache| Redis
    end
```

---

## 🔐 Authentication Strategy (BFF)

We implement the **Backend for Frontend (BFF)** pattern to secure user sessions and support Server-Side Rendering (SSR).

*   **Zero Trust**: Backend microservices (`auth-service`, `bid-service`, etc.) are **private** and not exposed to the public internet. They accept requests only from the BFF.
*   **HttpOnly Cookies**: Access and Refresh tokens are stored in secure, HttpOnly cookies. The browser never sees the raw tokens, preventing XSS attacks.
*   **Server Actions**: The frontend calls Next.js Server Actions which act as a secure proxy. The Node.js server attaches the tokens to the downstream ConnectRPC requests.
*   **React Server Components**: Protected pages fetch data directly on the server, reading cookies and making authenticated backend calls before rendering HTML.
*   **Transparent Refresh**: If a token is expired, the BFF automatically refreshes it using the refresh cookie and retries the request, without the client needing to handle complex logic.

For detailed architecture documentation, see [docs/PLAN-AUTH-BFF.md](docs/PLAN-AUTH-BFF.md).

---

## 🛠 Tech Stack & Patterns

-   **Frontend**: [Next.js 15 (App Router), React 19, Server Components, Server Actions, Tailwind, Shadcn/ui](frontend/README.md)
-   **Language**: Go 1.25+ (Generics, Context-driven)
-   **Orchestration**: Kubernetes (Kind), Helm, Tilt, ctlptl
-   **Database**: PostgreSQL (Raw `pgx` for maximum control over transactions)
-   **Messaging**: RabbitMQ (Topic-based exchanges for decoupled scaling)
-   **Caching**: Redis (Bidding leaderboards and item metadata)
-   **Protocol**: Protobuf + ConnectRPC for high-efficiency, type-safe communication
-   **Pattern**: Hexagonal Architecture (Clean Architecture)

---

## 🔌 API & Communication

We use **ConnectRPC** for service-to-service communication, adapted for the BFF pattern:

1.  **Browser → BFF**: Next.js **Server Actions** handle mutations (login, place bid, etc.) with automatic cookie management. **React Server Components** fetch data for page rendering.
2.  **BFF → Services**: The Node.js server uses **ConnectRPC** clients (Protocol Buffers) to communicate with the internal, private microservices.
3.  **Testing**: Backend services still support standard JSON over HTTP, making them easy to test with curl/Postman (via `localhost` access or port-forwarding).

### Communication Flow

```mermaid
graph LR
    Browser[Browser]
    BFF["Next.js BFF<br/>- RSC for reads<br/>- Actions for mutations"]
    Services["Go Services<br/>(private)"]

    Browser -->|"Server Actions<br/>(mutations)"| BFF
    BFF -->|"HTML/JSON"| Browser
    
    BFF -->|ConnectRPC| Services
    Services -->|Response| BFF
```

### Testing Endpoints (JSON)

**Prerequisite**: Add `127.0.0.1 api.gavel.local app.gavel.local` to your `/etc/hosts` file.

**Place Bid** (Write)
```bash
curl -X POST -H "Content-Type: application/json" \
  -d '{"item_id": "uuid", "user_id": "uuid", "amount": 15000}' \
  http://api.gavel.local/bids.v1.BidService/PlaceBid
```

**Get User Stats** (Read)
```bash
curl -X POST -H "Content-Type: application/json" \
  -d '{"user_id": "uuid"}' \
  http://api.gavel.local/userstats.v1.UserStatsService/GetUserStats
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
*   Node.js & PNPM (via fnm: `brew install fnm`)
    ```bash
    cd ./<project-folder> # cd to the project root folder
    fnm use # this should automatically pick the correct node version from .nvmrc
    npm install -g pnpm@10.26
    ```

### 1. Setup Local Cluster
We use `ctlptl` for declarative cluster management. This creates a Kind cluster and a local container registry.

```bash
make cluster
```

**Important**: Ensure you have `127.0.0.1 api.gavel.local app.gavel.local` in your `/etc/hosts`.

### 2. Start Backend Environment
Run the backend stack (Infrastructure + Services) with Tilt. This will build images, apply Helm charts, and stream logs:
```bash
make dev
```
*   Press `Space` to open the Tilt UI
*   Services are accessible at `http://api.gavel.local`

### 3. Access Frontend
The frontend application is deployed to the cluster and accessible via Ingress.
*   Open `http://app.gavel.local`

*(Optional)* For UI-only development with faster hot-reload, you can run `pnpm dev` locally in the `frontend/` directory (listening on localhost:3000), but `app.gavel.local` serves the full SSR experience from the cluster.

#### Frontend Setup
To run the frontend locally or execute tests, you must create the environment configuration files:
```bash
cd frontend
cp .env.example .env.local
cp .env.example .env.test
```

### 4. Verify Deployment
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
| `pnpm --dir frontend proto:gen` | Generate TypeScript clients from Protobuf |
| `make lint` | Run linters |
| `make test` | Run full test suite |


## 🧪 Testing Strategy

Gavel prioritizes **real-world reliability** over theoretical unit coverage. Our testing philosophy is built on three pillars:

*   **Vertical Slice Integration**: We test the entire request lifecycle (Handler -> Service -> Repository -> DB) using real infrastructure. This validates the contracts between layers and ensures that SQL queries, transaction boundaries, and error handling work together correctly.
*   **Testcontainers**: We use [Testcontainers-go](https://golang.testcontainers.org/) to spin up ephemeral, production-identical instances of dependencies (Postgres, RabbitMQ) for every test suite.
*   **No Mocks Policy**: We avoid mocking infrastructure layers. If we need to test business logic that doesn't depend on IO, we extract it into pure functions and unit test them separately.

### Running Tests

Integration tests are separated using the `integration` build tag to keep the development loop fast.

| Command | Action |
|:---|:---|
| `make test-unit` | Run unit tests (no external dependencies) |
| `make test-integration` | Run integration tests (requires Docker) |
| `make test` | Run the full test suite |

---
