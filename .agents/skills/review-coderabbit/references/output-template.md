# CodeRabbit evaluation report template

Write the report to `.agents/project/pr-reviews/pr-{N}-coderabbit-evaluation.md`.

Replace placeholders; reproduce **full** CodeRabbit bodies from fetch output — do not summarize away actionable detail.

---

```markdown
# PR #{N} — CodeRabbit review evaluation (complete)
**PR:** [{title}]({pr_url})
**Branch:** `{head}` → `{base}`
**Reviewed head:** `{head_sha}`
**Evaluation date:** {YYYY-MM-DD}
**Status:** Investigation only — no code changes applied yet.

This document reproduces **all** CodeRabbit findings from PR #{N}, including the full review body and inline comments. Each finding includes our verification verdict.

---

## Source links

| Source | URL |
| --- | --- |
| PR | {pr_url} |
| Review #{review_id} | {pr_url}#pullrequestreview-{review_id} |
| Inline comment ({path_hint}) | {pr_url}#discussion_r{id} |
| CodeRabbit summary comment | {pr_url}#issuecomment-{summary_id} |

## Finding inventory ({total} CodeRabbit findings)

| # | Severity | Location | cr-comment ID | Verdict |
| --- | --- | --- | --- | --- |
| C1 | Critical (inline) | `path/file.go` L97–127 | `{hash}` | VALID — Must fix |
| M1 | Major | `path/file.go-131-136` | `{hash}` | VALID — Should fix |

## Product decisions (optional)

Use when a valid CodeRabbit concern is overridden by product/spec choice:

### Product decision — {topic} ({finding_ref})

**Decision ({date}):** {what we decided}

**Why:**

- {bullet}

**Action before merge:** {concrete step or "none"}

---

## CodeRabbit summary comment (full)

**Author:** coderabbitai[bot]
**URL:** {summary_comment_url}

{paste full body from coderabbit/summary-comment.md}

### Our evaluation — summary comment

- **Merge risk:** {Agree/Disagree} — {one line}
- **Pre-merge checks:** {bullet per failed check we care about}
- **Walkthrough:** {optional note if inaccurate}

---

## Inline Critical comments (full)

**Review:** {review_url}

### Inline on `{path}` (lines {start}-{end})

- **Discussion:** {discussion_url}
- **Comment ID:** `{id}`
- **Review ID:** `{review_id}`

{paste full body from coderabbit/inline/{id}.md — omit the YAML frontmatter added by fetch script}

**Our evaluation (`{cr-id}`):** {VERDICT} — **{Priority}**. {rationale}

---

## Review #{review_id} body (full Major comments)

**URL:** {review_url}

> CodeRabbit note: "Actionable comments posted: {inline_count}" — Critical items were posted inline; {major_count} Major items below.

### M1 — {location}

{paste full finding section from review-body.md}

**Our evaluation (`{cr-id}`):** {VERDICT} — **{Priority}**. {rationale}

---

(repeat for each Major finding M2…Mn)

---

## Executive summary (prioritized actions)

### Must fix before merge

- `{cr-id}`: {one-line action}

### Should fix before merge

- `{cr-id}`: {one-line action}

### Fix soon

- `{cr-id}`: {one-line action}

### Resolved / product decisions

- `{cr-id}` ({ref}): {decision}

### Skip / disagree

- `{cr-id}` ({ref}): {reason}

---

*Generated from GitHub API fetch of PR #{N} reviews and comments on {date}.*
```

## Finding extraction hints

From `coderabbit/review-body.md`:

- Fingerprint: `<!-- cr-comment:v1:{hash} -->`
- Section headers like `### M1 — path/to/file.go-131-136`
- Severity markers: `🔴 Critical`, `🟠 Major`

From `coderabbit/inline/*.md`:

- One finding per file; YAML frontmatter has `comment_id`, `path`, `line`

From `coderabbit/summary-comment.md`:

- Walkthrough, pre-merge checks, merge risk — evaluate holistically; do not duplicate as separate M-items unless they map to distinct code issues

## Chat reply (after writing the report)

Post a compact table sorted by priority (Must fix first):

| Severity | Location | Verdict | cr-id |
| --- | --- | --- | --- |

Then: counts (`X Must fix, Y Should fix, Z Skip`) and link to the report file.

Do **not** apply fixes unless the user asks in a follow-up.
