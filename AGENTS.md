# Agent guide — FactoryMate

This repo is built **milestone-by-milestone** from [`docs/factorymate-roadmap.md`](docs/factorymate-roadmap.md). The product contract lives in [`docs/factorymate-spec.md`](docs/factorymate-spec.md).

## Before you implement

1. Read the milestone section in the roadmap (tasks + DoD).
2. Read spec sections linked in [`.agents/project/orchestrator/doc-index.md`](.agents/project/orchestrator/doc-index.md).
3. Check [`.agents/project/orchestrator/milestone-scopes.md`](.agents/project/orchestrator/milestone-scopes.md) for READ/WRITE scope and scoped CI commands.
4. For frontend UI: follow [`.cursor/rules/shadcn.mdc`](.cursor/rules/shadcn.mdc) (shadcn MCP) and [`.cursor/rules/03-i18n.mdc`](.cursor/rules/03-i18n.mdc) (no hardcoded strings).

## CodeRabbit PR reviews

When the user runs `/review-coderabbit <PR#>` or asks to evaluate CodeRabbit findings:

- Follow [`.agents/skills/review-coderabbit/SKILL.md`](.agents/skills/review-coderabbit/SKILL.md)
- Reports go to `.agents/project/pr-reviews/pr-{N}-coderabbit-evaluation.md`

## Orchestrated runs

When the user asks to orchestrate or run the roadmap autonomously:

- Follow [`.agents/skills/orchestrator/SKILL.md`](.agents/skills/orchestrator/SKILL.md)
- Use prompt templates in [`.agents/project/orchestrator/prompt-templates.md`](.agents/project/orchestrator/prompt-templates.md)
- **Execution agents** implement within WRITE SCOPE; do **not** edit roadmap checkboxes
- **Verifier agents** run three-layer checks; on PASS mark **all task checkboxes** under that milestone `- [x]`; on FAIL mark them `- [!] failed — <reason>`
- Autonomous loop ends at **M13**; M14 is manual backlog only

## Commits

Conventional commits with milestone ref: `feat(M3): implement fast-poll diff engine`

Do not push unless the user explicitly asks.

## Testing without production services

- **Discord webhooks:** use Go `httptest` mock server or optional `fauxcord` container — see [`docs/testing.md`](docs/testing.md)
- **FRM:** unit tests use JSON fixtures in `backend/testdata/frm/`; **live read-only server** at `http://192.168.178.42:8889` — see [`.cursor/rules/04-frm-live.mdc`](.cursor/rules/04-frm-live.mdc)
- **Do not** require a real Discord channel or production deploy for CI to pass

## Key paths

| What | Where |
| --- | --- |
| Message defaults seed | `backend/data/message_defaults.json` |
| FRM API reference | `docs/frm-docs/` |
| Frontend strings | `frontend/messages/en.json` |
| shadcn components | `frontend/components/ui/` |
| UI route → component map | spec §8.1 |
