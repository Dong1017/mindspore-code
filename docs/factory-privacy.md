# Factory Privacy and Safety

Factory Pack is designed to provide bounded local hints without exposing raw contribution or pack internals to the agent prompt.

## Draft and submission validation

Card creation and submission run privacy validation before writing draft cards or review bundles. Sensitive values such as credentials, tokens, and secrets block writes.

Home paths are sanitized in generated draft content where the draft builder has the home directory context.

## Runtime hint boundaries

`/diagnose` receives a bounded Factory hint block, not raw pack contents. The hint renderer excludes raw known-issue YAML markers, SQLite internals, full logs, and internal match scores.

Factory hints are labeled as prior diagnostic hints and must not be treated as final conclusions without checking current evidence.

## Local review and approval boundaries

`/factory card review <card-id>` reads only the local submission bundle and renders a bounded review view. It does not create approved cards, build packs, sync packs, upload, create PRs/MRs, or enqueue remote review work.

Manual approval requires explicit `observed` confidence and a non-empty human rationale. Approval re-runs validation and writes only the local approved source card path without overwriting existing files.

## Local pack storage

Pack sync installs to the default user-level Factory pack path resolved by `pack.DefaultPackPath()`. The pack path is used by runtime code; it does not create a new read/grep/glob authorization surface.

## Deferred safety-sensitive surfaces

Remote registry access, server uploads, automatic PR/MR creation, review services, signing, channel selection, background sync, multi-pack management, and enterprise/private pack policy are not implemented.
