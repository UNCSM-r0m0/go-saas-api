# Delta for API Gateway

## ADDED Requirements

### Requirement: Sandbox Service Routing

The system MUST route `/api/v1/sandbox/*` requests to sandbox-service.

#### Scenario: Authenticated sandbox request

- GIVEN a request to `POST /api/v1/sandbox/preview` with valid JWT
- WHEN it arrives at the gateway
- THEN it is proxied to `sandbox-service:3005`
- AND `X-User-ID` header is injected

#### Scenario: Unauthenticated sandbox request

- GIVEN a request to `/api/v1/sandbox/*` without auth
- WHEN it arrives at the gateway
- THEN it returns `401 Unauthorized`

### Requirement: Document Service Routing

The system MUST route document upload and download requests to document-service.

#### Scenario: File upload routing

- GIVEN a request to `POST /api/v1/files/upload` with valid auth
- WHEN it arrives at the gateway
- THEN it is proxied to `document-service:8000`

#### Scenario: Document retrieval routing

- GIVEN a request to `GET /api/v1/documents/{id}` with valid auth
- WHEN it arrives at the gateway
- THEN it is proxied to `document-service:8000`

#### Scenario: Upload size exceeded

- GIVEN a file upload larger than `MaxUploadSize`
- WHEN it arrives at the gateway
- THEN it returns `413 Payload Too Large`
- AND the request is NOT forwarded

## MODIFIED Requirements

### Requirement: JWT Validation

The system MUST validate JWT tokens at the gateway, including WebSocket upgrade requests.

(Previously: Only standard HTTP requests were validated; WS auth was fragile.)

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

#### Scenario: WebSocket upgrade with cookie

- GIVEN a WebSocket upgrade request with valid JWT in Cookie header
- WHEN the gateway processes it
- THEN the cookie is parsed and JWT validated
- AND `X-User-ID` is injected into the upgrade request

#### Scenario: WebSocket upgrade with header fallback

- GIVEN a WebSocket upgrade request without Cookie but with `Authorization: Bearer <token>`
- WHEN the gateway processes it
- THEN the header token is validated
- AND `X-User-ID` is injected into the upgrade request

### Requirement: Rate Limiting

The system MUST enforce rate limits per tier, using `anonymousId` for anonymous users when available.

(Previously: Anonymous rate limiting used raw client IP exclusively.)

#### Scenario: Free tier limit

- GIVEN an anonymous user who sent 3 messages today
- WHEN they send a 4th message
- THEN the gateway returns `429 Too Many Requests`
- AND `X-RateLimit-Reset` header is present

#### Scenario: Anonymous with anonymousId

- GIVEN an anonymous request with `anonymousId: "abc123"` in body
- WHEN rate limit is checked
- THEN the `anonymousId` value is used as the rate limit key

#### Scenario: Anonymous without anonymousId fallback

- GIVEN an anonymous request without `anonymousId` field
- WHEN rate limit is checked
- THEN the client IP is used as the rate limit key
