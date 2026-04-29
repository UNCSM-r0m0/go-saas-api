# Anonymous Rate Limiting Specification

## Purpose

Rate limit unauthenticated users by `anonymousId` instead of IP address.

## Requirements

### Requirement: anonymousId Extraction

The system MUST extract `anonymousId` from the request body for anonymous users.

#### Scenario: anonymousId present

- GIVEN a POST request with JSON body containing `anonymousId: "anon_123"`
- WHEN no Authorization header is present
- THEN the anonymousId is used as the rate limit key

#### Scenario: anonymousId absent

- GIVEN a request without Authorization header and without `anonymousId`
- WHEN rate limit is checked
- THEN the client IP is used as the rate limit key

### Requirement: Cross-IP Bucket Sharing

The system MUST share rate limit buckets for the same anonymousId across different IPs.

#### Scenario: Same anonymousId different IP

- GIVEN two requests with `anonymousId: "anon_123"` from different client IPs
- WHEN both requests arrive
- THEN they share the same rate limit bucket
- AND the limit counts both requests together
