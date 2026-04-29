# Delta for Chat Streaming

## MODIFIED Requirements

### Requirement: Anonymous Chat

The system MUST allow anonymous users to chat with rate limits, using `anonymousId` from the request body when provided.

(Previously: Anonymous sessions were created without tracking anonymousId; rate limiting used IP only.)

#### Scenario: Anonymous session

- GIVEN a request without Authorization header
- WHEN the request is processed
- THEN an anonymous session is created
- AND daily message limit (3) is enforced

#### Scenario: Anonymous with anonymousId

- GIVEN a request with `anonymousId: "abc123"` in the body and no Authorization header
- WHEN the request is processed
- THEN the anonymousId is propagated to downstream services via `X-Anonymous-ID` header
- AND rate limiting uses the anonymousId as key

#### Scenario: Anonymous without anonymousId fallback

- GIVEN a request without Authorization header and without `anonymousId`
- WHEN the request is processed
- THEN rate limiting falls back to client IP
- AND `X-Anonymous-ID` header is NOT set
