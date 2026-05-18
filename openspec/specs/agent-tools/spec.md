# Agent Tools Specification

## Purpose
Registry and execution of tools that agents can invoke to interact with the system and external resources.

## Requirements

### Requirement: Tool Registry

The system MUST maintain a registry of available tools with name, description, parameters schema, and handler.

#### Scenario: Tool discovery

- GIVEN the orchestrator loads
- WHEN it queries the tool registry
- THEN it receives a list of all registered tools with JSONSchema parameters

### Requirement: File Write Tool

The system MUST provide a `file_write` tool that persists generated content as artifacts.

#### Scenario: Writing an HTML artifact

- GIVEN a tool call `file_write(path="index.html", content="<html>...</html>")`
- WHEN the tool executes
- THEN an artifact record is created in the database
- AND the content is stored in the artifact store
- AND the artifact ID is returned

#### Scenario: Overwrite protection

- GIVEN a tool call targets an existing artifact path
- WHEN the tool executes without `overwrite=true`
- THEN it SHALL return an error
- AND suggest using a new filename or explicit overwrite

### Requirement: Code Execution Tool

The system MUST provide a `code_execute` tool that sends code to the sandbox service.

#### Scenario: Execute HTML preview

- GIVEN a tool call `code_execute(language="html", code="<html>...</html>")`
- WHEN the tool executes
- THEN it sends the payload to `sandbox-service`
- AND returns a preview URL

#### Scenario: Sandbox timeout

- GIVEN code execution exceeds 30 seconds
- WHEN the sandbox timeout triggers
- THEN the tool returns `error: execution_timeout`
- AND the sandbox process is killed

### Requirement: Web Search Tool

The system SHOULD provide a `web_search` tool for retrieving current information.

#### Scenario: Search and summarize

- GIVEN a tool call `web_search(query="latest Go version")`
- WHEN the tool executes
- THEN it queries DuckDuckGo or similar
- AND returns the top 3 results with title, snippet, and URL
