# Capture idempotency and filesystem publication contract

**Status:** implemented in draft [#109](https://github.com/dporkka/agentvault/pull/109), stacked on
[#108](https://github.com/dporkka/agentvault/pull/108) →
[#107](https://github.com/dporkka/agentvault/pull/107) →
[#106](https://github.com/dporkka/agentvault/pull/106) →
[#105](https://github.com/dporkka/agentvault/pull/105).
Not yet merged or deployed. This is inbox-scoped idempotency on cooperative
filesystems, **not** durable global exactly-once event processing.

## Authority and scope

- The authoritative capture is a complete Markdown document in a given vault's
  `00-inbox`. SQLite search and capture tables are **rebuildable projections**,
  not a second write authority for the capture.
- A nonempty `external_id` is an opaque, exact, case-sensitive idempotency key
  **within one vault inbox**. It is not an authorization credential.
- Same `external_id` with changed title/body/source still returns the original
  complete capture (current replay semantics). A separate strict
  `idempotency-conflict` contract would be a breaking API change.
- Empty or omitted `external_id` retains existing HTTP numbered filenames
  (`YYYY-MM-DD_capture_NNN.md`) and MCP generated-name behavior. Such captures
  are not deduplicated by payload.
- Existing clients' success shape remains: HTTP `path` and optional
  `idempotent: true`, MCP existing text response and idempotent prefix.
- No new SQLite canonical ledger, broker, lock service, or mutation approval
  requirement for trusted direct captures. Reviewed agent mutations remain
  owned by [PR #53](https://github.com/dporkka/agentvault/pull/53).

## Deterministic keyed path (draft #109)

For a *new* keyed capture, let `digest = lowercase_hex(SHA256(UTF8(external_id)))`
with the full 32-byte digest (64 hex characters). The canonical target is:

```text
<canonical vault root>/00-inbox/ext_<digest>.md
```

Hashing avoids unsafe path characters and avoids putting the raw key in the
filename. It does **not** make a low-entropy key secret; the Markdown
frontmatter retains `external_id` as a quoted string. Do not trim, lowercase,
case-fold, Unicode-normalize, or prepend a date to a key, because doing so
would silently change its identity or create a new path on each retry.
Resolve the vault root canonically before writing and reject symlinked/
non-directory inbox components: aliases must not evade the same authority.

Use the same canonical path in both HTTP and MCP. The winner's Markdown
frontmatter retains the supplied human-readable title and a deterministic
`capture_ext_<digest>` ID. A conflicting writer must return that winner's path
rather than its own unused generated ID.

## Keyed capture decision sequence

1. Validate and bound the external ID (suggested: 1–512 UTF-8 bytes,
   rejecting invalid encodings and control characters; specify migration/
   compatibility behavior before shipping the bound).
2. Resolve and verify the vault/inbox path without following unexpected
   symlinks; create the inbox when missing.
3. **Legacy compatibility phase:** scan existing `.md` captures under
   `00-inbox` for the exact frontmatter `external_id`. If one exists,
   return its existing path unchanged and mark the request idempotent.
   If multiple legacy files match, fail with an explicit duplicate-conflict
   diagnostic; never arbitrarily overwrite or silently pick one.
4. Attempt to write and fsync a uniquely named *hidden* staging file in the
   same directory. Atomically install it at `ext_<digest>.md` without
   replacing an existing destination (the #106 hard-link primitive on
   supported filesystems). Never expose a zero-byte `.md` reservation.
5. On `fs.ErrExist`, load and validate the existing target **from disk**.
   Only return `idempotent: true` if it is a complete, parseable capture with
   exactly the same `external_id`. If it is a symlink, malformed, or matches
   a different key, fail closed rather than overwrite.
6. On successful installation, return the canonical path; schedule index
   reconciliation afterward. If indexing fails, preserve the Markdown and
   report/repair projection lag, not a second capture write.

The process-local striped lock from #107 may reduce duplicate work but
must **not** be relied on for correctness between OS processes. Filesystems
that cannot support safe exclusive publication return an explicit unsupported
operation error; do not fall back to truncate-and-write or a racy
check-then-rename.

## Recovery and compatibility matrix

| Situation | Required outcome |
| --- | --- |
| Two processes publish same ID at once | Exactly one winner at the canonical path; loser verifies and returns that file as idempotent |
| Different IDs, same human title | Different key-derived paths; both survive |
| Pre-existing legacy capture with matching external ID | Reuse the legacy path; do not rename or duplicate it |
| More than one legacy capture matches | Visible conflict requiring reconciliation; no silent winner |
| Interrupted before install | Canonical `.md` absent, hidden staging may remain; retry can create it |
| Interrupted after install but before HTTP/MCP response | Retry validates canonical file and returns it idempotently |
| Destination exists but is malformed / different key / symlink | Explicit conflict, no overwrite |
| Search SQLite is missing/corrupt or indexer fails | Rebuild projection from Markdown; do not write a duplicate |
| User moves or deletes the capture | Deduplication scope ends at `00-inbox`; retries may create a new capture unless a future persistent move ledger is explicitly added |
| Deployment mixes old and new writer versions | **Not safe to advertise strong idempotency**; enforce a version rollout gate so all writers use the new path before enabling the contract |
| Two vault-root aliases reference the same directory | Resolve canonical authority first; refuse unsafe symlink paths |

### Limits

This provides **single inbox-path exclusion on cooperative local filesystems**,
not globally eternal exactly-once event processing. A deleted/moved capture,
an external writer ignoring the protocol, uncoordinated migrations, filesystem
power failure before directory metadata persistence, or a file share without
equivalent hard-link semantics can violate stronger guarantees. The design
does not claim a portable inter-process mutex or linearizable arbitrary
filesystem transactions.

## Verification and remaining limits

The implementation in #109 includes independent-process competition in the
`capturekey` tests, key-sensitive legacy lookup, malformed/wrong-key destination
conflicts, non-regular/symlink rejection, content validation, retry after an
already-completed install, Unicode IDs, and API/MCP integration tests. The
Windows/macOS workflow covers focused capture contracts; the Linux CI runs
the Go race suite. See the draft PR for **exact-head run references**.

This does **not** cover a deliberate process-kill fault at every staging/fsync/
link boundary, durable directory metadata under sudden power loss, all
network/shared filesystem substrates, or guaranteed recovery after a user
moves/deletes a note. The cross-process helper is only safe when **all writers
using that vault** adopt the deterministic keyed path protocol; rolling out
mixed-version writers concurrently can still create duplicates. A legacy
scan followed by a new path install is not atomic with an unupgraded writer.

`external_id` is limited to 1–512 valid UTF-8 bytes with no control
characters. This is a new admission constraint: deployers should identify
any legacy clients using larger/control-containing keys before migration.

Unkeyed captures retain existing naming behavior and are not assigned a
deterministic path. The separate reviewed mutation engine in #53 remains
independent; direct capture creation does not imply agent approval.
