# Serial Generation Async Notes

## Goal

Move physical product serial creation out of the `SubmitShippingForm` request path while keeping:

- durable task persistence
- restart recovery
- idempotent generation
- order-level status visibility in admin UI

## Current Design

- Request path:
  - `OrderService.SubmitShippingForm(...)`
  - if `SerialGenerationService` is wired:
    - save order normally
    - mark `orders.serial_generation_status = queued`
    - persist `serial_generation_tasks` row in the same transaction
    - wake background worker after commit
  - if not wired:
    - fallback to the previous synchronous serial generation path

- Background worker:
  - `SerialGenerationService`
  - persistent queue table: `serial_generation_tasks`
  - startup recovery: `processing -> queued`
  - claim runnable tasks, set order state to `processing`
  - call `SerialService.createMissingOrderSerialsTx(...)`
  - on success:
    - mark order `completed` or `not_required`
    - mark task `completed`
    - emit `serial.create.after`
  - on failure:
    - mark order `failed`
    - retry with bounded backoff
    - after max retries, task stays `failed`

## Idempotency

`ProductSerial` still does not store order-item index. To avoid duplicate generation on retry, the worker:

- derives desired quantity per `product_id` from order items
- counts existing serials grouped by `order_id + product_id`
- only creates the missing delta

This keeps retries safe without changing the current serial data shape.

## Added Persistence

- `orders.serial_generation_status`
- `orders.serial_generation_error`
- `orders.serial_generated_at`
- `serial_generation_tasks`

Migration also backfills legacy orders that already have serials to `serial_generation_status = completed`.

## Key Files

- `backend/internal/service/order_service.go`
- `backend/internal/service/serial_service.go`
- `backend/internal/service/serial_generation_service.go`
- `backend/internal/models/order.go`
- `backend/internal/models/serial_generation_task.go`
- `backend/internal/database/database.go`
- `frontend/components/orders/order-detail.tsx`

## Current Admin UX

Admin order detail now shows serial generation state when:

- queued
- processing
- failed
- cancelled

If serials already exist, the normal serial list still renders.

## Remaining Nice-to-Haves

- manual retry action in admin order detail
- dedicated admin task list / observability
- optional worker concurrency tuning
- request-time warning surface when a task is terminally failed
