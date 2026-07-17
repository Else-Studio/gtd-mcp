# gtd

**Alpha** — local-first [Getting Things Done](https://gettingthingsdone.com/) for people who work with AI agents.

`gtd` is a command-line GTD system. The intended way to use it is through the built-in **MCP server**: your agent runs the tools, applies GTD coaching, and you stay in conversation. The CLI exposes the full GTD surface as plain commands when you want it; the MCP layer is the opinionated coach.

| | |
| --- | --- |
| **Binary** | `gtd` |
| **Repository** | [Else-Studio/gtd-mcp](https://github.com/Else-Studio/gtd-mcp) |
| **License** | [MIT](LICENSE) |

---

## Install

### From GitHub Releases (recommended)

Download a prebuilt `gtd` binary from the [Releases](https://github.com/Else-Studio/gtd-mcp/releases) page, put it on your `PATH`, and verify:

```bash
gtd --help
```

> **Alpha note:** the first public release may still be in progress. Until assets appear on Releases, build from source below.

### From source

Requires a recent [Go](https://go.dev/dl/) toolchain.

```bash
git clone https://github.com/Else-Studio/gtd-mcp.git
cd gtd-mcp
go build -o gtd ./cmd/gtd
# optional: install onto PATH
# go install ./cmd/gtd
```

---

## Use with your AI agent (recommended)

1. Install the `gtd` binary (above).
2. Register the MCP server in your client (stdio: `gtd mcp`).
3. Tell the agent you want to do GTD (or use the `start_gtd_session` prompt). It should read the methodology resource, check system health, and drive capture / clarify / review / engage via tools — **you do not need to learn the CLI**.

### Claude Desktop

Add to your Claude Desktop MCP config (path varies by OS; typically under Claude’s application support / config directory):

```json
{
  "mcpServers": {
    "gtd": {
      "command": "gtd",
      "args": ["mcp"]
    }
  }
}
```

If `gtd` is not on the absolute path Claude uses, set `"command"` to the full path of the binary.

### Cursor

In Cursor MCP settings, add a server equivalent to:

```json
{
  "mcpServers": {
    "gtd": {
      "command": "gtd",
      "args": ["mcp"]
    }
  }
}
```

Other stdio MCP clients work the same way: run `gtd mcp` as the server process.

### What the agent gets

| Kind | Name | Role |
| --- | --- | --- |
| Resource | `gtd://methodology` | GTD coaching rules, NLP tokens, SOPs, tool catalog |
| Resource | `gtd://guides/getting_started` | Onboarding (areas, projects, first capture/clarify) |
| Resource | `gtd://state` | Compact health counts (inbox, next, agenda, stalled, …) |
| Prompt | `start_gtd_session` | Kick off a guided session from current state |
| Tools | `gtd_*` | Capture, clarify, list, projects/areas/people, engage views |

MCP tools invoke the same `gtd` CLI under the hood. Coaching workflow (when to process inbox, how to run a weekly review, what to do when a project stalls) lives in the MCP resources and tool descriptions — not as hidden CLI magic.

**First run:** if the workspace is missing, the agent (or you) should call `gtd_init` / `gtd init` once.

---

## Use the CLI directly

Everything is non-interactive. Default output is **JSON** (agent-friendly). Pass **`--plain`** for human tables.

```bash
gtd init
gtd add Call the plumber about the leak
gtd add Email Bob about proposal %Bob @computer /due:tomorrow
gtd inbox --plain
gtd next --plain
gtd agenda --plain
gtd stalled --plain
```

### Quick-add tokens

| Token | Meaning |
| --- | --- |
| `+Project` | Bind to a project (quote multi-word: `+"Kitchen Sink"`) |
| `!Area` | Bind to an area of focus when no project is set |
| `@context` | Context (e.g. `@computer`, `@phone`) |
| `#tag` | Tag |
| `%Person` | Delegate / waiting-for |
| `/due:…` `/start:…` `/review:…` | Dates (`today`, `tomorrow`, weekday, `YYYY-MM-DD`) |
| `/recur:…` | Recurrence where supported |
| `/next` `/waiting` `/someday` `/reference` `/done` | Status |

### Command map (GTD phases)

| Phase | Commands |
| --- | --- |
| **Capture** | `gtd add …` · `gtd task add …` |
| **Clarify / Organize** | `gtd inbox` · `gtd task update …` · `project` / `area` / `people` · `gtd task promote` |
| **Reflect** | `gtd stalled` · `gtd task list waiting` · `gtd task list someday` · `gtd index rebuild` |
| **Engage** | `gtd agenda` · `gtd next` |

Full entity CRUD:

- `gtd task` — add, update, list, delete, restore, duplicate, promote  
- `gtd project` — add, update, list, delete, restore  
- `gtd area` — add, update, list, delete, restore  
- `gtd people` — add, update, list, delete  
- `gtd init` · `gtd index rebuild` · `gtd mcp`

Use `gtd <command> --help` for flags and examples.

### Workspace

Data lives in a single local workspace:

- Default: `~/.gtd`
- Override: set `GTD_DIR` to another directory

Layout (created by `gtd init`):

```text
~/.gtd/
├── tasks/       # Markdown + YAML frontmatter (source of truth)
├── projects/
├── areas/
├── people/
├── index.db     # Rebuildable SQLite read index
└── config.yml   # Placeholder for now
```

If you edit files by hand, run `gtd index rebuild` so queries stay in sync.

---

## Design: unopinionated CLI, opinionated MCP

- **CLI** — full GTD *primitives*: statuses, containers, capture parser, agenda/next/stalled, soft-delete, and the structural rules the method requires (for example project vs area exclusivity, stall detection when the last next action is completed). It does not walk you through a weekly review; it executes commands.
- **MCP** — the coach: methodology text, session prompt, health snapshot, and tool guidance so an agent can run Capture → Clarify → Organize → Reflect → Engage with you in natural language.

You can ignore MCP and script the CLI. Most users should only talk to the agent.

---

## For developers

```bash
# from this repository root
go test ./...
go build -o gtd ./cmd/gtd
```

| Path | Responsibility |
| --- | --- |
| `cmd/gtd` | Cobra CLI, JSON/`--plain` output, MCP server |
| `internal/domain` | Entities, validation, GTD rules (no I/O) |
| `internal/parser` | Quick-add NLP |
| `internal/persistence/fs` | Markdown/YAML repositories |
| `internal/persistence/sqlite` | Index schema, sync, query views |

Contributions and forks: keep domain logic free of I/O, prefer JSON-stable CLI contracts, and treat markdown files as the source of truth.

### GTD reference

Domain concepts and methodology mapping were cross-checked against [Mindwtr](https://github.com/dongdongbh/Mindwtr) — an open-source GTD implementation that tracks the method closely. It is used here as a primary reference for GTD behavior and vocabulary, not as a product fork or visual/design inspiration.

---

## License

[MIT](LICENSE) © Else Studio
