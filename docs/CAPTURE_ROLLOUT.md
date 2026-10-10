# Keyed capture rollout and recovery preflight

**Status:** operational checklist for draft [#109](https://github.com/dporkka/agentvault/pull/109) and its read-only audit in draft [#111](https://github.com/dporkka/agentvault/pull/111). **Neither is deployed by this document.**

The exact-key deterministic path protocol provides one winner among cooperating processes sharing a supported local filesystem. It is **not** safe to run old random-name writers beside new keyed-path writers. The inbox Markdown remains authoritative; SQLite can be reindexed.

## Controlled cutover

1. **Identify every writer** for the vault, including API instances, MCP servers, scheduled agents, desktop sessions and CLI integrations. Agree a maintenance boundary, stop and verify that all old writers are quiesced. GitHub CI alone cannot establish this runtime fact.
2. Back up the vault, including `00-inbox`, and retain its original ownership and permissions. Avoid tools that rewrite frontmatter or normalize keys during backup.
3. With writing paused, run `agentvault --vault /path/to/vault capture audit`. The command emits a JSON report to stdout. Redirect the report to a location **outside the vault** if a written artifact is needed. A nonzero exit code blocks rollout.
4. Inspect every `duplicates` group (paths plus SHA-256 key fingerprint), `invalid_canonical` entry, and `malformed_files` entry. Resolve **manually** with a backed-up, reviewed decision; do not arbitrarily select the first matching file, delete duplicates, or convert an existing key into a different identity. Re-run the audit until it is clean. The report avoids raw external IDs but paths and fingerprints can still be sensitive.
5. Upgrade **all** writers using a vault to the new protocol while writes remain stopped. Do not activate a partial rolling deployment or let an older MCP process remain connected.
6. Resume writes, then verify one new `external_id` capture through the normal application interface and retry that same key. Confirm the path remains `00-inbox/ext_<64-hex>.md`, the retry reports idempotency, the original Markdown bytes remain unchanged, and the search projection catches up. Use a synthetic test key and avoid modifying real captured events for probes.
7. If rollback is necessary, stop writers again. **Do not** reactivate a writer that uses random paths for keyed events alongside the new version. Backward compatibility needs an explicit rollback plan; keeping the new protocol deployed is safer than an uncontrolled mixed-version rollback.

## Crash boundary

`atomicfile.WriteNewFile` stages complete data, syncs and closes the file, and publishes it through an exclusive same-directory hard link. Tests in `core/internal/atomicfile/crash_test.go` terminate a child process immediately **before** or **after** the link operation. A pre-link crash leaves no visible final `.md`; a post-link crash leaves the complete final note for retries to discover. A crash can leave hidden `.tmp-` staging files.

**Never automatically sweep staging files that might still belong to active writers.** A conservative cleanup operation requires a separate ownership/age/open-handle policy and a quiescence verification. Deleting stale staging files is not part of the audit and not part of the capture write operation.

## What the audit does not establish

A read-only audit represents one filesystem snapshot. It cannot freeze incoming writers, prove every writer version is compatible, prove directory metadata is durable across sudden power loss, validate remote network filesystem semantics, or recover keys from files that were moved outside `00-inbox`. It intentionally never repairs, renames or deletes anything.

A passing GitHub Actions job is necessary but not sufficient to promote a mixed-version production deployment. Promotion requires an operator-confirmed writer inventory, backup, clean audit, coordinated cutover, and retry verification.
