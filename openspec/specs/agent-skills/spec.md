# Agent Skills Specification

## Purpose
Registry and execution of reusable skills that optimize agent performance for common tasks.

## Requirements

### Requirement: Skill Registry

The system MUST store skills with pattern, template, and metadata.

#### Scenario: Skill lookup

- GIVEN a task matching a known skill pattern
- WHEN the orchestrator queries the registry
- THEN the matching skill template is returned
- AND the template is injected into the prompt

### Requirement: Skill Execution

The system MUST execute skills with variable substitution.

#### Scenario: Execute HTML page skill

- GIVEN a skill template for "single-page HTML"
- WHEN the user asks for a landing page
- THEN the template is populated with user requirements
- AND sent to the LLM for generation

### Requirement: Manual Skill Creation

The system SHOULD allow admins to create skills manually.

#### Scenario: Admin creates skill

- GIVEN a POST `/admin/skills` with `{name, pattern, template}`
- WHEN an admin submits it
- THEN the skill is stored in the database
- AND available for future sessions
