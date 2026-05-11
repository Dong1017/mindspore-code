# Factory Commands

The current Factory command surface is intentionally small.

## Supported commands

### `/factory card create --from-last-run`

Creates a local draft known-issue card from the latest `/diagnose` or `/fix` run summary. If diagnose and fix summaries refer to different topics, the latest run kind controls which summary is used.

### `/factory card submit <card-path>`

Validates a local draft card and writes a local review bundle. This does not upload to a server or create a PR/MR.

### `/factory pack sync`

Attempts to sync from the configured Factory pack source. The adapter exists, but no config source is currently wired; this returns a clear no-source error.

### `/factory pack sync <source-path>`

Installs a local pack source into the default user-level Factory pack location after validation.

## Unsupported commands

Unknown `/factory` and `/factory pack` subcommands return an unsupported command message. Deferred product surfaces include remote registry operations, uploads, automatic PR/MR creation, review queues, multi-pack management, signing, channel selection, background sync, and enterprise/private pack policy.

## Output

Successful pack sync output is bounded and includes source, destination, pack name, pack version, pack schema version, card schema version, compiled case count, and checksum.

Failure output reports the sync error and whether an existing local pack was preserved or no local pack was installed. It does not claim the preserved pack is valid unless separately validated.
