# Sub-agent monitoring

**Applies to any agent that dispatches sub-agents** via Task (`run_in_background: true` or not).

## The failure mode

Parent infers **stalled** from git silence, takes over the sub-agent's WRITE scope, and duplicates work.

**Git history alone is not liveness.** Sub-agents may produce no commits for long stretches while still working.

## Liveness signals (use together)

| Signal | What to check |
| ------ | ------------- |
| **Sub-agent transcript** | New tool calls, assistant turns |
| **Task / Await** | Background notification or changing output |
| **Git** | New commits when WRITE scope commits |
| **Session memory** | `active/<SESSION-ID>.md` mtime (if used) |

Transcript is the **primary** signal for explore/execution agents.

## Stall detection (two-sample — mandatory)

**Never** declare stalled from a single check.

1. **Appears stalled** — no notification for unusually long time, or user asks status
2. **First transcript read** — note last activity
3. **Wait 10–20 seconds**
4. **Second transcript read** — progress = new lines/tool calls
5. **No progress** → terminate old agent → audit partial work → dedupe → spawn replacement

**Forbidden:** two agents on the same milestone concurrently.

## Parent: do not take over

While a sub-agent runs for a scoped milestone:

- Do not implement in its WRITE scope
- Do not mark roadmap `[x]` until verifier PASS
- Follow two-sample rule before declaring stall

```text
appears stalled?
  → read transcript (sample 1)
  → wait 10–20s
  → read transcript (sample 2)
  → progress? → keep waiting
  → no progress? → terminate → audit → dedupe → re-dispatch
```
