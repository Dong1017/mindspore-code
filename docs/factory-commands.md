# Factory Commands

The current Factory command surface is intentionally small and local-first.

## Supported commands

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

If no server config is present, attempts to sync from the configured Factory pack source. The adapter exists, but no config source is currently wired; this returns a clear no-source error.

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
