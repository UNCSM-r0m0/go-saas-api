# NATS Cross-Service Events Specification

## Purpose

Enable asynchronous communication between services via NATS for cache invalidation and limit refresh.

## Requirements

### Requirement: Usage Recorded Event

The system MUST publish a `usage.recorded` event when usage is recorded.

#### Scenario: Usage triggers cache invalidation

- GIVEN usage-service records new usage for a user
- WHEN the transaction completes
- THEN a `usage.recorded` event is published to NATS
- AND billing-service consumes it to invalidate cached usage totals

### Requirement: Subscription Changed Event

The system MUST publish a `subscription.changed` event when a subscription tier changes.

#### Scenario: Tier change refreshes limits

- GIVEN billing-service changes a user's subscription tier
- WHEN the change is persisted
- THEN a `subscription.changed` event is published to NATS
- AND usage-service consumes it to refresh rate limits within 30 seconds

### Requirement: Event Delivery

The system MUST use at-least-once delivery for NATS events.

#### Scenario: Consumer temporarily down

- GIVEN a NATS consumer is offline when an event is published
- WHEN the consumer comes back online
- THEN it receives the missed event
- AND processes it idempotently
