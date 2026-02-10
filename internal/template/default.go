package template

// DefaultTemplate is the embedded default prompt template.
// It uses {{variable}} placeholders for dynamic content injection.
const DefaultTemplate = `# iteratr Session
Session: {{session}} | Iteration: #{{iteration}}

{{history}}

## Spec
{{spec}}

{{tasks}}

{{notes}}

## Rules
- ONE task per iteration - complete fully, then STOP
- Test changes before marking complete
- Write iteration-summary before stopping
- Call session-complete only when ALL tasks done
- Respect user-added tasks even if not in spec

## Workflow
1. Pick ONE ready task (highest priority, no blockers) using task-next tool
2. Mark task as in_progress using task-update tool
3. Implement + test
4. Mark task as completed using task-update tool
5. Write iteration-summary using iteration-summary tool
6. STOP (do not pick another task)

## If Stuck
- Add a note using note-add tool with type "stuck" describing the issue
- Mark task blocked or fix before completing
- If blocked by another task: use task-update tool to set depends_on

## Subagents
Spin up subagents (via Task tool) to parallelize work. Each subagent has fresh context, so "one task per agent" is preserved.

**DO parallelize when:**
- Tasks are independent (no shared files)
- Tasks have no uncommitted dependencies between them
- Read-only research while you implement

**DO NOT parallelize when:**
- Tasks modify the same files (causes conflicts)
- Task B depends on Task A's uncommitted changes
- Uncertain about conflicts - err sequential

Mark all delegated tasks in_progress, then completed when subagents finish.
{{extra}}`
