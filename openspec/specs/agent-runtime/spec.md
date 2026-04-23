# Agent Runtime Specification

## Purpose
Orquesta el procesamiento de mensajes del usuario, selecciona el tipo de agente, gestiona el ciclo de vida de sesiones y coordina tool calls.

## Requirements

### Requirement: Session Lifecycle

The system MUST create a unique session per conversation. The session MUST track agent type, context window, and active tool calls.

#### Scenario: New session creation

- GIVEN a user sends a message without an existing session
- WHEN the orchestrator receives the request
- THEN a new `agent_session` row is created with `agent_type='chat'`
- AND the session ID is returned in the response headers

#### Scenario: Session resumption

- GIVEN a user sends a message with `X-Session-ID` header
- WHEN the orchestrator receives the request
- THEN the existing session context is loaded from Redis
- AND the conversation continues with full history

### Requirement: Agent Type Routing

The system MUST classify the user intent and route to the appropriate agent type.

#### Scenario: Coder agent selection

- GIVEN the user prompt contains keywords like "create", "build", "make", "html", "css", "page"
- WHEN the classifier analyzes the prompt
- THEN `agent_type` SHALL be set to `coder`
- AND the coder system prompt is injected into the LLM context

#### Scenario: Conversational fallback

- GIVEN no specific agent type is detected
- WHEN the classifier analyzes the prompt
- THEN `agent_type` SHALL default to `conversational`

### Requirement: Streaming Response

The system MUST stream LLM responses via Server-Sent Events (SSE).

#### Scenario: SSE streaming

- GIVEN a valid chat request
- WHEN the LLM generates tokens
- THEN each token chunk is streamed to the client as `data: {json}\n\n`
- AND the final event contains `done: true`

#### Scenario: Tool call in stream

- GIVEN the LLM response contains a tool call
- WHEN the tool call is detected in the stream
- THEN streaming pauses
- AND the tool is executed
- AND the result is injected back into context
- AND streaming resumes

### Requirement: Error Handling

The system MUST handle LLM failures gracefully.

#### Scenario: LLM timeout

- GIVEN the LLM does not respond within 30 seconds
- WHEN the timeout triggers
- THEN the user receives an SSE event with `error: timeout`
- AND the session is marked with error status

#### Scenario: Provider fallback

- GIVEN the primary LLM provider returns 5xx
- WHEN the orchestrator detects the failure
- THEN it SHALL attempt the next provider by priority
- AND the user receives a seamless response
