# GTD Agent Workflow & CLI Reference Guide

## Part 1: GTD Methodology Mapping & Core CLI Concepts

| GTD Phase | Concept Description | CLI Mapping |
| :--- | :--- | :--- |
| **1. Capture** | Frictionless collection of open loops into a trusted inbox. | `gtd task add "<text>"` automatically starts in `inbox` status. |
| **2. Clarify** | Deciding what each loop means and what concrete actions are needed. | Process `gtd inbox`. Apply 2-min rule, assign projects, or set statuses. |
| **3. Organize** | Sorting tasks by context, project, area, and urgency constraints. | Update status (`next`, `waiting`, `someday`), contexts (`@computer`), tags (`#urgent`). |
| **4. Reflect** | Reviewing the system weekly to keep it clean, accurate, and current. | `gtd stalled` identifies active projects missing active `next` action steps. |
| **5. Engage** | Choosing the best action to perform right now under current constraints. | `gtd agenda` displays overdue / due-today actions; filter by context. |

### Core Task Statuses
* `inbox`: Raw, unprocessed loop.
* `next`: Immediately actionable task.
* `waiting`: Delegated task; requires an assignee (`assignedTo`) to track follow-up.
* `someday`: Deferred idea/task; reviewed periodically but hidden from daily agenda.
* `reference`: Informational note; date constraints are cleared but checklists/notes remain.
* `done`: Completed task.
* `archived`: Finished historical tasks/projects.

---

## Part 2: Quick-Add NLP Parser Specification
The parser scans the task title text for specific prefix tokens:
1. **Projects (`+`)**: Binds the task to a project (e.g. `+Renovate Home`). Matches existing projects case-insensitively using a greedy matcher.
2. **Areas (`!`)**: Binds the task to an Area of Focus (e.g. `!Finances`). Only applied if no project is assigned.
3. **Contexts (`@`)**: Binds situational contexts (e.g. `@computer`, `@phone`).
4. **Tags (`#`)**: General categorization tags (e.g. `#urgent`, `#groceries`).
5. **Delegates (`%`)**: Assigns a delegate (e.g. `%Alice`).
6. **Date Commands (`/`)**:
   - `/due:<date>`: Sets due date. Supports `today`, `tomorrow`, `monday` (relative day) or exact `YYYY-MM-DD`.
   - `/start:<date>`: Sets start date.
   - `/review:<date>`: Sets next review date.
7. **Status Commands (`/`)**:
   - `/next`, `/someday`, `/waiting`, `/reference`: Sets the initial task status.

*Example Input String:*
`"Email backup proposal %Bob @computer +Work Migration /due:tomorrow"`

---

## Part 3: Step-by-Step AI Agent SOPs

### SOP 1: Clarify & Process Inbox (Daily/On-Demand)
1. Query `gtd inbox` to gather unprocessed items.
2. For each task returned:
   - If the task is actionable:
     - **2-Minute Rule**: If the task takes <2 minutes, immediately prompt the user to execute it right now and record it as completed.
     - **Delegate**: If someone else should do it, update status to `waiting` and assign a delegate:
       `gtd task update <id> "%Bob /waiting"` (If Bob does not exist, add him first via `gtd people add Bob`).
     - **Projects**: If it requires >1 step, bind it to an active project using `+ProjectName`.
       - *Fallback*: If the project does not exist, create the project first: `gtd project add "Project Name"`, then update the task.
     - **Next Action**: If it's a standalone immediate action, flag it: `gtd task update <id> "/next"`.
   - If non-actionable:
     - **Someday/Maybe**: Move to `someday`.
     - **Reference**: Move to `reference`.
     - **Delete**: Soft-delete useless tasks via `gtd task delete <id>`.

### SOP 2: Reflect & Weekly Review
Perform the weekly review to align focus, ensure data integrity, and unblock projects:
1. Run `gtd index rebuild` to ensure the index database is fully consistent with the disk files.
2. Run `gtd stalled` to fetch all active projects lacking concrete next actions.
   - For each stalled project, fetch all candidate tasks belonging to it.
   - Present these candidates to the user and prompt them to promote one to `/next` (e.g. `gtd task update <id> "/next"`) or add a new action step.
3. Run `gtd task list waiting` to review pending delegations. Prompt the user for follow-ups if items have been sitting for too long.
4. Run `gtd project list` and show active projects to the user, ensuring they are still relevant and not completed.

### SOP 3: Engage (Agenda Execution)
1. Run `gtd agenda` to retrieve all overdue and due-today tasks.
2. Present the agenda to the user as their primary checklist.
3. Allow the user to filter tasks by their current physical constraints (e.g., query tasks with context tag `@computer` or `@errand`) to matching current environments.
