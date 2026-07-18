# GTD Agent Workflow & MCP/CLI Reference Guide

Agents should use **MCP tools** (`gtd_*`). Humans may use the equivalent CLI. Both hit the same code path.

## Part 1: GTD Methodology Mapping

| GTD Phase | Concept | MCP tool | CLI equivalent |
| :--- | :--- | :--- | :--- |
| **1. Capture** | Frictionless collection into inbox | `gtd_task_add` | `gtd add <words…>` (or `gtd task add`; whole-task quotes optional) |
| **2. Clarify** | Decide meaning and next action | `gtd_get_inbox` then `gtd_task_update` | `gtd inbox` / `gtd task update` |
| **3. Organize** | Context, project, area, status | `gtd_task_update`, entity tools | `gtd task update`, `project`/`area`/`people` |
| **4. Reflect** | Weekly review, unstick projects | `gtd_index_rebuild`, `gtd_get_stalled`, `gtd_task_list` | `index rebuild`, `stalled`, `task list` |
| **5. Engage** | What to do now | `gtd://state`, `gtd_get_agenda`, `gtd_get_next` | `agenda`, `next` |

### Core Task Statuses
* `inbox`: Raw, unprocessed loop.
* `next`: Immediately actionable task.
* `waiting`: Delegated; track with `assigned_to` / `%person`.
* `someday`: Deferred; hidden from daily agenda; review periodically.
* `reference`: Informational note; schedule fields cleared.
* `done`: Completed task.
* `archived`: Finished historical tasks/projects.

---

## Part 2: Quick-Add NLP Parser Specification

Used by `gtd_task_add` and the optional `text` param on `gtd_task_update`:

1. **Projects (`+`)**: Bind to project (e.g. `+Renovate_Home`). Matches existing projects case-insensitively.
2. **Areas (`!`)**: Bind to Area of Focus (e.g. `!Finances`) when no project is set.
3. **Contexts (`@`)**: Situational context (e.g. `@computer`, `@phone`).
4. **Tags (`#`)**: Categories (e.g. `#urgent`).
5. **Delegates (`%`)**: Assignee (e.g. `%Alice`).
6. **Date Commands (`/`)**:
   - `/due:<date>`: `today`, `tomorrow`, weekday, or `YYYY-MM-DD`
   - `/start:<date>`, `/review:<date>`
   - `/recur:…` where supported
7. **Status Commands (`/`)**: `/next`, `/someday`, `/waiting`, `/reference`, `/done`

*Example:* `"Email backup proposal %Bob @computer +Work_Migration /due:tomorrow"`

---

## Part 3: MCP Conventions

### Shared list filters
Optional on `gtd_task_list`, `gtd_get_agenda`, `gtd_get_next` (omit if unused):

| Param | Meaning |
| :--- | :--- |
| `area_id` | Area UUID |
| `area` | Area name |
| `project_id` | Project UUID |
| `project` | Project title |
| `context` | Context string |
| `assigned_to` | Person name |

Prefer **names** when the user speaks names; use **IDs** from prior tool results.

### Clearable fields (updates)
On `gtd_task_update` / `gtd_project_update`:

* **Omit** a key → leave field unchanged.
* Pass **empty string `""`** → clear the field (where the CLI supports clear).

Clearable on tasks: `project_id`, `area_id`, `area`, `assigned_to`, `start_offset`, `recurrence`, `contexts`, `tags`.  
Clearable on projects: `area_id`, `area`.

For `contexts` / `tags`: empty string clears the whole list; a non-empty comma-separated value **replaces** the list (e.g. `@phone,@office`). Use this when a wrong context was assigned.

### System health resource
Read **`gtd://state`** for counts only (`inbox_count`, `next_count`, `agenda_count`, `stalled_project_count`, `waiting_count`, `someday_count`, `workspace_ok`, `errors`). Then call the matching query tool for full lists. Cache counts within a coaching turn.

If tools fail with missing workspace errors, call **`gtd_init`** first.

---

## Part 4: Step-by-Step Agent SOPs

### SOP 1: Clarify & Process Inbox
1. `gtd_get_inbox` (or check `inbox_count` on `gtd://state`).
2. For each task:
   - **2-minute rule**: if trivial, prompt user to do it and set status `done` via `gtd_task_update`.
   - **Delegate**: `gtd_people_add` if needed, then `gtd_task_update` with `status=waiting` and `assigned_to` (or NLP `"%Bob /waiting"` in `text`).
   - **Multi-step**: `gtd_project_add` if needed, bind with `project_id` or NLP `+Project`.
   - **Single next action**: `gtd_task_update` with `status=next` or text `/next`.
   - **Non-actionable**: `status=someday` / `reference`, or `gtd_task_delete`.

### SOP 2: Reflect & Weekly Review
1. `gtd_index_rebuild` — sync index after any external file edits. After copying the workspace to a new device (rsync/Unison/manual), call **`gtd_init`** then **`gtd_index_rebuild`** so dirs and the SQLite index exist on that machine.
2. `gtd_get_stalled` — projects with zero `next` actions; for each, list candidates (`gtd_task_list` with `project_id`) and promote one to `next` or add a step.
3. `gtd_task_list` with `status=waiting` — review delegations.
4. `gtd_project_list` — still relevant?
5. Optional: `gtd_task_list` with `status=someday`.

### SOP 3: Engage
1. Read `gtd://state` for counts.
2. `gtd_get_agenda` as primary checklist (filter by `context` / `area` for environment).
3. If agenda is empty, `gtd_get_next` with the same filters.
4. Complete work with `gtd_task_update` (`status=done`); watch for `project_stalled` in the response.

---

## Part 5: MCP Tool Catalog

| Tool | Purpose | Primary params |
| :--- | :--- | :--- |
| `gtd_init` | Bootstrap workspace | — |
| `gtd_index_rebuild` | Resync SQLite from files | — |
| `gtd_task_add` | Capture (NLP) | `text`; optional `project_id`, `area_id`, `area`, `assigned_to` |
| `gtd_task_update` | Clarify / organize | `id`; optional `text`, `status`, clearable fields (incl. `contexts`, `tags`), `start_offset`, `recurrence` |
| `gtd_task_list` | List by status/filters | optional `status` + shared filters |
| `gtd_task_delete` / `gtd_task_restore` | Soft-delete / restore | `id` |
| `gtd_task_duplicate` | Clone as next | `id` |
| `gtd_task_promote` | Task → project | `id`, `project_title` |
| `gtd_get_inbox` | Inbox list | — |
| `gtd_get_next` | Next actions | shared filters |
| `gtd_get_agenda` | Due/start now | shared filters |
| `gtd_get_stalled` | Projects without next | — |
| `gtd_project_add` | New project | `title`; optional `area_id`, `area` |
| `gtd_project_update` | Status / area | `id`; optional `status`, `area_id`, `area` |
| `gtd_project_list` / `delete` / `restore` | CRUD | `id` where needed |
| `gtd_area_*` | Area CRUD + cascade | `name` / `id` |
| `gtd_people_*` | People CRUD | `name` / `id` |
| `gtd_context_list` | Distinct contexts on tasks | — |
| `gtd_tag_list` | Distinct tags on tasks | — |

Resources: `gtd://methodology`, `gtd://guides/getting_started`, `gtd://state`.  
Prompt: `start_gtd_session`.
