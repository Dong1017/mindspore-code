# CLAUDE.md — Project Rules for AI Contributors

## Build & Test

```bash
go build ./...          # build all packages
go test ./...           # run all tests
go vet ./...            # static analysis
```

## Project Structure

```
ms-cli/
  cmd/ms-cli/main.go        # CLI entrypoint (thin wrapper)
  internal/app/              # bootstrap, wire, run — private to this repo
  agent/                     # agent brain: session, loop, planner, router, skill loading
  workflow/                  # step orchestration, DAG, retry
  runtime/                   # shell, sandbox, workspace, artifacts
  permission/                # standalone permission engine
  integrations/              # LLM providers, MCP, external APIs
  ui/                        # Bubbletea TUI panels and components
  report/                    # report generation
  trace/                     # execution tracing and logs
  configs/                   # shared configuration types
  docs/                      # architecture docs, roadmap
```

## External Dependencies

- **`vigo999/mindspore-skills`** — External skill catalog. Contains skill definitions (`skill.yaml`) that `agent/router/` loads and matches against user intent. Skill implementations live there, not in this repo.

## Architecture Boundaries

This project has strict package ownership rules. **Do not move, merge, or restructure packages** without explicit approval from the maintainer.

### Dependency Flow

Dependencies flow **downward only**. Never create upward or circular imports.

```
cmd/ms-cli → internal/app → agent, ui
                           ↓
              agent → workflow, permission, integrations, configs
                           ↓
              workflow → runtime, permission, configs
                           ↓
              runtime → configs (+ stdlib only)
              permission → configs (+ stdlib only)
              integrations → configs (+ stdlib only)
```

### Package Dependency Rules

```
cmd/ms-cli/       # Calls internal/app only. Nothing else.
internal/app/     # Wiring layer. May import anything. Must NOT be imported by any other package.
agent/            # May import workflow/, permission/, integrations/, configs/.
                  # Must NOT import internal/app/, ui/, cmd/, runtime/ directly.
workflow/         # May import runtime/, permission/, configs/.
                  # Must NOT import agent/, ui/.
runtime/          # May import configs/. Must NOT import any other internal package.
permission/       # May import configs/. Must NOT import any other internal package.
integrations/     # May import configs/. Must NOT import agent/, ui/, runtime/.
ui/               # May import agent/, configs/. Must NOT be imported by agent/ or runtime/.
configs/          # Shared types only. No imports from other internal packages.
trace/            # May import configs/. Must NOT import agent/, ui/.
report/           # May import configs/, trace/. Must NOT import agent/, ui/.
```

### Package Ownership & Purpose

| Package | Purpose | Stable? |
|---|---|---|
| `cmd/ms-cli/` | CLI entrypoint. Thin — just calls `internal/app.Run()`. | Yes |
| `internal/app/` | Config init, dependency injection, runtime assembly, CLI/TUI startup. Go-private (not importable externally). | Yes |
| `agent/` | Agent runtime: understand user intent, load/select skills, manage sessions, plan tasks, chat loop. | Evolving |
| `agent/session/` | Session state and persistence. | Evolving |
| `agent/loop/` | Core agent execution loop. | Evolving |
| `agent/planner/` | Takes selected skill + params, produces executable plan (`[]PlanStep` with dependencies). | Evolving |
| `agent/router/` | Loads skill definitions from `vigo999/mindspore-skills`, matches user intent → skill ID + params, resolves to workflows. | Evolving |
| `workflow/` | Workflow engine — reads `workflow.yaml`, step orchestration, retry, dependency DAG. | Evolving |
| `runtime/` | Execution infrastructure: shell commands, sandbox, workspace, artifacts. | Evolving |
| `runtime/shell/` | Shell command execution. | Evolving |
| `runtime/sandbox/` | Restricted execution environment. | Planned |
| `runtime/workspace/` | Working directory and file management. | Evolving |
| `runtime/artifacts/` | User-visible output files produced by execution (generated code, reports, downloaded files). **External-facing** — users consume these. | Planned |
| `permission/` | Permission engine (levels, decisions, cache). Reusable beyond CLI — shared by agent, workflow, runtime. | Yes |
| `integrations/` | External service clients (LLM providers, MCP protocol, external APIs). | Evolving |
| `ui/` | Bubbletea TUI: chat panel, task panel, logs, results. | Evolving |
| `configs/` | Configuration types and constants. | Yes |
| `trace/` | Structured event logs for debugging, replay, and session history (JSONL trajectories, tool call traces). **Internal-facing** — developers and agents consume these. | Evolving |
| `report/` | Report generation from traces/results. | Evolving |

### Core Types — Do Not Change Without Approval

These types are foundational. Changing their signatures breaks multiple packages:

- `permission.PermissionLevel` and `permission.PermissionDecision` (`permission/types.go`)
- `integrations/llm` provider interfaces and `ToolSchema`
- `agent/loop` engine interfaces
- `agent/session` session and snapshot types

If you need to extend a core type, **add new fields/methods** rather than modifying existing ones.

### Interface Boundaries Between Packages

Agent subsystems communicate through the following flow:

```
User message
    ↓
agent/router/       # loads skill definitions from mindspore-skills
                    # matches intent → SkillMatch{skillID, params}
    ↓
agent/planner/      # reads skill's workflow definition
                    # produces []PlanStep with dependencies
    ↓
workflow/           # executes the plan step-by-step
    ↓
runtime/            # runs shell commands, manages workspace/artifacts
```

Each boundary uses an interface — packages do not reach into each other's internals:

```go
// agent/planner produces this; workflow/engine consumes it
type PlanStep struct {
    ID           string
    Action       string
    Params       map[string]any
    DependsOn    []string
}

// workflow/ exposes this; agent/ calls it
type WorkflowRunner interface {
    Execute(ctx context.Context, steps []PlanStep) (Result, error)
}
```

Workflow talks to runtime through interfaces, not direct struct access.

### trace/ vs runtime/artifacts/ Boundary

These two packages handle execution outputs but serve different consumers:

| | `trace/` | `runtime/artifacts/` |
|---|---|---|
| **What** | Structured event logs (JSONL), tool call records, session trajectories | Output files: generated code, reports, downloaded files |
| **Who consumes** | Internal — developers, agents, replay/debug systems | External — end users, downstream tools |
| **Lifecycle** | Append-only during execution, read during debug/resume | Created during execution, delivered to user after |
| **Examples** | `session.jsonl`, tool call trace, planner decision log | Generated `.py` file, benchmark report, fetched dataset |

**Rules:**
- `trace/` must NOT store user-deliverable output files — those go in `runtime/artifacts/`.
- `runtime/artifacts/` must NOT store debug/replay logs — those go in `trace/`.
- `report/` may read from both `trace/` (for execution data) and `runtime/artifacts/` (for result files) to compose final reports.

### What NOT to Do

- **Do NOT merge `permission/` into any other package** — it is a shared, standalone service.
- **Do NOT create a separate `skills/` package** — skill loading and selection lives in `agent/router/`. Skill definitions live in `vigo999/mindspore-skills`, not in this repo. If skill runtime becomes complex enough to extract later, discuss with maintainer first.
- **Do NOT put skill definitions in this repo** — they belong in `vigo999/mindspore-skills`. This repo only loads and executes them.
- **Do NOT import `ui/` from `agent/`, `runtime/`, or `workflow/`** — use interfaces or event buses for communication.
- **Do NOT import `agent/` from `workflow/`** — workflow is a lower layer. If it needs agent context, receive it via interface.
- **Do NOT add direct LLM provider imports outside `integrations/`** — all LLM access goes through `integrations/llm/`.
- **Do NOT bypass permission checks** — all tool/skill execution must go through the permission layer.
- **Do NOT add new top-level packages** without discussing with the maintainer first.
- **Do NOT create packages for unbuilt features** — no empty packages. Build it when you need it.
- **Do NOT mix trace and artifact concerns** — debug/replay logs go in `trace/`, user-deliverable files go in `runtime/artifacts/`. See the boundary table above.

## Code Style

- Go standard formatting (`gofmt`/`goimports`).
- Error messages: lowercase, no punctuation, wrap with `fmt.Errorf("context: %w", err)`.
- Interfaces belong in the package that *uses* them, not the package that implements them.
- Prefer returning `error` over `panic`. Reserve `panic` for truly unrecoverable states.
- Constructor functions: `NewXxx(...)` pattern.

## Git Conventions

- Branch from `main`.
- PR titles: `feat:`, `fix:`, `refactor:`, `docs:`, `test:` prefixes.
- Keep PRs focused — one concern per PR. Do not mix refactoring with new features.
