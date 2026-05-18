# Agent SubAgent Specification

## Purpose
Spawning, communication, and aggregation of parallel sub-agents for complex tasks.

## Requirements

### Requirement: SubAgent Spawning

The system MUST spawn isolated sub-agents as goroutines for parallel work.

#### Scenario: Parallel coding and copywriting

- GIVEN a task "Create a landing page for a coffee shop"
- WHEN the orchestrator decomposes it
- THEN a `coder` sub-agent and a `copywriter` sub-agent are spawned
- AND both run in parallel

### Requirement: Parent-Child Communication

The system MUST provide bidirectional channels between parent and sub-agents.

#### Scenario: Task assignment

- GIVEN a spawned sub-agent
- WHEN the parent sends a Task via the inbox channel
- THEN the sub-agent receives and processes it
- AND returns a Result via the outbox channel

### Requirement: Result Aggregation

The system MUST merge results from multiple sub-agents.

#### Scenario: Merge HTML and copy

- GIVEN a `coder` returns HTML and a `copywriter` returns text
- WHEN both complete
- THEN the merger combines them into a single artifact
- AND returns the unified result to the orchestrator

### Requirement: SubAgent Cancellation

The system MUST cancel sub-agents when the parent session ends.

#### Scenario: User disconnects

- GIVEN active sub-agents processing
- WHEN the user closes the connection
- THEN all sub-agents receive cancellation signal
- AND resources are released within 5 seconds
