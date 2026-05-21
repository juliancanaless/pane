# Reframing Pane: From Coordination Tool to Environment-as-Product

> Date: 2026-05-18
> Context: Inspired by OpenAI's Codex harness engineering principles and the realization that context loss across ~20 agent sessions/day is the #1 bottleneck. This document reframes Pane from a multi-agent coordination utility into the unified persistence and awareness layer for all agent workflows.

---

## The Pattern You've Built Three Times

There are three layers in the current workflow where agents lose context, and each one has been solved independently with a separate architecture:

### Layer 1: Neon/APM Agent → User Conversations
The Neon agent loses context between turns. Schema gets re-fetched, IQL queries get re-tried, graph walks get re-explored.

**Solution built:** Session-state infrastructure (Passes 1-3 on `feat/agent-session-state-registry`). Schema caching, IQL history, graph-walk cache, hallucination guard, per-field freshness metadata. Bespoke implementation in `neon_session_renderer.py`.

### Layer 2: Pi/Claude Sessions → Developer Workflow
Pi and Claude sessions lose context between handoffs. Every new session starts cold — re-reads files, re-discovers architecture, re-learns decisions.

**Solution built:** `/handoff` extension (one-shot LLM extraction), `APPEND_SYSTEM.md` (static rules), skills (on-demand workflow instructions), Obsidian vault (manual knowledge base). Each is a manual approximation of persistent memory.

### Layer 3: Concurrent Agent Sessions → Same Workspace
Multiple agents working in the same codebase don't know about each other. Humans relay context between panes.

**Solution built (in progress):** Pane v1 — shared awareness board, session identity, intent tracking, messaging, git preflight.

**These are the same problem at different scopes:** an agent needs to know what happened before it arrived and what's happening around it.

---

## The Reframe

> **This is not a pivot.** V1's scope (concurrent coordination) is unchanged and remains the right foundation. This reframe is about recognizing that the data Pane is already collecting in v1 — session identity, intents, file activity, messages — naturally extends to solve the sequential context and agent memory problems too. It's a widening of the lens, not a change in direction.

Pane shouldn't just be for concurrent agents. It should be the **unified persistence and awareness layer** for all three scopes:

- **Sequential continuity** — a new session inherits the full lineage of what previous sessions did in this workspace, on this task, in this pane. No handoff extraction needed; the context was recorded as it happened.
- **Concurrent coordination** — the existing v1 vision. Sessions see each other's intent, working sets, and messages.
- **Agent memory** — the Neon/APM agent's session-state infrastructure becomes a Pane client, not a bespoke implementation. Same protocol, same storage.

### The OpenAI Lesson

The OpenAI Codex team's 5 harness engineering principles (from their 1M-line codebase built entirely by agents) boil down to one meta-principle:

> **The environment is the product. The code is a side effect.**

They treated the harness — the shell around the agent — as the primary deliverable. ExecPlans, mechanical linters, observability stacks, Chrome DevTools integration — all of it was environment design, not agent prompting.

Pane is already this idea for concurrent coordination. The reframe extends it: **Pane is the environment for ALL agent work.** It's not a tool in your workflow — it IS your workflow.

---

## Three Concrete Moves

### 1. Sequential Session Lineage

**Problem:** `/handoff` does a one-shot LLM extraction at session end, compressing an entire conversation into ~50 lines. Information is lost. The next session starts with a lossy summary, not real history.

**Solution:** Pane tracks session chains. A session can declare continuity:

```bash
pane continue <previous-session-id>
# or inferred automatically from same pane/TTY
```

Throughout a session's lifetime, Pane is already recording intent changes, file touches, messages, and board updates. When a new session starts in the same pane, `pane summary` doesn't just show "who's active now" — it shows the full lineage:

- What the last N sessions in this pane did (from their intents and activity)
- What decisions were made (from explicit `pane decide` or message threads)
- What's still open (from unresolved questions and incomplete intents)
- Which files matter (from file activity tracking, not LLM guessing)

**This kills the handoff problem mechanically.** No LLM extraction. No lossy compression. The context was recorded incrementally as it happened — structured, queryable, and complete.

The `pane summary` command becomes the handoff. The difference: handoff is reconstructive (tries to recover context after the fact), while Pane is accumulative (records context as it happens).

**Implications for Claude sessions:** Even without a native extension, a Claude session can participate by running `pane init`, `pane intent`, and `pane summary` as shell commands. The human doesn't need to manually write a handoff prompt — they tell Claude to run `pane summary` and it gets the full context from the previous session's recorded activity.

### 2. Activity Log as Work Tracking Source of Truth

**Problem:** Work tracking is currently a three-step manual process:
1. Write to-dos in Obsidian during the week
2. Run `commit-tracker` scan at week's end to find what git says happened
3. Reconcile the two sources (5 buckets) to produce an accurate archive + weekly update

This works, but it's reconstructive — it tries to piece together what happened after the fact, from two incomplete signals (to-dos = intent, commits = output).

**Solution:** Pane is already recording the missing middle layer: **what the agent was actually doing, in real time.**

If Pane tracks:
- Session start/end times (duration)
- Intent changes over time (task narrative)
- File touches (working set)
- Messages between sessions (coordination overhead)
- Git operations (commits, branch switches)

Then the weekly wrap-up skill should query Pane, not git:

```bash
pane history --since 2026-05-18 --format work-log
```

Output:
```
Session abc (Pi, 2h13m): "rewire APM performance ranking over HTTP"
  Files: apm_tools.py (+918), apm_api_client.py (+112), probe_apm.py (+171)
  Git: 3 commits on tenant-registry, PR #86 merged
  
Session def (Claude, 45m): "fix Langfuse duplicate generations"
  Files: chat_litellm.py (+194), litellm_service.py (+85)
  Git: 1 commit, PR #87 merged

Session ghi (Pi, 30m): "calls with Shalom - onboarding"
  No files, no git. Intent-only session.
```

**Commits tell you what code changed. Pane tells you why and how long it took.**

The weekly-wrap-up skill becomes trivial: query Pane's history, group by theme, generate the report. No reconciliation needed — there's one source of truth, not two.

The Obsidian to-dos become what they should be: a lightweight intent layer ("things I want to do"), not a work-tracking system ("things I did"). Pane handles the latter automatically.

### 3. Unified Agent Memory Protocol

**Problem:** The Neon agent has its own bespoke session-state system:
- `NeonSessionRenderer` in `neon_session_renderer.py`
- Per-tool `save_agent_session_state` calls
- Schema cache, IQL history, graph-walk cache stored in Django's session model
- Full-slice serializer with per-field freshness metadata

This works, but it's a custom persistence layer that only the Neon agent can use. If a new agent type needs session state (and they will — the APM agent, future agents), each one needs its own renderer, its own serializer, its own cache invalidation logic.

**Solution:** Pane exposes a generic key-value state API scoped by session and namespace:

```bash
pane state set neon.schemas '{"equipment": {...}}'
pane state get neon.schemas
pane state set neon.iql_history '[{"query": "...", "success": true}]'
pane state list neon.*
```

The Neon agent's session-state infrastructure becomes a Pane client. Same daemon, same storage, same protocol. The schema cache is `pane state set neon.schemas`. The IQL history is `pane state set neon.iql_history`. Cache invalidation is `pane state delete neon.graph_cache`.

**Benefits:**
- Any agent type gets persistent memory for free — just pick a namespace
- The human can inspect agent memory: `pane state list neon.*` shows exactly what the agent "remembers"
- Cross-agent memory becomes possible: the APM agent could read `pane state get neon.schemas` if it needs schema info
- One persistence system to maintain, not N per agent type

---

## What This Means for V1 Prioritization

The current v1 scope (session identity, board, messaging, git preflight) is the right foundation. The reframe doesn't change v1 — it changes what v2 and v3 look like:

| Version | Scope | Key Capability |
|---------|-------|---------------|
| **v1** (current) | Concurrent coordination | Board, messaging, git preflight, session identity |
| **v2** | Sequential continuity | Session lineage, `pane continue`, `pane summary` with history, `pane history` for work tracking |
| **v3** | Agent memory | `pane state` API, Neon/APM agent integration, cross-agent memory |

The v1→v2 bridge is small: session lineage is mostly "keep recording what you're already recording, but make it queryable across session boundaries." The data model already has sessions, intents, and timestamps.

---

## The End State

One local daemon that gives ALL agents — Pi, Claude, Neon, APM, future ones — shared persistent memory:

- **Sequential sessions** inherit context automatically. No handoff prompts, no manual re-orientation.
- **Concurrent sessions** coordinate without human relay. No "hey, are you still touching that file?"
- **Work tracking** falls out as a side effect of the activity log. No manual to-do reconciliation.
- **Agent memory** is unified. No bespoke persistence per agent type.

The human stops being the message bus, the context bus, AND the memory bus.

**Pane isn't a tool for your workflow. Pane IS your workflow.**
