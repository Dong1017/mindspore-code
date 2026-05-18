# Factory Commands

The Factory command surface is intentionally small and local-first. `/factory ...` slash commands are the interactive TUI Factory surface. `mscli factory ...` commands are the non-interactive surface for external agents, shell scripts, and CI.

Both surfaces share the same command semantics and policy boundaries. External automation should call `mscli factory ...` directly instead of driving TUI slash input.

## Interactive command matrix

| Command | Scope | Interactive action |
| --- | --- | --- |
| `/factory status` | Local with optional server metadata lookup | Inspect local pack state, server configuration, latest server pack metadata, checksum match status, and local card/review counts. |
| `/factory card create` | Local only | Create a draft card from the latest `/diagnose` or `/fix` summary. |
| `/factory card submit {card-path}` | Local only | Validate a draft and create a local review bundle. |
| `/factory card review {card-id}` | Local only | Render the bounded manual review view. |
| `/factory card review {card-id} --approve --confidence observed --rationale "{manual rationale}"` | Local only | Approve a reviewed local card for later pack build. |
| `/factory pack build {cards-dir} {output-pack}` | Local only | Compile approved local cards into a pack artifact. |
| `/factory pack publish {pack-path}` | Local validation plus server upload | Publish a validated local pack to the configured `mscli-server`. |
| `/factory pack sync` | Server-backed when configured; otherwise no-source error | If server config exists, install latest server pack through the validated local sync path. Without server config, pass an explicit `{source-path}`. |
| `/factory pack sync {source-path}` | Local only | Install an explicit local pack source through the validated local sync path. |
| `/factory pack match-debug "{diagnose text}"` | Local developer debug only | Audit local pack matching without mutating state or calling an LLM. |

## Non-interactive CLI matrix

External agents, shell scripts, and CI should use `mscli factory ...` commands rather than TUI slash-command input. The non-interactive commands return zero on success and non-zero on usage, validation, and server failures.

| Command | Scope | Non-interactive action |
| --- | --- | --- |
| `mscli factory status` | Local with optional server metadata lookup | Inspect local pack state, server configuration, latest server pack metadata, checksum match status, and local card/review counts. |
| `mscli factory card submit {card-path}` | Local only | Validate a draft and create a local review bundle. |
| `mscli factory card review {card-id}` | Local only | Render the bounded manual review view. |
| `mscli factory card review {card-id} --approve --confidence observed --rationale "{manual rationale}"` | Local only | Approve a reviewed local card for later pack build. |
| `mscli factory pack build {cards-dir} {output-pack}` | Local only | Compile approved local cards into a pack artifact. |
| `mscli factory pack publish {pack-path}` | Local validation plus server upload | Publish a validated local pack to the configured `mscli-server`. |
| `mscli factory pack sync` | Server-backed when configured; otherwise no-source error | If server config exists, install latest server pack through the validated local sync path. Without server config, pass an explicit `{source-path}`. |
| `mscli factory pack sync {source-path}` | Local only | Install an explicit local pack source through the validated local sync path. |
| `mscli factory pack match-debug "{diagnose text}"` | Local developer debug only | Audit local pack matching without mutating state or calling an LLM. |

`mscli factory card create` is not implemented because card creation depends on the interactive session's latest `/diagnose` or `/fix` run summary.

### `/factory status` / `mscli factory status`

Renders bounded Factory state for agents and maintainers: local pack installed/missing, local pack path and metadata when available, server configured status, config source (`env`, `credentials.json`, or `none`), latest server pack metadata when configured and reachable, local/server checksum match when both are known, and local counts for draft cards, review items, and approved cards.

If a configured server is unreachable, status still returns local state and reports `server_reachable: false` with a bounded reason.

### `/factory card create`

Creates a local draft known-issue card from the latest `/diagnose` or `/fix` run summary. If diagnose and fix summaries refer to different topics, the latest run kind controls which summary is used.

`/factory card create --from-last-run` remains supported as a compatibility alias. Other create sources are not supported.

### `/factory card submit {card-path}` / `mscli factory card submit {card-path}`

Validates a local draft card and writes a local review bundle under `factory/submissions/{card-id}/`. Successful output identifies the local review item and prints the next review command. This does not upload to a server or create a PR/MR.

### `/factory card review {card-id}` / `mscli factory card review {card-id}`

Reads only `factory/submissions/{card-id}/card.yaml`, `summary.md`, and `validation.json`, then renders a bounded local review view with validation status, a summary excerpt, and reviewer checklist. Review mode is read-only: it does not modify the bundle, create an approved card, build a pack, sync a pack, upload, or enqueue review work.

### `/factory card review {card-id} --approve --confidence observed --rationale "{manual rationale}"` / `mscli factory card review {card-id} --approve --confidence observed --rationale "{manual rationale}"`

Approves a local review item after manual review. Approval output includes the approved card path, governance changes, and a reminder that pack build is still required. Approval requires `observed` confidence and a non-empty rationale, rejects `verified`, re-runs schema/privacy/draft validation, applies stable/approved/observed governance with UTC `updated_at`, re-runs pack-readiness validation, and writes `factory/cards/{card-id}.yaml` without overwriting an existing file.

Approval remains local. It does not modify the review bundle, build or sync a pack, upload, create a PR/MR, use a review queue, or perform LLM approval.

### `/factory pack build {cards-dir} {output-pack}` / `mscli factory pack build {cards-dir} {output-pack}`

Compiles pack-eligible source cards from a local cards directory into the requested output pack. Build output is bounded and does not install, sync, upload, or publish the pack.

### `/factory pack publish {pack-path}` / `mscli factory pack publish {pack-path}`

Validates a locally built pack, then uploads it to the configured `mscli-server` using `MSCLI_FACTORY_SERVER_URL` and `MSCLI_FACTORY_TOKEN`. The server stores pack metadata plus the pack blob in SQLite for this internal P3.0a pilot only; this is not the long-term artifact storage design.

Duplicate checksum publishes are allowed. Each publish creates a new server row, and the latest pack is selected by `created_at DESC, id DESC`.

### `/factory pack sync` / `mscli factory pack sync`

If `MSCLI_FACTORY_SERVER_URL` and `MSCLI_FACTORY_TOKEN` are set, downloads `/factory/packs/latest/download` to a temporary file and installs it through the same validated local sync path as explicit source sync.

No-arg sync behavior is the same on both surfaces: with server config it downloads and installs the latest server pack; without server config it returns a clear no-source error. Pass `{source-path}` to perform an explicit local sync.

### `/factory pack sync {source-path}` / `mscli factory pack sync {source-path}`

Installs an explicit local pack source into the default user-level Factory pack location after validation.

### `/factory pack match-debug "{diagnose text}"` / `mscli factory pack match-debug "{diagnose text}"`

Loads the default local Factory pack and runs bounded local matching against the provided text for local developer/debug audit use. It is not normal user-facing diagnostic output and is not injected into `/diagnose` prompts. It does not call an LLM, run `/diagnose`, mutate cards or packs, upload data, expose raw card YAML, expose SQLite rows, or print full logs.

## Unsupported commands

Unknown `/factory`, `/factory card`, and `/factory pack` subcommands return an unsupported command or usage message. Unknown `mscli factory`, `mscli factory card`, and `mscli factory pack` subcommands return non-zero with an unsupported command or usage message. Deferred product surfaces include remote registry operations, uploads, automatic PR/MR creation, review queues, multi-pack management, signing, channel selection, background sync, and enterprise/private pack policy.

## Output

Successful pack build output includes cards directory, output path, pack name, pack schema version, card schema version, source case count, compiled case count, and checksum.

Successful pack sync output is bounded and includes source, destination, pack name, pack version, pack schema version, card schema version, compiled case count, and checksum.

Failure output reports the sync error and whether an existing local pack was preserved or no local pack was installed. It does not claim the preserved pack is valid unless separately validated.

Successful card and pack workflow outputs include a minimal `next:` hint when there is a clear follow-up command. Failure and usage output does not include `next:` hints.

## Boundaries and non-goals

The card workflow remains local-only. `/factory pack publish`, `mscli factory pack publish`, server-backed `/factory pack sync`, and server-backed `mscli factory pack sync` are the only current commands that use the Factory server.

Current non-goals include `mscli factory card create`, `/factory card submit-dir`, server-side card submission, server-side card review, server-side card approval, server-side pack build, direct DB/server API access for agents, review queues, automatic PR/MR creation, signing, channels, multi-pack management, background sync, full JSON output coverage, and a broad CLI rewrite.
