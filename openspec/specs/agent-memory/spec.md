# Agent Memory Specification

## Purpose
Persistent and ephemeral memory for agent context across sessions.

## Requirements

### Requirement: Short-Term Memory

The system MUST maintain the current conversation context in Redis.

#### Scenario: Context window

- GIVEN an active session
- WHEN a new message arrives
- THEN the last N messages (context window) are loaded from Redis
- AND the new message is appended

### Requirement: Long-Term Memory

The system SHOULD store user preferences and facts in PostgreSQL.

#### Scenario: Preference recall

- GIVEN a user previously stated "I prefer Bootstrap"
- WHEN a new session starts
- THEN the preference is loaded from long-term memory
- AND injected into the system prompt

### Requirement: Episodic Search

The system MAY search past sessions for relevant context.

#### Scenario: Session recall

- GIVEN the user asks "What did we build yesterday?"
- WHEN episodic search runs
- THEN relevant past sessions are retrieved via FTS5
- AND summarized by LLM for context injection
