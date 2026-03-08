# ms-cli Architecture

## Layers

- `cmd/ms-cli`: single CLI entrypoint. Only parses process args and calls `internal/app`.
- `internal/app`: composition layer for config bootstrap, dependency wiring, and application startup.
- `agent`: intent handling, routing, planning, session lifecycle, and chat loop.
- `skills`: skill runtime (registry, loader, executor). Skill implementations live outside this tree.
- `workflow`: workflow engine, runner, and step contracts.
- `runtime`: command execution and runtime resources (shell, sandbox, workspace, artifacts).
- `ui`: TUI composition (`app`, `views`, `panels`, `components`).
- `report`: task result summaries and export outputs.
- `trace`: execution telemetry and timeline events.
- `configs`: static and runtime configuration loading.

## Dependency Direction

- `cmd -> internal/app`
- `internal/app -> agent|skills|workflow|runtime|ui|configs|trace|report`
- `agent -> skills|workflow`
- `workflow -> runtime`
- `ui -> agent/workflow read-only facades`
- `runtime` must not import upper layers.

## Migration Rule

During migration, keep old modules as adapters and move behavior layer by layer. Avoid cross-layer refactors in one PR.
