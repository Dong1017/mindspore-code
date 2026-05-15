# Factory Commands

The current Factory command surface is intentionally small and local-first. It is the supported agent action surface for Factory work: agents should use these `/factory` commands instead of writing Factory SQLite rows directly or calling Factory server APIs by hand.

## Supported commands

| Command | Scope | Agent action |
| --- | --- | --- |
| `/factory status` | Local with optional server metadata lookup | Inspect local pack state, server configuration, latest server pack metadata, checksum match status, and local card/review counts. |
| `/factory card create` | Local only | Create a draft card from the latest `/diagnose` or `/fix` summary. |
| `/factory card submit {card-path}` | Local only | Validate a draft and create a local review bundle. |
| `/factory card review {card-id}` | Local only | Render the bounded manual review view. |
| `/factory card review {card-id} --approve --confidence observed --rationale "{manual rationale}"` | Local only | Approve a reviewed local card for later pack build. |
| `/factory pack build {cards-dir} {output-pack}` | Local only | Compile approved local cards into a pack artifact. |
| `/factory pack publish {pack-path}` | Local validation plus server upload | Publish a validated local pack to the configured `mscli-server`. |
| `/factory pack sync` | Server-backed when configured; otherwise configured local source | Install the latest configured Factory pack through the validated local sync path. |
| `/factory pack sync {source-path}` | Local only | Install an explicit local pack source through the validated local sync path. |
| `/factory pack match-debug "{diagnose text}"` | Local developer debug only | Audit local pack matching without mutating state or calling an LLM. |

### `/factory status`

Renders bounded Factory state for agents and maintainers: local pack installed/missing, local pack path and metadata when available, server configured status, config source (`env`, `credentials.json`, or `none`), latest server pack metadata when configured and reachable, local/server checksum match when both are known, and local counts for draft cards, review items, and approved cards.

If a configured server is unreachable, status still returns local state and reports `server_reachable: false` with a bounded reason.

### `/factory card create`

Creates a local draft known-issue card from the latest `/diagnose` or `/fix` run summary. If diagnose and fix summaries refer to different topics, the latest run kind controls which summary is used.

`/factory card create --from-last-run` remains supported as a compatibility alias. Other create sources are not supported.

### `/factory card submit {card-path}`

Validates a local draft card and writes a local review bundle under `factory/submissions/{card-id}/`. Successful output identifies the local review item and prints the next review command. This does not upload to a server or create a PR/MR.

### `/factory card review {card-id}`

Reads only `factory/submissions/{card-id}/card.yaml`, `summary.md`, and `validation.json`, then renders a bounded local review view with validation status, a summary excerpt, and reviewer checklist. Review mode is read-only: it does not modify the bundle, create an approved card, build a pack, sync a pack, upload, or enqueue review work.

### `/factory card review {card-id} --approve --confidence observed --rationale "{manual rationale}"`

Approves a local review item after manual review. Approval output includes the approved card path, governance changes, and a reminder that pack build is still required. Approval requires `observed` confidence and a non-empty rationale, rejects `verified`, re-runs schema/privacy/draft validation, applies stable/approved/observed governance with UTC `updated_at`, re-runs pack-readiness validation, and writes `factory/cards/{card-id}.yaml` without overwriting an existing file.

Approval remains local. It does not modify the review bundle, build or sync a pack, upload, create a PR/MR, use a review queue, or perform LLM approval.

### `/factory pack build {cards-dir} {output-pack}`

Compiles pack-eligible source cards from a local cards directory into the requested output pack. Build output is bounded and does not install, sync, upload, or publish the pack.

### `/factory pack publish {pack-path}`

Validates a locally built pack, then uploads it to the configured `mscli-server` using `MSCLI_FACTORY_SERVER_URL` and `MSCLI_FACTORY_TOKEN`. The server stores pack metadata plus the pack blob in SQLite for this internal P3.0a pilot only; this is not the long-term artifact storage design.

Duplicate checksum publishes are allowed. Each publish creates a new server row, and the latest pack is selected by `created_at DESC, id DESC`.

### `/factory pack sync`

If `MSCLI_FACTORY_SERVER_URL` and `MSCLI_FACTORY_TOKEN` are set, downloads `/factory/packs/latest/download` to a temporary file and installs it through the same validated local sync path as explicit source sync.

No-arg sync precedence is: server config, then intentionally wired configured local source, then no-source error.

If server config is not present, attempts to sync from the configured Factory pack source. The adapter exists, but no config source is currently wired; this returns a clear no-source error.

### `/factory pack sync {source-path}`

Installs an explicit local pack source into the default user-level Factory pack location after validation.

### `/factory pack match-debug "{diagnose text}"`

Loads the default local Factory pack and runs bounded local matching against the provided text for local developer/debug audit use. It is not normal user-facing diagnostic output and is not injected into `/diagnose` prompts. It does not call an LLM, run `/diagnose`, mutate cards or packs, upload data, expose raw card YAML, expose SQLite rows, or print full logs.

## Unsupported commands

Unknown `/factory`, `/factory card`, and `/factory pack` subcommands return an unsupported command or usage message. Deferred product surfaces include remote registry operations, uploads, automatic PR/MR creation, review queues, multi-pack management, signing, channel selection, background sync, and enterprise/private pack policy.

## Output

Successful pack build output includes cards directory, output path, pack name, pack schema version, card schema version, source case count, compiled case count, and checksum.

Successful pack sync output is bounded and includes source, destination, pack name, pack version, pack schema version, card schema version, compiled case count, and checksum.

Failure output reports the sync error and whether an existing local pack was preserved or no local pack was installed. It does not claim the preserved pack is valid unless separately validated.

Successful card and pack workflow outputs include a minimal `next:` hint when there is a clear follow-up command. Failure and usage output does not include `next:` hints.

## Boundaries and non-goals

The card workflow remains local-only. `/factory pack publish` and server-backed `/factory pack sync` are the only current commands that use the Factory server.

Current non-goals include `/factory card submit-dir`, server-side card submission, server-side card review, server-side card approval, server-side pack build, review queues, automatic PR/MR creation, signing, channels, multi-pack management, background sync, full JSON output coverage, and a non-interactive CLI rewrite.
