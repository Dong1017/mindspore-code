# Factory Pack

Factory Pack is a local, curated diagnostic hint workflow for MindSpore CLI. It lets accepted known-issue cards be compiled into a local SQLite pack and used as bounded hints during `/diagnose`.

## Implemented workflow

### P0 runtime diagnostic closure

`/diagnose` can load the local `factory-core.pack`, match the current diagnostic fingerprint against approved cases, and append a bounded `[Factory Diagnostic Hints]` block to the task. Hints are prior evidence, not final conclusions.

### P1 contribution closure

`/factory card create --from-last-run` creates a draft known-issue card from the latest `/diagnose` or `/fix` summary.

`/factory card submit <card-path>` validates a draft card and writes a local review bundle under `factory/submissions/`.

### P1.5 card schema closure

Known-issue cards use `known_issue/v0.5`. Pack compilation records the expected card schema in the pack manifest as `card_schema_version`.

### P2 local distribution closure

`/factory pack sync [source-path]` installs a local pack source into the default user-level Factory pack location. The source pack is copied to a temporary file, validated with `pack.Load`, and installed only after validation succeeds.

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

## Contribution flow

```text
/diagnose or /fix run
  -> latest run summary stored in memory
  -> /factory card create --from-last-run
  -> draft YAML written under factory/cards/drafts/
  -> /factory card submit <card-path>
  -> local review bundle written under factory/submissions/
```

Drafts remain local. Submission means a local review bundle, not a server upload.

## Local sync flow

```text
/factory pack sync <source-path>
  -> resolve local source path
  -> resolve destination with pack.DefaultPackPath()
  -> copy source to temp file in destination directory
  -> validate temp pack with pack.Load
  -> backup existing destination with hidden temp backup
  -> install validated pack
  -> remove temporary files where practical
```

## Deferred

Factory Pack does not currently implement remote registry, remote upload, automatic PR/MR creation, review service, multi-pack support, pack signing, channel selection, enterprise/private pack policy, background sync, or a new config subsystem.
