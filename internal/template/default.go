package template

// DefaultTemplate is the embedded default prompt template.
// It uses {{variable}} placeholders for dynamic content injection.
const DefaultTemplate = `## Context
Session: {{session}} | Iteration: #{{iteration}}
Spec: {{spec}}
{{inbox}}{{notes}}
## Task State
{{tasks}}

## Tools

Use Bash to call iteratr tools. All commands require --data-dir flag.

### Task Management
- {{binary}} tool task-add --data-dir .iteratr --name {{session}} --content "description" [--status remaining|in_progress|completed|blocked]
- {{binary}} tool task-status --data-dir .iteratr --name {{session}} --id ID --status STATUS
- {{binary}} tool task-list --data-dir .iteratr --name {{session}}

### Notes
- {{binary}} tool note-add --data-dir .iteratr --name {{session}} --content "text" --type learning|stuck|tip|decision
- {{binary}} tool note-list --data-dir .iteratr --name {{session}} [--type TYPE]

### Inbox
- {{binary}} tool inbox-list --data-dir .iteratr --name {{session}}
- {{binary}} tool inbox-mark-read --data-dir .iteratr --name {{session}} --id ID

### Session Control
- {{binary}} tool session-complete --data-dir .iteratr --name {{session}} - Call when ALL tasks done

## Workflow
1. Check inbox, mark read after processing
2. Ensure all spec tasks exist in task list
3. Pick ONE task, mark in_progress, do work, mark completed
4. Run tests, commit with clear message
5. If stuck/learned something: note_add
6. When ALL done: session_complete("{{session}}")

Rules: ONE task/iteration. Test before commit. Call session_complete to end - do NOT just print a message.
{{extra}}`
