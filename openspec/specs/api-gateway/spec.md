# API Gateway Specification

## Purpose
Reverse proxy, auth validation, rate limiting, and routing to backend services.

## Requirements

### Requirement: Service Routing

The system MUST route requests to appropriate services based on path prefix.

#### Scenario: Route to agent-service

- GIVEN a request to `/api/v1/agent/*`
- WHEN it arrives at the gateway
- THEN it is proxied to `agent-service:3002`
- AND the path is forwarded unchanged

#### Scenario: Route to auth-service

- GIVEN a request to `/api/v1/auth/*`
- WHEN it arrives at the gateway
- THEN it is proxied to `auth-service:3003`

#### Scenario: Artifact preview routing

- GIVEN a request to `/artifacts/{id}/preview`
- WHEN it arrives at the gateway
- THEN it is proxied to `agent-service:3002/artifacts/{id}/preview`

### Requirement: JWT Validation

The system MUST validate JWT tokens at the gateway.

#### Scenario: Valid token

- GIVEN a request with valid `Authorization: Bearer <token>`
- WHEN the gateway processes it
- THEN the token is validated
- AND the user ID is added as `X-User-ID` header
- AND the request is forwarded

#### Scenario: Invalid token

- GIVEN a request with expired JWT
- WHEN the gateway processes it
- THEN it returns `401 Unauthorized`
- AND the request is NOT forwarded

### Requirement: Rate Limiting

The system MUST enforce rate limits per tier.

#### Scenario: Free tier limit

- GIVEN an anonymous user who sent 3 messages today
- WHEN they send a 4th message
- THEN the gateway returns `429 Too Many Requests`
- AND `X-RateLimit-Reset` header is present
