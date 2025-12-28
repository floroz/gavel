# Debugging Guide: Hybrid Local/Cluster Development

This guide explains how to debug a specific service locally (e.g., using a debugger like Delve or VS Code) while keeping its dependencies (Postgres, RabbitMQ, other services) running in the Kubernetes cluster.

## The Strategy: Port Forwarding

Instead of running everything locally (Docker Compose) or everything remotely (K8s), we bridge the two.
1.  **Cluster**: Runs `Postgres`, `RabbitMQ`, `Redis`.
2.  **Local**: Runs `Bid Service` (in your IDE).
3.  **Bridge**: We `kubectl port-forward` the cluster resources to `localhost` so your local service can find them.

---

## 1. Start the Cluster Dependencies
First, ensure the cluster is running with all infrastructure. You can use Tilt for this, or just let Tilt run and then "stop" the service you want to debug locally so they don't conflict.
```bash
make dev
```
*   **Tip**: In the Tilt UI, you can uncheck the `bid-service` resource to stop it from running in the cluster, freeing up ports and preventing double-processing.

## 2. Port Forward Infrastructure
Open a terminal and run the following commands to expose cluster resources to your localhost.

### Postgres (Bids)
```bash
kubectl port-forward svc/postgres-bids-postgresql 5432:5432
```
*   Local Access: `postgres://user:password@localhost:5432/bid_db?sslmode=disable`

### Postgres (User Stats)
```bash
kubectl port-forward svc/postgres-stats-postgresql 5433:5432
```
*   Local Access: `postgres://user:password@localhost:5433/stats_db?sslmode=disable`
*   *Note: We map to port 5433 to avoid conflict with the Bids DB.*

### RabbitMQ
```bash
kubectl port-forward svc/rabbitmq 5672:5672
```
*   Local Access: `amqp://user:password@localhost:5672/`

### Redis
```bash
kubectl port-forward svc/redis-master 6379:6379
```
*   Local Access: `localhost:6379`

---

## 3. Run Service Locally with Env Vars
Now you can run your service. You must override the environment variables to point to `localhost` instead of the K8s DNS names.

### Bid Service (API)
```bash
BID_DB_URL="postgres://user:password@localhost:5432/bid_db?sslmode=disable" \
RABBITMQ_URL="amqp://user:password@localhost:5672/" \
REDIS_URL="localhost:6379" \
go run services/bid-service/cmd/api/main.go
```

### Bid Worker
```bash
BID_DB_URL="postgres://user:password@localhost:5432/bid_db?sslmode=disable" \
RABBITMQ_URL="amqp://user:password@localhost:5672/" \
go run services/bid-service/cmd/worker/main.go
```

---

## 4. VS Code Launch Configuration (`.vscode/launch.json`)
For a seamless F5 debugging experience, add this to your `.vscode/launch.json`.

```json
{
    "version": "0.2.0",
    "configurations": [
        {
            "name": "Debug Bid API",
            "type": "go",
            "request": "launch",
            "mode": "auto",
            "program": "${workspaceFolder}/services/bid-service/cmd/api/main.go",
            "env": {
                "BID_DB_URL": "postgres://user:password@localhost:5432/bid_db?sslmode=disable",
                "RABBITMQ_URL": "amqp://user:password@localhost:5672/",
                "REDIS_URL": "localhost:6379"
            }
        },
        {
            "name": "Debug Bid Worker",
            "type": "go",
            "request": "launch",
            "mode": "auto",
            "program": "${workspaceFolder}/services/bid-service/cmd/worker/main.go",
            "env": {
                "BID_DB_URL": "postgres://user:password@localhost:5432/bid_db?sslmode=disable",
                "RABBITMQ_URL": "amqp://user:password@localhost:5672/"
            }
        }
    ]
}
```

## Summary Checklist
1.  `make dev` (Keep it running).
2.  Stop `bid-service` in Tilt UI (optional, avoids log noise).
3.  `kubectl port-forward svc/postgres-bids-postgresql 5432:5432`
4.  `kubectl port-forward svc/rabbitmq 5672:5672`
5.  Run/Debug in VS Code.

