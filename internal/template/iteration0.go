package template

// Iteration0Template is the prompt template for Iteration #0 (planning phase).
// This runs once before the main iteration loop to load all tasks from the spec.
// The agent has a restricted tool set: only task-add, task-list, and iteration-summary.
const Iteration0Template = `# iteratr Session — Iteration #0 (Planning)
Session: {{session}}

## Spec
{{spec}}

{{tasks}}

## Your Job
Read the spec above and load ALL tasks into the system using the task-add tool.

### Rules
- Add EVERY task from the spec — do not skip any
- Preserve the order from the spec
- Set priorities: 0=critical, 1=high, 2=medium, 3=low, 4=backlog
- Group related tasks in a single task-add call for efficiency
- Do NOT start implementing anything — planning only
- When done, call iteration-summary with a count of tasks added
- If tasks already exist, verify completeness against spec and add any missing ones

### Priority Guide
- Foundation/setup tasks: priority 0-1
- Core feature tasks: priority 1-2
- Polish/cleanup tasks: priority 3-4
{{extra}}`
