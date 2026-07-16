package sqlite

const schemaSQL = `
CREATE TABLE IF NOT EXISTS tasks (
  id          TEXT PRIMARY KEY,
  title       TEXT NOT NULL,
  status      TEXT NOT NULL,
  priority    TEXT,
  energyLevel TEXT,
  assignedTo  TEXT,
  startTime   TEXT,
  relativeStartOffset TEXT,
  dueDate     TEXT,
  recurrence  TEXT,
  tags        TEXT,
  contexts    TEXT,
  description TEXT,
  textDirection TEXT,
  attachments TEXT,
  location    TEXT,
  projectId   TEXT REFERENCES projects(id) ON DELETE SET NULL,
  areaId      TEXT REFERENCES areas(id) ON DELETE SET NULL,
  orderNum    INTEGER,
  timeEstimate TEXT,
  timeSpentMinutes INTEGER,
  reviewAt    TEXT,
  completedAt TEXT,
  createdAt   TEXT NOT NULL,
  updatedAt   TEXT NOT NULL,
  deletedAt   TEXT
);

CREATE TABLE IF NOT EXISTS projects (
  id           TEXT PRIMARY KEY,
  title        TEXT NOT NULL,
  status       TEXT NOT NULL,
  color        TEXT NOT NULL,
  orderNum     INTEGER,
  tagIds       TEXT,
  supportNotes TEXT,
  attachments  TEXT,
  dueDate      TEXT,
  reviewAt     TEXT,
  areaId       TEXT REFERENCES areas(id) ON DELETE SET NULL,
  areaTitle    TEXT,
  createdAt    TEXT NOT NULL,
  updatedAt    TEXT NOT NULL,
  deletedAt    TEXT
);

CREATE TABLE IF NOT EXISTS areas (
  id       TEXT PRIMARY KEY,
  name     TEXT NOT NULL,
  color    TEXT,
  icon     TEXT,
  orderNum INTEGER NOT NULL,
  createdAt TEXT NOT NULL,
  updatedAt TEXT NOT NULL,
  deletedAt TEXT
);



CREATE TABLE IF NOT EXISTS people (
  id            TEXT PRIMARY KEY,
  name          TEXT NOT NULL,
  note          TEXT,
  referenceLink TEXT,
  createdAt     TEXT NOT NULL,
  updatedAt     TEXT NOT NULL,
  deletedAt     TEXT
);

CREATE TABLE IF NOT EXISTS settings (
  id   INTEGER PRIMARY KEY CHECK (id = 1),
  data TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS schema_migrations (
  version INTEGER PRIMARY KEY
);

-- Triggers for task status enum
CREATE TRIGGER IF NOT EXISTS trigger_tasks_status_insert
BEFORE INSERT ON tasks
FOR EACH ROW
WHEN NEW.status NOT IN ('inbox', 'next', 'waiting', 'someday', 'reference', 'done', 'archived')
BEGIN
    SELECT RAISE(ABORT, 'invalid task status');
END;

CREATE TRIGGER IF NOT EXISTS trigger_tasks_status_update
BEFORE UPDATE ON tasks
FOR EACH ROW
WHEN NEW.status NOT IN ('inbox', 'next', 'waiting', 'someday', 'reference', 'done', 'archived')
BEGIN
    SELECT RAISE(ABORT, 'invalid task status');
END;

-- Triggers for tasks JSON constraints
CREATE TRIGGER IF NOT EXISTS trigger_tasks_json_insert
BEFORE INSERT ON tasks
FOR EACH ROW
WHEN (NEW.tags IS NOT NULL AND NOT json_valid(NEW.tags)) OR
     (NEW.contexts IS NOT NULL AND NOT json_valid(NEW.contexts)) OR
     (NEW.attachments IS NOT NULL AND NOT json_valid(NEW.attachments)) OR
     (NEW.recurrence IS NOT NULL AND NOT json_valid(NEW.recurrence)) OR
     (NEW.relativeStartOffset IS NOT NULL AND NOT json_valid(NEW.relativeStartOffset))
BEGIN
    SELECT RAISE(ABORT, 'invalid json payload in tasks');
END;

CREATE TRIGGER IF NOT EXISTS trigger_tasks_json_update
BEFORE UPDATE ON tasks
FOR EACH ROW
WHEN (NEW.tags IS NOT NULL AND NOT json_valid(NEW.tags)) OR
     (NEW.contexts IS NOT NULL AND NOT json_valid(NEW.contexts)) OR
     (NEW.attachments IS NOT NULL AND NOT json_valid(NEW.attachments)) OR
     (NEW.recurrence IS NOT NULL AND NOT json_valid(NEW.recurrence)) OR
     (NEW.relativeStartOffset IS NOT NULL AND NOT json_valid(NEW.relativeStartOffset))
BEGIN
    SELECT RAISE(ABORT, 'invalid json payload in tasks');
END;

-- Triggers for project status enum
CREATE TRIGGER IF NOT EXISTS trigger_projects_status_insert
BEFORE INSERT ON projects
FOR EACH ROW
WHEN NEW.status NOT IN ('active', 'someday', 'waiting', 'archived')
BEGIN
    SELECT RAISE(ABORT, 'invalid project status');
END;

CREATE TRIGGER IF NOT EXISTS trigger_projects_status_update
BEFORE UPDATE ON projects
FOR EACH ROW
WHEN NEW.status NOT IN ('active', 'someday', 'waiting', 'archived')
BEGIN
    SELECT RAISE(ABORT, 'invalid project status');
END;

-- Triggers for projects JSON constraints
CREATE TRIGGER IF NOT EXISTS trigger_projects_json_insert
BEFORE INSERT ON projects
FOR EACH ROW
WHEN (NEW.tagIds IS NOT NULL AND NOT json_valid(NEW.tagIds)) OR
     (NEW.attachments IS NOT NULL AND NOT json_valid(NEW.attachments))
BEGIN
    SELECT RAISE(ABORT, 'invalid json payload in projects');
END;

CREATE TRIGGER IF NOT EXISTS trigger_projects_json_update
BEFORE UPDATE ON projects
FOR EACH ROW
WHEN (NEW.tagIds IS NOT NULL AND NOT json_valid(NEW.tagIds)) OR
     (NEW.attachments IS NOT NULL AND NOT json_valid(NEW.attachments))
BEGIN
    SELECT RAISE(ABORT, 'invalid json payload in projects');
END;

-- tasks Indexes
CREATE INDEX IF NOT EXISTS idx_tasks_status             ON tasks(status);
CREATE INDEX IF NOT EXISTS idx_tasks_projectId          ON tasks(projectId);
CREATE INDEX IF NOT EXISTS idx_tasks_deletedAt          ON tasks(deletedAt);
CREATE INDEX IF NOT EXISTS idx_tasks_dueDate            ON tasks(dueDate);
CREATE INDEX IF NOT EXISTS idx_tasks_startTime          ON tasks(startTime);
CREATE INDEX IF NOT EXISTS idx_tasks_reviewAt           ON tasks(reviewAt);
CREATE INDEX IF NOT EXISTS idx_tasks_completedAt        ON tasks(completedAt);
CREATE INDEX IF NOT EXISTS idx_tasks_createdAt          ON tasks(createdAt);
CREATE INDEX IF NOT EXISTS idx_tasks_updatedAt          ON tasks(updatedAt);
CREATE INDEX IF NOT EXISTS idx_tasks_status_deletedAt   ON tasks(status, deletedAt);
CREATE INDEX IF NOT EXISTS idx_tasks_project_deletedAt  ON tasks(projectId, deletedAt);
CREATE INDEX IF NOT EXISTS idx_tasks_project_status_deletedAt ON tasks(projectId, status, deletedAt);
CREATE INDEX IF NOT EXISTS idx_tasks_projectId_orderNum ON tasks(projectId, orderNum);
CREATE INDEX IF NOT EXISTS idx_tasks_area_deletedAt     ON tasks(areaId, deletedAt);
CREATE INDEX IF NOT EXISTS idx_tasks_area_id            ON tasks(areaId);

-- projects Indexes
CREATE INDEX IF NOT EXISTS idx_projects_status         ON projects(status);
CREATE INDEX IF NOT EXISTS idx_projects_areaId         ON projects(areaId);
CREATE INDEX IF NOT EXISTS idx_projects_area_deletedAt ON projects(areaId, deletedAt);
CREATE INDEX IF NOT EXISTS idx_projects_area_order     ON projects(areaId, orderNum);
CREATE INDEX IF NOT EXISTS idx_projects_dueDate        ON projects(dueDate);
`
