# Factory Card Schema

Factory known-issue cards use schema `known_issue/v0.5` and kind `known_issue`.

Required top-level fields: `schema_version`, `kind`, `id`, `title`, `tags`, `case`, `match`, `guidance`, `provenance`, and `governance`.

- `case` records `problem_type`, `stage`, `domain`, and `hardware`. `domain` is one of `mindspore`, `torch`, `torch_npu`, `cann`, or `unknown`.
- `match` contains bounded signals. Pack eligibility requires at least one keyword or regex.
- `guidance` contains reviewer-facing symptom, diagnosis, action/fix, verification, and non-cause material.
- `provenance.references` and `provenance.expected_behavior` are string lists and are required for pack eligibility.
- `governance` starts as draft/pending/bootstrap. Pack eligibility requires stable/approved with `confidence: observed` or `verified`. No code path auto-promotes drafts.

```yaml
schema_version: known_issue/v0.5
kind: known_issue
id: stable-ascend-import
title: torch_npu import fails when CANN runtime is not sourced
tags: [ascend]
case:
  problem_type: failure
  stage: import
  domain: torch_npu
  hardware: ascend
match:
  keywords: [torch_npu, CANN]
guidance:
  symptom: ImportError mentions torch_npu or CANN runtime dependencies.
  diagnosis: CANN runtime environment may not be sourced.
  actions: [Check whether the CANN runtime environment is visible.]
  fix: Source the CANN environment before importing torch_npu.
  verification: Run a Python import smoke test after sourcing CANN.
provenance:
  references: [script:/path/to/repro.py]
  expected_behavior: [torch_npu imports successfully after CANN runtime is visible.]
governance:
  lifecycle: stable
  review_status: approved
  confidence: observed
```
