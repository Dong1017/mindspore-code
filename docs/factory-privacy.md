# Factory Privacy and Safety

Factory Pack provides bounded local hints without exposing raw contribution or pack internals to the agent prompt.

## Validation and review boundaries

Card creation, submission, and manual approval run privacy validation. Sensitive values such as credentials, tokens, and secrets block writes.

`/factory card review <card-id>` is bounded and read-only. Manual approval is explicit, observed-confidence only, validates again, and writes only the local approved source card without overwriting existing files.

## Runtime hint boundaries

`/diagnose` receives a bounded Factory hint block, not raw pack contents. The hint renderer excludes raw known-issue YAML markers, SQLite internals, full logs, and internal match scores.

Factory hints are prior diagnostic hints, not final conclusions without current evidence.

## Local pack storage

Pack sync installs to the default user-level Factory pack path resolved by `pack.DefaultPackPath()`. The pack path is used by runtime code; it does not create a new read/grep/glob authorization surface.

## Deferred safety-sensitive surfaces

Remote registry access, server uploads, automatic PR/MR creation, review services, signing, channel selection, background sync, multi-pack management, and enterprise/private pack policy are not implemented.
