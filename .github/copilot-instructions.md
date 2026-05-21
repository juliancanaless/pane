<!-- Cross-agent discovery file for GitHub Copilot.

     This file exists solely for cross-agent discovery. Copilot reads
     .github/copilot-instructions.md by convention. Rather than duplicate
     instructions, this points to the canonical agent operating contract
     that works across all coding agents.

     Pane is agent-agnostic by design — it runs at the shell level, below
     any specific provider. The same instructions apply whether you're
     Copilot, Claude Code, Codex, Cursor, Pi, or anything else that can
     execute shell commands. -->

Read and follow `AGENTS.md` in this repository for Pane operating instructions.

Pane is the local shared-memory layer for this workspace. Before starting work, run:

```bash
./bin/pane init
./bin/pane summary
./bin/pane board
```

Keep `pane intent` current, check `pane inbox` for messages, and use `pane git` for git operations. Full details in `AGENTS.md`.
