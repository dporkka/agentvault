#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
SCRIPT="$ROOT/scripts/verify-release-ci.sh"
SHA=0123456789abcdef0123456789abcdef01234567
OTHER=aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT
mkdir "$TMP/bin"
cat > "$TMP/bin/gh" <<'MOCK'
#!/usr/bin/env bash
printf '%s\n' "$*" > "$MOCK_ARGS_FILE"
if [[ "${MOCK_FAIL:-}" == "1" ]]; then exit 43; fi
cat "$MOCK_JSON_FILE"
MOCK
chmod +x "$TMP/bin/gh"
export PATH="$TMP/bin:$PATH" MOCK_ARGS_FILE="$TMP/args" MOCK_JSON_FILE="$TMP/response.json"
make_response() {
  jq -n --arg sha "$1" --arg branch "$2" --arg event "$3" --arg status "$4" --arg conclusion "$5" \
    '{workflow_runs:[{head_sha:$sha,head_branch:$branch,event:$event,status:$status,conclusion:$conclusion}]}' > "$MOCK_JSON_FILE"
}
expect_success() {
  if ! "$SCRIPT" "$SHA" dporkka/agentvault >/dev/null; then echo "expected success: $1" >&2; exit 1; fi
  if ! grep -q 'repos/dporkka/agentvault/actions/workflows/ci.yml/runs' "$MOCK_ARGS_FILE"; then
    echo "wrong API endpoint" >&2; exit 1
  fi
  if ! grep -q "head_sha=$SHA" "$MOCK_ARGS_FILE"; then echo "did not filter tagged head" >&2; exit 1; fi
}
expect_failure() {
  if "$SCRIPT" "$SHA" dporkka/agentvault >"$TMP/out" 2>&1; then echo "unexpected success: $1" >&2; exit 1; fi
}
make_response "$SHA" main push completed success
expect_success 'verified main push'
make_response "$OTHER" main push completed success
expect_failure 'another SHA cannot pass'
make_response "$SHA" feature push completed success
expect_failure 'feature branch cannot pass'
make_response "$SHA" main pull_request completed success
expect_failure 'PR checks cannot stand in for main push'
make_response "$SHA" main push completed failure
expect_failure 'failed main CI'
make_response "$SHA" main push in_progress null
expect_failure 'incomplete main CI'
printf '{"workflow_runs":[]}' > "$MOCK_JSON_FILE"
expect_failure 'no completed CI'
printf '{"workflow_runs":null}' > "$MOCK_JSON_FILE"
expect_failure 'malformed response'
make_response "$SHA" main push completed success
MOCK_FAIL=1 expect_failure 'API failure must block'
if "$SCRIPT" badsha dporkka/agentvault >"$TMP/out" 2>&1; then echo 'invalid SHA allowed' >&2; exit 1; fi
if "$SCRIPT" "$SHA" '../bad/repo' >"$TMP/out" 2>&1; then echo 'invalid repo allowed' >&2; exit 1; fi
echo 'release admission guard: all cases passed'
