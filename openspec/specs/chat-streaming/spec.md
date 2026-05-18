# Chat Streaming Specification

## Purpose
Streaming de conversaciones con LLMs vía Server-Sent Events (SSE). Evoluciona de chat simple a chat con tool calls integrado.

## Requirements

### Requirement: SSE Endpoint

The system MUST provide a POST `/agent/chat` endpoint that streams responses.

#### Scenario: Basic streaming

- GIVEN a POST `/agent/chat` with `{message, model, session_id}`
- WHEN the request is valid
- THEN the response headers include `Content-Type: text/event-stream`
- AND tokens are streamed as SSE events

#### Scenario: Streaming with tool execution

- GIVEN a chat request that triggers a tool call
- WHEN the LLM emits a tool call during streaming
- THEN the stream pauses
- AND a `tool_call` event is sent to the client
- AND after execution, a `tool_result` event is sent
- AND the LLM response resumes

### Requirement: Model Selection

The system MUST allow users to select the LLM model.

#### Scenario: Explicit model

- GIVEN a request with `model: "qwen2.5-coder:7b"`
- WHEN the request is processed
- THEN the specified model is used

#### Scenario: Default model

- GIVEN a request without model field
- WHEN the request is processed
- THEN the system default model is used

### Requirement: Anonymous Chat

The system MUST allow anonymous users to chat with rate limits.

#### Scenario: Anonymous session

- GIVEN a request without Authorization header
- WHEN the request is processed
- THEN an anonymous session is created
- AND daily message limit (3) is enforced
