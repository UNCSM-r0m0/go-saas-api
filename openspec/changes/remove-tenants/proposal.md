# Proposal: Remove Multi-Tenancy + Schema Cleanup

## Intent

Eliminate all multi-tenancy infrastructure from this personal AI chat backend. The project operates as a single-tenant system (hardcoded tenant UUID everywhere), so tenant isolation adds complexity with zero benefit.

## Scope

### In Scope
- Drop `tenants` table and `tenant_id` columns from all tables
- Remove all RLS policies and `set_tenant`/`current_tenant` functions
- Remove `TenantID` from JWT claims and invalidate all sessions
- Remove `X-Tenant-ID` header injection from API Gateway, middleware, and handlers
- Strip `tenant_id` parameters from all repository/service methods
- Remove `setTenant()` calls from 6 repositories
- Update file storage paths: `basePath/<tenantID>/<fileID>` → `basePath/<fileID>`
- Apply bundled schema improvements (message_files junction table, remove embedding, rename column, PK fix, user columns, drop tool_executions, encryption_key_version)
- Update Swagger docs and regenerate

### Out of Scope
- Auto-conversation-titling feature
- Any new capabilities or frontend redesign
- Data migration (single tenant — no data to preserve)

## Capabilities

### New Capabilities
None.

### Modified Capabilities
All existing capabilities require delta specs due to tenant removal and schema changes:
- `api-gateway`: Remove `X-Tenant-ID` injection, update JWT validation, rate-limit cache keys
- `agent-runtime`: Remove tenant from session/orchestrator/websocket models
- `agent-tools`: Update file storage paths
- `artifact-storage`: Remove tenant from artifact records
- `chat-streaming`: Remove tenant from conversation/message models
- `agent-memory`: Remove `embedding` column; schema-only change
- `agent-skills`: Remove tenant from skill records
- `agent-subagent`: Remove tenant from sub-agent context
- `sandbox-service`: No functional changes (no tenant references)

## Approach

1. **Single migration**: `migrations/011_remove_tenants_and_schema_cleanup.sql` drops RLS first, then columns/table, then applies schema improvements.
2. **Models**: Strip `TenantID` fields from all domain structs.
3. **Repositories**: Remove `tenantID` parameters and `setTenant()` calls.
4. **Services/Handlers**: Stop reading `X-Tenant-ID`.
5. **Middleware/JWT/Gateway**: Remove tenant from claims, context, CORS, and proxy headers.
6. **File storage**: Simplify path construction.
7. **Tests**: Update fixtures to match new schemas.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `migrations/` | Modified | New migration + 7 historical files with tenant refs |
| `internal/*/model*.go` | Modified | Remove `TenantID` fields |
| `internal/*/repository*.go` | Modified | Remove tenant params and `setTenant()` |
| `internal/*/service*.go` | Modified | Remove tenant params |
| `internal/*/*handler.go` | Modified | Stop parsing `X-Tenant-ID` |
| `internal/platform/middleware/` | Modified | Remove tenant from JWT/API key context |
| `pkg/jwt/` | Modified | Remove `TenantID` from Claims |
| `cmd/api-gateway/` | Modified | Stop injecting `X-Tenant-ID`; update cache keys |
| `internal/agent/websocket/` | Modified | Remove tenant from client keys |
| `internal/fileupload/` | Modified | New storage path scheme |
| `docs/` | Modified | Regenerate Swagger |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| All JWT sessions invalidated | Certain | Document re-login requirement; no code workaround |
| Existing file uploads unreachable | High | New files use new paths; old paths orphaned (acceptable for personal project) |
| Missed tenant references | Med | Global search `tenant`/`Tenant` after changes; compile check |
| RLS left enabled = empty queries | High | Migration drops RLS BEFORE removing columns |

## Rollback Plan

Restore from database backup taken before migration. Revert Go changes via `git checkout`. File storage path change is one-way; old uploads remain at old paths unless manually migrated.

## Dependencies

- **Before**: Database backup mandatory.
- **After**: Users must re-login to obtain new JWT tokens.

## Success Criteria

- [ ] All queries execute without `setTenant()` calls
- [ ] Zero `tenant_id` references remain in Go code
- [ ] `tenants` table does not exist in database
- [ ] No RLS policies or `current_tenant` function remain
- [ ] Register, login, chat, list conversations, and file upload work end-to-end
