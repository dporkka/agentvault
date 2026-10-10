#!/usr/bin/env bash
# Fail closed unless this exact release commit has a successful CI workflow
# run triggered by a push to main. PR-only and unrelated commit runs do not
# qualify; branch protection and independent review are separate requirements.
set -euo pipefail

if [[ "$#" -ne 2 ]]; then
  echo 'usage: verify-release-ci.sh <40-char-commit-sha> <owner/repo>' >&2
  exit 2
fi
commit="$1"
repo="$2"
if [[ ! "$commit" =~ ^[0-9a-fA-F]{40}$ ]] || [[ ! "$repo" =~ ^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$ ]]; then
  echo 'invalid release commit or repository identifier' >&2
  exit 2
fi

# Use the GitHub Actions workflow-run endpoint instead of checking only a
# check-run name: release requires a completed *push-to-main* CI run, not a
# green PR check whose branch was later superseded.
if ! response="$(gh api -X GET "repos/${repo}/actions/workflows/ci.yml/runs" \
  -f "head_sha=${commit}" -f 'event=push' -f 'per_page=100')"; then
  echo 'cannot verify the release commit CI status; refusing release' >&2
  exit 1
fi

if ! jq -e --arg sha "$commit" 'any(.workflow_runs[]?; .head_sha == $sha and .head_branch == "main" and .event == "push" and .status == "completed" and .conclusion == "success")' <<< "$response" >/dev/null; then
  echo 'release commit has no verified, successful push-to-main CI run; refusing release' >&2
  exit 1
fi

echo "verified successful push-to-main CI for ${commit}"
