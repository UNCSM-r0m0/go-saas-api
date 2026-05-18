# Proposal: Integrate r3-chat Backend

## Intent

Close the integration gaps between the go-saas-api backend and the r3-chat React frontend. Fix port inconsistencies, fragile WebSocket auth, invisible sandbox/document services, broken anonymous rate limiting, and underutilized NATS so the full stack works end-to-end in local dev and Docker.

## Scope

### In Scope
- Unify gateway port to `3000` across Docker Compose and `start-local.ps1`
- Harden WebSocket auth upgrade (cookie + header fallback, proper `X-User-ID` injection)
- Expose `sandbox-service` via API Gateway (`/api/v1/sandbox/*`) with auth + rate limits
- Expose `document-service` via API Gateway (`/api/v1/documents/*`, `/api/v1/files/upload`) for frontend uploads
- Fix anonymous rate limiting to use `anonymousId` sent by r3-chat instead of raw client IP
- Add missing endpoints r3-chat consumes (`/chat/usage/stats`)
- Wire NATS for cross-service events (usage recorded → billing cache invalidated, subscription changed → usage limits refreshed)

### Out of Scope
- Changes to r3-chat UI/UX (except port config if needed)
- New microservices or complete architecture redesign
- Production deployment pipeline changes

## Capabilities

### New Capabilities
- `sandbox-gateway-routes`: Proxy sandbox endpoints through API Gateway with JWT/API key auth
- `document-gateway-routes`: Proxy document upload/download through API Gateway
- `anonymous-rate-limiting`: Rate limit by `anonymousId` body field for unauthenticated users
- `nats-cross-service-events`: Publish/subscribe usage and billing events via NATS

### Modified Capabilities
- `api-gateway`: Add routes for sandbox, document service; harden WS auth context injection
- `chat-streaming`: Accept and propagate `anonymousId` from request body for anonymous sessions

## Approach

1. **Infra alignment**: Change `docker-compose.yml` gateway port from `3001` to `3000`, update service URL env vars to match `start-local.ps1` ports.
2. **Gateway routing**: Add proxy groups in `cmd/api-gateway/main.go` for `/sandbox/*` → sandbox-service:3005 and `/documents/*`, `/files/upload` → document-service:8000.
3. **WS auth hardening**: Ensure `proxyTo` injects `Cookie` headers correctly on upgrade; validate JWT from cookie before WS handshake; pass `X-User-ID` / `X-Anonymous-ID` downstream.
4. **Anonymous rate limiting**: Update `middleware/ratelimit.go` to check request body for `anonymousId` when no JWT is present; fall back to IP only if absent.
5. **Missing endpoints**: Ensure `usage-service` exposes `/chat/usage/stats` and gateway routes it.
6. **NATS events**: Add lightweight publishers in `usage-service` (usage recorded) and `billing-service` (subscription changed); add consumers to refresh Redis-tier cache.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `docker-compose.yml` | Modified | Gateway port 3001→3000; add sandbox/doc URLs to gateway env |
| `start-local.ps1` | Modified | Keep port 3000; verify infra ports align |
| `cmd/api-gateway/main.go` | Modified | New proxy routes; WS auth context injection |
| `internal/platform/middleware/ratelimit.go` | Modified | Use `anonymousId` from body for anonymous limits |
| `internal/platform/middleware/jwt.go` | Modified | Ensure cookie auth works on WS upgrade |
| `cmd/usage-service/main.go` | Modified | Add `/chat/usage/stats` endpoint; NATS publisher |
| `cmd/billing-service/main.go` | Modified | NATS publisher for subscription changes |
| `cmd/api-gateway/main.go` | New | NATS consumer for cache invalidation |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Port conflict on 3000 | Low | Verify no other service uses 3000 in dev; document in README |
| WS auth still fails with cookies | Med | Add integration test for WS upgrade with cookie header |
| Document service upload size limits | Med | Gate `MaxUploadSize` config through gateway; return 413 early |
| NATS event ordering issues | Low | Use JetStream with at-least-once delivery; idempotent consumers |

## Rollback Plan

- Revert `docker-compose.yml` port to `3001`
- Remove new gateway route groups (sandbox, documents)
- Restore `ratelimit.go` to IP-based anonymous limiting
- Disable NATS publishers (feature-flag via env var `NATS_EVENTS_ENABLED`)

## Dependencies

- r3-chat frontend sends `anonymousId` in chat request bodies (already implemented)
- NATS server running (already in docker-compose)

## Success Criteria

- [ ] `docker-compose up` and `start-local.ps1` both use gateway port 3000 without conflicts
- [ ] r3-chat WebSocket connection to `/api/v1/agent/ws` succeeds with cookie auth
- [ ] Frontend can POST `/api/v1/sandbox/preview` and receive sandboxed iframe URL
- [ ] Frontend can POST `/api/v1/files/upload` and GET `/api/v1/documents/{id}`
- [ ] Anonymous users with same `anonymousId` share the same rate limit bucket (not IP)
- [ ] `/api/v1/chat/usage/stats` returns usage stats for the authenticated user
- [ ] Changing subscription tier invalidates cached tier within 30 seconds via NATS event
