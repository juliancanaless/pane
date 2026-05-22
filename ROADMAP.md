# Pane Roadmap

This is the guided plan from V1-done through project completion.

## Product narrative — why each version matters

The single thread through the entire roadmap: **each version removes a different kind of thinking the human shouldn't have to do.**

### V1 ✅ — The human stops being the message bus

Before V1, you are the shared memory. You copy-paste context between panes, you remember who's touching what, you tell agent 3 what agent 1 figured out. V1 replaces that with a board agents read and write themselves. The human can step back from relaying.

**The value: agents know about each other without you telling them.**

### V2 — The human stops being the collision detector

V1 tells you *what* is happening. But you still have to look at the board and think "wait, those two are both in `auth/` — that's going to be a problem." V2 computes that for you. Overlap detection, file-level preflight warnings, hot directory tracking. The system does the spatial reasoning about who's near who.

**The value: the system warns before damage, not after.** You stop scanning the board with anxiety and start trusting it to flag what matters.

### V3 — The human stops being the code analyst

V2 says "two sessions touched `auth.ts`." That's useful but crude — maybe they touched completely unrelated functions. V3 says "Session 1 changed `validateToken()` return type and Session 2 is calling it with the old signature." The Rust analysis engine gives Pane actual understanding of code structure: imports, symbols, dependency edges. Warnings become precise instead of proximity-based.

**The value: relevance replaces proximity.** False positives drop dramatically. Agents get told exactly what changed and why it matters to them, not just that someone was nearby.

### Done — The human stops being the system administrator

V3 is powerful but you still `make build`, manage the daemon, debug socket conflicts, manually set up shell hooks. Done means `brew install pane && pane setup` and you never think about it again. Worker hierarchies, platform hardening, work tracking as a side effect. Pane becomes invisible infrastructure — like git itself, you forget it's a separate tool.

**The value: Pane disappears into the environment.** It's not a tool in your workflow, it *is* your workflow.

> **Relaying → Detecting → Analyzing → Administering.** By Done, the human's only job is deciding what to build.

---

V1 is the 80/20 foundation: sessions, board, messaging, git guardrails, file activity, shell integration, continuity, agent state, and daemon lifecycle. It was dogfooded and marked done on 2026-05-21.

What remains is three scopes of increasing depth, each building on what exists.

---

## Version summary

| Version | Theme | Key question it answers |
|---------|-------|------------------------|
| **V1** ✅ | Reliable awareness | "Who is doing what right now?" |
| **V2** | Deep coordination | "Where is my work colliding with yours?" |
| **V3** | Semantic intelligence | "What symbols changed and who depends on them?" |
| **Done** | Production-grade infrastructure | "Is this dependable enough to forget it's there?" |

---

## V2 — Deep coordination

V2 takes the V1 surface from "useful first pass" to "trustworthy daily infrastructure." The data is already being collected; V2 makes it actionable.

### V2.1 — Working-set overlap detection

**Why:** V1 tracks file activity per session but never computes overlap. Agents can't see when they're about to collide.

**What:**
- Derive overlap from recent `file_activity` by session (shared files, shared hot directories)
- Add compact overlap indicators to `pane board`
- Include current-session overlap warnings in `pane summary`
- Feed overlap into git preflight warnings for risky operations
- Tune noise thresholds through dogfooding

**Exit criteria:** Two agents touching the same file area see a clear warning before either one commits or rebases.

### V2.2 — Richer git preflight

**Why:** V1 preflight checks branch-level overlap. Real collisions happen at the file level.

**What:**
- Preflight checks file activity overlap, not just branch overlap
- Confirmation prompt instead of hard block for forceful operations (`push --force`, `reset --hard`)
- More complete target-branch parsing and remote/push semantics
- Surface relevant overlap context in the warning message

**Exit criteria:** `pane git rebase main` shows which specific files from other sessions would be affected.

### V2.3 — Board and summary signal quality

**Why:** V1 board/summary output is functional but not tuned. After daily use, some data is noise and some useful signals are missing.

**What:**
- Hot directory derivation (directories with 2+ recently active files)
- Recent guardrail events in board
- Richer history filtering and output shape
- Decide whether summary should show full unread message bodies or previews
- Refine open-thread semantics after dogfooding
- `pane board --all` or `pane sessions list` for explicit lifecycle inspection
- Clearer stale/closed labels in history output

**Exit criteria:** Board and summary feel like the right density — agents don't ignore them but don't get flooded.

### V2.4 — Daemon hardening

**Why:** V1 daemon works but has known gaps in edge cases.

**What:**
- Process locking so two daemons don't fight over the same DB/socket
- Stronger stale PID/process detection
- Actual log redirection and rotation
- Safer startup around stale sockets
- `pane daemon start --background` or auto-start policy beyond shell hook
- Auto-start behavior outside shell hook
- Consider whether long-running agent commands need activity signals beyond prompt heartbeat

**Exit criteria:** A user never has to think about whether the daemon is running or debug socket/PID conflicts.

### V2.5 — Targeting and naming ergonomics

**Why:** Short IDs work but are still opaque. Agents and humans want meaningful names.

**What:**
- Richer aliases or human-friendly session names
- Dogfood whether board stays readable with current short-ID indicators
- Consider `pane name "auth-refactor"` for explicit session naming

**Exit criteria:** `pane ask auth-refactor "are you done?"` works and reads naturally.

### V2.6 — File watcher hardening

**Why:** V1 uses polling. It works but isn't efficient or precise.

**What:**
- Replace or augment polling with platform-native watcher (FSEvents/inotify via `rjeczalik/notify`)
- Respect `.gitignore` and `.paneignore`
- Tune attribution confidence during dogfooding
- Derive hot directories from watcher events

**Exit criteria:** File activity updates are near-instant and don't include noise from build artifacts or dependencies.

### V2 dogfood checklist

Run on real default daemon/DB with multiple panes doing real work:

1. Two agents editing files in the same directory — board shows overlap indicator.
2. `pane git rebase` with overlapping file activity — preflight shows specific file warnings.
3. `pane board` during a 3-pane session — output feels useful, not noisy.
4. Daemon survives a restart without socket/PID confusion.
5. Short IDs or names feel natural in `pane ask` targeting.
6. File activity appears within 1-2 seconds of a file save.

### V2 dogfood status

Implementation is complete. Real dogfood found an overlap gap where a shared test file was attributed to one session only, so board/summary and git preflight did not show file-overlap warnings.

That attribution bug has been fixed: ambiguous same-cwd watcher events are now recorded for all equally likely sessions, while more specific cwd matches still win. Temp-daemon smoke tests and the original real default daemon/DB dogfood now show board overlap and git preflight overlap warnings for `tmp-pane-dogfood/overlap.txt`.

V2 dogfood is complete. V3 can proceed.

---

## V3 — Semantic intelligence

V3 introduces the Rust analysis layer from the original vision. This is where Pane moves from file-level awareness to symbol-level understanding.

### V3.1 — Rust analysis engine scaffold

**Why:** Symbol-level analysis (tree-sitter parsing, semantic diffing, dependency graphs) is CPU-bound work that benefits from Rust's performance and safety guarantees.

**What:**
- Create a Rust workspace (`analysis/`) alongside the Go daemon
- Tree-sitter parser integration for primary languages (TypeScript, Go, Python, Rust)
- FFI bridge or local pipe protocol between Go daemon and Rust analysis engine
- Build system integration (`make build` compiles both Go and Rust)

**Decision:** FFI (CGo/shared library) vs. local pipe (separate process). Pipe is cleaner for independent development and restart; FFI is lower latency. Decide based on V3.2 latency requirements.

**Exit criteria:** Rust analysis engine can parse a file and return its symbol table to the Go daemon.

### V3.2 — Dependency graph construction

**Why:** Knowing that `auth.ts` imports `validateToken` from `crypto.ts` lets Pane warn when a signature change affects downstream files.

**What:**
- Build file-level and symbol-level directed dependency graph from import/require/use statements
- Store graph edges in SQLite with confidence scores
- Incremental updates: re-parse only changed files
- Co-change heuristic layer: analyze git history for files frequently modified together

**Exit criteria:** `pane` can answer "which files depend on this symbol?" for the primary project language.

### V3.3 — Semantic overlap and preflight

**Why:** File-level overlap is crude. "Two sessions touched `auth.ts`" is less useful than "Session 1 changed `validateToken()` return type and Session 2 is calling it with the old signature."

**What:**
- Semantic diff: when a file changes, identify which symbols changed (not just which lines)
- Symbol-level overlap: flag when one session modifies a symbol that another session's working set depends on
- Feed semantic overlap into git preflight and board
- Determine relevance threshold: not every symbol change matters to every session

**Exit criteria:** `pane board` shows "Session 1 changed `validateToken()` signature — Session 2's working set depends on it" instead of just "overlap in auth.ts".

### V3.4 — Temporal decay and summarization

**Why:** As sessions run longer, raw event history becomes noise. The vision doc describes events decaying from full detail to summary to forgotten.

**What:**
- Implement decay tiers: full (< 5 min) → summary (5 min–2 hr) → compressed (2–72 hr) → pruned
- Summaries generated by the analysis engine, not just truncation
- Board and summary adapt output density based on decay tier
- History queries respect decay tiers

**Exit criteria:** A 4-hour session's board is still readable — recent events are detailed, older events are compressed summaries.

### V3 dogfood checklist

1. Change a function signature in one pane — other pane sees which downstream files are affected.
2. Board shows symbol-level overlap, not just file-level.
3. Git preflight warns about semantic conflicts, not just file proximity.
4. After a long session, board output stays useful (decay working).
5. Rust analysis engine latency is < 100ms for incremental file re-parse.

---

## Done — Production-grade infrastructure

"Done" means Pane is infrastructure you forget about. It's always running, always correct, always useful, and never in the way.

### D.1 — Worker/child session hierarchies

**Why:** The original vision describes agents spawning child agents for subtasks. Those child sessions should register with Pane automatically.

**What:**
- `pane spawn` or automatic child session registration
- Parent session sees child session status without polling
- Session hierarchy visible in board and history
- Child sessions inherit parent workspace and intent context

### D.2 — Richer session lineage

**Why:** V1 has immediate parent links. Real workflows have deep chains of sequential sessions.

**What:**
- Accumulate structured decisions and open threads beyond intents/messages
- Handle long lineage chains, not only immediate parent + recent history
- Lineage visualization in `pane history`

### D.3 — Agent state conventions and cross-agent memory

**Why:** V1 `pane state` is raw key-value. Agents need conventions for namespaces, ownership, and cross-agent reading.

**What:**
- Conventions for namespace ownership (e.g., `neon.*`, `apm.*`)
- Decide whether state should be workspace-only, global, or both
- Cross-agent memory: one agent can read another's state
- Surface selected state in summaries when useful
- Richer query/output formats

### D.4 — Installer and distribution

**Why:** Currently requires `make build` and manual PATH setup.

**What:**
- `brew install pane` or equivalent
- `pane setup` one-command installer
- Automatic shell hook installation
- `pane doctor` for diagnosing configuration issues

### D.5 — Activity-based work tracking

**Why:** The reframing doc describes `pane history --format work-log` replacing manual commit scanning for weekly reports.

**What:**
- Rich history output with session durations, file counts, git operation counts
- `pane history --format work-log` for structured work reports
- Integration surface for weekly wrap-up workflows

### D.6 — Platform hardening

**Why:** V1 is macOS-focused. The architecture supports Linux but it's untested.

**What:**
- Linux CI and testing
- inotify-based file watcher validation
- Platform-specific daemon lifecycle behavior

### Done checklist

1. A fresh machine can `brew install pane && pane setup` and be fully operational.
2. Agent spawns a child task — parent sees child session in board automatically.
3. `pane history --since 1w --format work-log` produces a useful weekly report.
4. `pane state list neon.*` shows what the Neon agent remembers.
5. Pane has run for a full work week without manual daemon intervention.
6. Semantic overlap warnings have prevented at least one real collision.

---

## What this roadmap does NOT include

These are from the original vision but are explicitly out of scope for this plan. They represent potential future exploration, not committed work:

- **Full shell interception beyond git** — wrapping `rm`, `mv`, etc.
- **Automatic natural-language reply capture** — extracting answers from agent output
- **Learned workflow models** — predictive coordination from historical patterns
- **UI or dashboard** — Pane stays CLI-first
- **Remote/cloud deployment** — Pane stays local
- **PID-based file attribution** — requires elevated permissions (Endpoint Security / fanotify)

---

## Reading order for contributors

1. `README.md` — what Pane is and current status
2. `ROADMAP.md` — where it's going (this file)
3. `AGENTS.md` — how agents participate
4. `ARCHITECTURE.md` — system design and ownership
5. `PROGRESS.md` — detailed phase history and implementation notes
6. `V1_READY.md` — V1 definition (achieved)
7. `USE_CASES.md` — the problems Pane solves
8. `REFRAMING.md` — long-term vision context
9. `docs/architecture.md` — detailed V1 technical architecture
10. `docs/80-20-overview.md` — V1 product scope rationale

## Source material

The original vision and 80/20 scoping documents that informed this project live at:

```
~/Documents/axiom/10 Personal/13 Effort/Pane/
├── Pane Vision.md        — full technical vision including Go/Rust architecture
└── Pane 80-20 Approach.md — V1 scoping rationale
```

These are preserved as historical context. The canonical plan is this file.
