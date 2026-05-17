# Sandbox Gateway Routes Specification

## Purpose

Expose sandbox-service functionality through the API Gateway with proper auth and rate limiting.

## Requirements

### Requirement: Sandbox Preview Endpoint

The system MUST proxy sandbox preview requests to sandbox-service.

#### Scenario: Successful preview

- GIVEN an authenticated request to `POST /api/v1/sandbox/preview` with `{code, language}`
- WHEN the gateway routes it
- THEN it reaches `sandbox-service:3005`
- AND returns `{iframe_url, execution_id}`

#### Scenario: Unauthorized preview

- GIVEN an unauthenticated request to `/api/v1/sandbox/preview`
- WHEN the gateway processes it
- THEN it returns `401 Unauthorized`

### Requirement: Sandbox Execution Status

The system MUST expose sandbox execution status.

#### Scenario: Get execution status

- GIVEN an authenticated request to `GET /api/v1/sandbox/execution/{id}`
- WHEN the gateway routes it
- THEN it returns the execution status from sandbox-service
