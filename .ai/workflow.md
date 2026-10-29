# Workflow loop (runs until the project is done)

```
┌──────────────────────────────────────────────────────────────────┐
│ 1. Business Analyst  → writes/updates REQ-xxx in .ai/requirements │
│ 2. Project Manager   → breaks REQ into tasks in .ai/sprints        │
│ 3. Team Lead         → assigns tasks (backend / frontend / ui-ux)  │
│ 4. Engineers         → build; run gen/test; log every error        │
│ 5. QA Tester         → test plan + results; opens bugs as tasks    │
│ 6. Security Expert   → threat check on the diff; blocks if needed  │
│ 7. Orchestrator      → reviews, merges, updates LESSONS/ADR, loops │
└──────────────────────────────────────────────────────────────────┘
```

## How a session runs
1. Orchestrator reads memory, picks the top open sprint task.
2. Spawns the role agents it needs (see `.claude/agents/`). Each agent reads `.ai/` first.
3. Engineers work ONLY inside their module folders unless the task says otherwise.
4. Definition of done for a task:
   - code compiles (`go build ./...`, `pnpm typecheck`), tests pass,
   - GraphQL schema + generated code in sync (`make gen` clean),
   - QA checklist ticked, security checklist ticked,
   - lesson logged if any error happened,
   - sprint board updated.
5. Orchestrator marks task done, writes a 3-line sprint note, picks next task.

## Error → lesson protocol
When anything fails:
```
scripts/ai/log-lesson.sh "<area>" "<what broke>" "<root cause>" "<fix>" "<prevention rule>"
```
Agents must grep LESSONS.md for the area before starting work in that area.

## Skill growth protocol
After finishing a task, the engineer appends to their role file under "Learned patterns" anything that would make the next similar task faster or safer. Keep each entry ≤ 3 lines.
