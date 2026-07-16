# Getting Started with GTD CLI

Welcome! This guide helps you set up your system, perform your initial massive capture, and structure your life domains.

### Using the MCP server (AI agents)
1. Call **`gtd_init`** once if the workspace is not initialized.
2. Capture with **`gtd_task_add`** (same NLP tokens as CLI).
3. Process with **`gtd_get_inbox`** and **`gtd_task_update`**.
4. Read **`gtd://methodology`** for full SOPs and the tool catalog; use **`gtd://state`** for health counts.

Human CLI equivalents: `gtd init`, `gtd task add`, `gtd inbox`, `gtd task update`.

---

## 1. Structuring Your System

### Areas of Focus (`!area`)
Areas are your ongoing, infinite responsibilities. If it doesn't have an end date, it is an Area:
* Use areas for major clients, life domains, or departments (e.g., `!Work`, `!Personal`, `!Home`, `!Leisure`).

### Projects (`+project`)
Projects are finite, multi-step outcomes with a clear "definition of done" (achievable in weeks/months):
* **Good**: `+ClientA_Q3_Dashboard`, `+Onboard_New_Dev`, `+Fix_Kitchen_Sink`.
* **Avoid**: Creating open-ended project containers like `+ClientA` or `+Housework`. Piling tasks into them indefinitely breaks "stalled project" heuristics.

### Contexts (`@context`) & Tags (`#tag`)
* **Contexts**: Where or how you do a task (e.g., `@computer`, `@home`, `@phone`, `@shopping`).
* **Tags**: Use tags for multi-dimensional grouping (e.g., `#research`, `#chore`, teammate names, or client names if you don't want dedicated Areas).

---

## 2. Your First Capture (The Brain Dump)

The golden rule of GTD is: **Separate Capture from Clarification.** 
When doing your initial dump, do not spend time setting up Areas or Projects first, and do not worry about prioritization. Just empty your head.

* **Capture Anything**: Dump everything raw into your inbox.
* **No-Error Project Capture**: If you associate a task with a project that doesn't exist yet (e.g., typing `+Fix_Kitchen_Sink`), the system will not fail. It captures the task safely and designates the project as "to be created later" when you clarify your inbox.
* **Examples**:
  * `Refactor auth middleware +ClientA_Auth @computer /due:friday`
  * `Deep clean kitchen @home`
  * `Water plants /recur:weekly`

---

## 3. Clarifying Your Inbox

Once your brain dump is complete and your inbox is full, switch gears to process the items one by one:
1. **Is it actionable?** If no, delete it, snooze it to `someday`, or mark it as a `reference` note.
2. **Is it multiple steps?** If yes, it is a project. Create the project entity (e.g., `+Fix_Kitchen_Sink`) under its corresponding Area and move the task into it.
3. **What is the next action?** Assign the task a status: `/next` (do as soon as possible), `/waiting` (delegated to `%person`), or `/due:` (date-specific).
4. **Context & Tags**: Assign contexts (`@context`) and tags (`#tag`) to help you filter what is doable in your current physical or mental state.

### Pro-Tip: The First Clarify is Different
* **The First Clarify (Setup)**: Since you are starting from a clean slate, this first session will take longer. You are not just sorting tasks; you are establishing your Projects and Areas from scratch. Treat this initial structure as a **draft** that you will refine over time.
* **Subsequent Clarifies (Maintenance)**: Once the structure is built, daily clarifies will be fast, mechanical, and take only 5–10 minutes.
