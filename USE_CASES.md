# Pane Use Cases

Pane is shared local memory for agent work. The features matter because they support concrete workflows that are currently awkward, fragile, or human-mediated.

## 1. Agent restart continuity

An agent exits, crashes, reaches a context limit, or is replaced by a different model.

Pane should help the next agent answer:

- what was this workspace doing recently?
- what session am I continuing from?
- what intent, files, messages, and branch context did I inherit?

The goal is that a restarted agent does not begin cold and does not require the human to rewrite the handoff every time.

## 2. Cross-pane handoff

Work moves from one terminal pane to another, or from one agent to another.

Pane should make that handoff explicit:

```bash
pane continue <session-id>
pane summary
```

The new session can say, "I am continuing that work," and Pane can preserve the lineage.

## 3. Concurrent agent awareness

Several agents work in the same repo at once.

Pane should let each agent see:

- who else is active
- what each session is trying to do
- which files were touched recently
- whether there are unread questions or pending replies
- whether a risky operation may collide with another session

The human should not have to act as the live coordination board.

For this to feel trustworthy, Pane also needs board freshness: old sessions should not clutter the active view just because they are durably remembered. First-pass lifecycle cleanup uses `pane close`, `pane sessions prune`, and stale-session hiding so history remains durable while the active board stays useful.

## 4. Human handoff relief

The human often becomes the memory layer: summarizing prior work, warning agents about each other, and relaying messages.

Pane should reduce that burden by giving agents commands they can run themselves:

```bash
pane summary
pane board
pane history --since 24h
pane inbox
```

The human can still inspect and override, but should not be required to keep shared state current.

## 5. Workspace memory over terminal scrollback

Important context is often trapped in terminal history, chat logs, or the human's memory.

Pane should keep the useful parts in a local, queryable workspace memory: sessions, intents, recent activity, messages, git events, and lineage.

The goal is not a perfect project history. The goal is enough durable context for agents to resume and coordinate safely.

## 6. Safer high-risk operations

Some commands are risky when multiple agents are working: rebases, resets, force pushes, branch changes, and merges.

Pane should use shared workspace awareness as a guardrail before those operations happen.

Git is not the product center; it is one important moment where shared memory can prevent avoidable damage.

## 7. Specialized agent memory

A specialized agent may need to remember compact local facts: service names, environment notes, investigation status, handoff data, or tool-specific state.

Pane should provide a simple namespaced JSON store:

```bash
pane state set agent.notes '{"handoff":"tests need review"}'
pane state get agent.notes
pane state list agent.
```

The goal is not to replace source files or project documentation. The goal is to give agents a shared local memory slot for facts that should survive one session without inventing a new cache per tool.

## 8. Provider-agnostic agent collaboration

Different coding agents can run in different terminals with different providers.

Pane should work below all of them at the shell/workspace layer. If an agent can run commands, it can participate.

The long-term vision is a local environment layer where agents coordinate through shared state rather than provider-specific integrations.
