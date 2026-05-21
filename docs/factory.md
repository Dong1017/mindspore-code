# Factory

Factory is a local-first known-issue card and pack workflow. `/factory ...` is the interactive TUI surface; `mscli factory ...` is the non-interactive agent/script/CI surface. Both share command logic.

## Commands

| Workflow | TUI | CLI | Scope |
| --- | --- | --- | --- |
| Status | `/factory status` | `mscli factory status` | Local state plus optional server metadata. |
| Create draft | `/factory card create` | Not supported | TUI-only latest `/diagnose` or `/fix` summary. |
| Submit | `/factory card submit {card-path}` | `mscli factory card submit {card-path}` | Local validation and review bundle. |
| Review | `/factory card review {card-id}` | `mscli factory card review {card-id}` | Read-only local review. |
| Approve | `/factory card review {card-id} --approve --confidence observed --rationale "{manual rationale}"` | `mscli factory card review {card-id} --approve --confidence observed --rationale "{manual rationale}"` | Local manual approval. |
| Build | `/factory pack build {cards-dir} {output-pack}` | `mscli factory pack build {cards-dir} {output-pack}` | Compile only. |
| Publish | `/factory pack publish {pack-path}` | `mscli factory pack publish {pack-path}` | Validate with `pack.Load`, then upload. |
| Sync latest | `/factory pack sync` | `mscli factory pack sync` | Server latest when configured; otherwise no-source error. |
| Sync source | `/factory pack sync {source-path}` | `mscli factory pack sync {source-path}` | Explicit local source via validated `pack.Sync`. |
| Match debug | `/factory pack match-debug "{diagnose text}"` | `mscli factory pack match-debug "{diagnose text}"` | Local bounded matching audit. |

## Pack and safety boundaries

Factory Pack compiles approved cards into a local SQLite pack used as bounded `/diagnose` hints. Hints exclude raw cards, SQLite rows, full logs, and internal scores.

Card submit/review/approve is local-only. Approval requires `observed` confidence and rationale, reruns validation, and writes `factory/cards/{card-id}.yaml` without overwrite. Pack sync validates before install and preserves the existing pack on failure where practical. Status and match-debug are read-only.

No submit-dir, server-side card workflows/build, direct DB/server API agent access, review queues, automatic PR/MR creation, signing, channels, multi-pack management, background sync, full JSON output, or broad CLI rewrite.
