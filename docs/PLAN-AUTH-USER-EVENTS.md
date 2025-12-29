# Plan: User Created Event Propagation

## Goal
Enable `user-stats-service` (and other future services) to react when a new user is registered in `auth-service` using the Transactional Outbox Pattern.

## Phase 1: Event Definition & Infrastructure
- [ ] **Define Proto Event**: Add `UserCreated` message to `api/proto/events.proto`.
- [ ] **Generate Code**: Run `make proto` to update Go bindings.
- [ ] **Migration (Auth)**: Create `migrations/00002_create_outbox.sql` in `auth-service` to add `outbox_events` table.

## Phase 2: Auth Service (Producer)
- [ ] **Outbox Repository**:
    - Define `OutboxRepository` interface in `internal/domain/users/ports.go`.
    - Implement `PostgresOutboxRepository` in `internal/adapters/database/outbox_repository.go`.
- [ ] **Update Service Logic**:
    - Update `UserService` struct to include `OutboxRepository`.
    - Modify `Register` method in `internal/domain/users/service.go` to create a `UserCreated` event within the registration transaction.
- [ ] **Outbox Relay**:
    - Implement `OutboxRelay` in `internal/adapters/events/relay.go` (similar to bid-service).
    - It needs to poll `outbox_events`, publish to RabbitMQ, and update status.
    - Setup RabbitMQ publisher adapter.
- [ ] **Wire Up**:
    - Update `cmd/api/main.go` to initialize the relay and start it in a goroutine.

## Phase 3: User Stats Service (Consumer)
- [ ] **User Consumer**:
    - Create `UserConsumer` in `internal/adapters/events/user_consumer.go`.
    - It should bind to `auction.events` exchange with routing key `user.created`.
- [ ] **Service Logic**:
    - Add `ProcessUserCreated(ctx context.Context, event UserCreatedEvent) error` to `UserStatsService`.
    - Handle idempotency (if user stats already exist, do nothing or update).
- [ ] **Wire Up**:
    - Update `cmd/worker/main.go` (or `api/main.go` if it runs consumers) to start `UserConsumer`.

## Phase 4: Verification & Testing
- [ ] **Unit Tests**:
    - Test `Register` logic ensures event creation.
- [ ] **Integration Test (Auth)**:
    - Verify `Register` writes to both `users` and `outbox_events` tables atomically.
- [ ] **End-to-End Test**:
    - Register a user via API.
    - Verify `user-stats-service` has created a record for that user.

