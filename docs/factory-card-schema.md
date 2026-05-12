# Factory Card Schema

Factory known-issue cards use schema `known_issue/v0.5` and kind `known_issue`.

## Top-level shape

A card contains:

- `schema_version`
- `kind`
- `id`
- `title`
- `tags`
- `case`
- `match`
- `guidance`
- `provenance`
- `governance`

## Case metadata

`case` describes the issue category and applicability metadata:

- `problem_type`
- `stage`
- `domain`
- `hardware`

`domain` must be one of `mindspore`, `torch`, `torch_npu`, `cann`, or `unknown`. Unknown or unclear values should use `unknown`, not guessed values.

## Matching metadata

`match` contains bounded match signals such as keywords and regex patterns. Pack eligibility requires at least one keyword or regex.

## Guidance

`guidance` contains reviewer-facing diagnostic material:

- `symptom`
- `trigger_signals`
- `diagnosis`
- `diagnosis_details`
- `actions`
- `fix`
- `verification`
- `non_causes`

Generated diagnose-only drafts must include a non-empty verification placeholder so reviewers know validation is still required.

## Provenance

`provenance` records source references and expected behavior. `references` and `expected_behavior` are lists of strings. Pack-eligible cards require references and expected behavior.

```yaml
provenance:
  references:
    - "script:/path/to/repro.py"
    - "report:out/report.md"
    - "command:python repro.py --mode broken"
```

Do not encode references as structured objects in `known_issue/v0.5`.

## Governance

Draft cards are generated with:

- `lifecycle: draft`
- `review_status: pending`
- `confidence: bootstrap`

Pack eligibility requires:

- `lifecycle: stable`
- `review_status: approved`
- `confidence: observed` or `verified`
- non-empty match signals
- non-empty symptom, diagnosis, verification, references, and expected behavior

No code path auto-promotes draft cards to stable, approved, observed, or verified.

## Minimal example

```yaml
schema_version: known_issue/v0.5
kind: known_issue
id: stable-ascend-import
title: torch_npu import fails when CANN runtime is not sourced
tags:
  - ascend
case:
  problem_type: failure
  stage: import
  domain: torch_npu
  hardware: ascend
match:
  keywords:
    - torch_npu
    - CANN
guidance:
  symptom: ImportError mentions torch_npu or CANN runtime dependencies.
  diagnosis: CANN runtime environment may not be sourced.
  actions:
    - Check whether the CANN runtime environment is visible.
  fix: Source the CANN environment before importing torch_npu.
  verification: Run a Python import smoke test after sourcing CANN.
provenance:
  references:
    - "script:/path/to/repro.py"
    - "report:out/report.md"
    - "command:python repro.py --mode broken"
  expected_behavior:
    - torch_npu imports successfully after CANN runtime is visible.
governance:
  lifecycle: stable
  review_status: approved
  confidence: observed
```
