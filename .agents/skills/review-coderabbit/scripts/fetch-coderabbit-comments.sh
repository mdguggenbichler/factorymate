#!/usr/bin/env bash
# Fetch full CodeRabbit PR comments via GitHub API (no terminal truncation).
# Usage: fetch-coderabbit-comments.sh <PR_NUMBER> [--repo owner/repo]
set -euo pipefail

CODERABBIT_LOGIN="coderabbitai[bot]"
PR_NUMBER=""
REPO=""

usage() {
  cat <<EOF
Usage: $(basename "$0") <PR_NUMBER> [--repo owner/repo]

Fetches CodeRabbit review comments for a pull request and writes them to:
  .agents/project/pr-reviews/.raw/pr-<N>/

Requires: gh, jq
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    -h|--help)
      usage
      exit 0
      ;;
    --repo)
      REPO="${2:?--repo requires owner/repo}"
      shift 2
      ;;
    *)
      if [[ -z "$PR_NUMBER" ]]; then
        PR_NUMBER="$1"
      else
        echo "error: unexpected argument: $1" >&2
        usage >&2
        exit 1
      fi
      shift
      ;;
  esac
done

if [[ -z "$PR_NUMBER" ]]; then
  echo "error: PR number required" >&2
  usage >&2
  exit 1
fi

if ! command -v gh >/dev/null 2>&1; then
  echo "error: gh CLI not found" >&2
  exit 1
fi

if ! command -v jq >/dev/null 2>&1; then
  echo "error: jq not found" >&2
  exit 1
fi

# Resolve repo root (script lives in .agents/skills/review-coderabbit/scripts/)
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../../../.." && pwd)"

if [[ -z "$REPO" ]]; then
  REPO="$(gh repo view --json nameWithOwner --jq .nameWithOwner 2>/dev/null || true)"
fi
if [[ -z "$REPO" ]]; then
  echo "error: could not detect repo; pass --repo owner/repo" >&2
  exit 1
fi

OUT_DIR="$REPO_ROOT/.agents/project/pr-reviews/.raw/pr-${PR_NUMBER}"
CR_DIR="$OUT_DIR/coderabbit"
INLINE_DIR="$CR_DIR/inline"
mkdir -p "$INLINE_DIR"

echo "Fetching PR #${PR_NUMBER} from ${REPO}..."

PR_JSON="$(gh pr view "$PR_NUMBER" --repo "$REPO" \
  --json title,url,headRefName,baseRefName,headRefOid,author 2>/dev/null)" || {
  echo "error: PR #${PR_NUMBER} not found in ${REPO}" >&2
  exit 1
}

echo "$PR_JSON" >"$OUT_DIR/pr-meta.json"

API_BASE="repos/${REPO}/pulls/${PR_NUMBER}"

echo "  reviews..."
gh api --paginate "${API_BASE}/reviews" >"$OUT_DIR/reviews.json"

echo "  issue comments..."
gh api --paginate "repos/${REPO}/issues/${PR_NUMBER}/comments" >"$OUT_DIR/issue-comments.json"

echo "  review comments (inline)..."
gh api --paginate "${API_BASE}/comments" >"$OUT_DIR/review-comments.json"

# Filter CodeRabbit only
jq --arg login "$CODERABBIT_LOGIN" '[.[] | select(.user.login == $login)]' \
  "$OUT_DIR/reviews.json" >"$OUT_DIR/coderabbit-reviews.json"

jq --arg login "$CODERABBIT_LOGIN" '[.[] | select(.user.login == $login)]' \
  "$OUT_DIR/issue-comments.json" >"$OUT_DIR/coderabbit-issue-comments.json"

jq --arg login "$CODERABBIT_LOGIN" '[.[] | select(.user.login == $login)]' \
  "$OUT_DIR/review-comments.json" >"$OUT_DIR/coderabbit-review-comments.json"

# Extract review body (use the largest non-empty body if multiple reviews exist)
REVIEW_COUNT="$(jq 'length' "$OUT_DIR/coderabbit-reviews.json")"
if [[ "$REVIEW_COUNT" -gt 0 ]]; then
  jq -r 'sort_by(.body | length) | reverse | .[0].body // empty' \
    "$OUT_DIR/coderabbit-reviews.json" >"$CR_DIR/review-body.md"
  REVIEW_ID="$(jq -r 'sort_by(.body | length) | reverse | .[0].id // empty' \
    "$OUT_DIR/coderabbit-reviews.json")"
else
  : >"$CR_DIR/review-body.md"
  REVIEW_ID=""
fi

# Summary comment: largest CodeRabbit issue comment body
ISSUE_COMMENT_COUNT="$(jq 'length' "$OUT_DIR/coderabbit-issue-comments.json")"
if [[ "$ISSUE_COMMENT_COUNT" -gt 0 ]]; then
  jq -r 'sort_by(.body | length) | reverse | .[0].body // empty' \
    "$OUT_DIR/coderabbit-issue-comments.json" >"$CR_DIR/summary-comment.md"
  SUMMARY_COMMENT_ID="$(jq -r 'sort_by(.body | length) | reverse | .[0].id // empty' \
    "$OUT_DIR/coderabbit-issue-comments.json")"
else
  : >"$CR_DIR/summary-comment.md"
  SUMMARY_COMMENT_ID=""
fi

# Inline comments
INLINE_COUNT=0
while IFS=$'\t' read -r cid body path line; do
  [[ -z "$cid" ]] && continue
  INLINE_COUNT=$((INLINE_COUNT + 1))
  {
    echo "---"
    echo "comment_id: ${cid}"
    echo "path: ${path}"
    echo "line: ${line}"
    echo "---"
    echo
    printf '%s' "$body"
  } >"$INLINE_DIR/${cid}.md"
done < <(jq -r '.[] | [.id, .body, .path, (.line // "null")] | @tsv' \
  "$OUT_DIR/coderabbit-review-comments.json")

# Build meta.json with body lengths for truncation checks
jq -n \
  --arg pr_number "$PR_NUMBER" \
  --arg repo "$REPO" \
  --arg fetched_at "$(date -u +"%Y-%m-%dT%H:%M:%SZ")" \
  --arg review_id "$REVIEW_ID" \
  --arg summary_comment_id "$SUMMARY_COMMENT_ID" \
  --argjson pr_meta "$PR_JSON" \
  --argjson review_count "$REVIEW_COUNT" \
  --argjson issue_comment_count "$ISSUE_COMMENT_COUNT" \
  --argjson inline_count "$INLINE_COUNT" \
  --arg review_body_api_length "$(jq -r 'sort_by(.body | length) | reverse | .[0].body // "" | length' "$OUT_DIR/coderabbit-reviews.json")" \
  --arg summary_body_api_length "$(jq -r 'sort_by(.body | length) | reverse | .[0].body // "" | length' "$OUT_DIR/coderabbit-issue-comments.json")" \
  '{
    pr_number: ($pr_number | tonumber),
    repo: $repo,
    fetched_at: $fetched_at,
    pr: $pr_meta,
    coderabbit: {
      review_count: $review_count,
      review_id: (if $review_id == "" then null else ($review_id | tonumber) end),
      review_body_api_length: ($review_body_api_length | tonumber),
      issue_comment_count: $issue_comment_count,
      summary_comment_id: (if $summary_comment_id == "" then null else ($summary_comment_id | tonumber) end),
      summary_body_api_length: ($summary_body_api_length | tonumber),
      inline_count: $inline_count
    }
  }' >"$OUT_DIR/meta.json"

# Length verification (file bytes vs API-reported body length)
verify_length() {
  local label="$1"
  local file="$2"
  local api_len="$3"
  local file_len
  file_len="$(wc -c <"$file" | tr -d ' ')"
  if [[ "$api_len" -eq 0 && "$file_len" -eq 0 ]]; then
    echo "  ${label}: (empty)"
    return 0
  fi
  if [[ "$file_len" -lt "$api_len" ]]; then
    echo "  ${label}: MISMATCH file=${file_len} bytes, API=${api_len} — possible truncation!" >&2
    return 1
  fi
  echo "  ${label}: ${file_len} bytes (API body length: ${api_len}) OK"
}

echo
echo "Output: ${OUT_DIR}"
echo "Length verification:"
ERR=0
verify_length "review-body.md" "$CR_DIR/review-body.md" \
  "$(jq -r '.coderabbit.review_body_api_length' "$OUT_DIR/meta.json")" || ERR=1
verify_length "summary-comment.md" "$CR_DIR/summary-comment.md" \
  "$(jq -r '.coderabbit.summary_body_api_length' "$OUT_DIR/meta.json")" || ERR=1

INLINE_IDX=0
for f in "$INLINE_DIR"/*.md; do
  [[ -e "$f" ]] || continue
  cid="$(basename "$f" .md)"
  api_len="$(jq -r --arg id "$cid" '.[] | select(.id == ($id | tonumber)) | .body | length' \
    "$OUT_DIR/coderabbit-review-comments.json")"
  verify_length "inline/${cid}.md" "$f" "$api_len" || ERR=1
  INLINE_IDX=$((INLINE_IDX + 1))
done

if [[ "$INLINE_IDX" -eq 0 ]]; then
  echo "  inline: (none)"
fi

echo
echo "Summary:"
jq -r '"  PR: \(.pr.title)\n  Branch: \(.pr.headRefName) -> \(.pr.baseRefName)\n  Head: \(.pr.headRefOid[0:7])\n  CodeRabbit reviews: \(.coderabbit.review_count)\n  Summary comments: \(.coderabbit.issue_comment_count)\n  Inline comments: \(.coderabbit.inline_count)"' \
  "$OUT_DIR/meta.json"

if [[ "$ERR" -ne 0 ]]; then
  echo "error: length verification failed — bodies may be truncated" >&2
  exit 1
fi

echo "Done."
