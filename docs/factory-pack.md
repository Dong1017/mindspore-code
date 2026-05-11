# Factory Pack

Factory Pack is a local, curated diagnostic hint workflow for MindSpore CLI. Accepted known-issue cards can be compiled into a local SQLite pack and used as bounded hints during `/diagnose`.

See `docs/factory-commands.md` for command syntax and UX details.

## Runtime flow

```text
/diagnose input
  -> build diagnostic context
  -> load local factory-core.pack
  -> match known issue cases
  -> render bounded Factory hint block
  -> pass task + hints to the agent loop
```

The runtime does not expose raw cards, SQLite rows, full logs, or internal scores in the hint block.

## Local workflow boundaries

Drafts and review bundles remain local. Submission is not a server upload. Approval is local manual promotion to an approved source card, not pack build or installation.

Build does not install or sync the output pack. Sync validates a local source pack, installs only after validation succeeds, and preserves existing installed packs on failure where practical.

## Deferred

Factory Pack does not currently implement remote registry, remote upload, automatic PR/MR creation, review service, multi-pack support, pack signing, channel selection, enterprise/private pack policy, background sync, or a new config subsystem.
