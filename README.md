# TaskFlow

## Overview

Small task app: sign up, log in, projects, tasks (status, priority, assignee, due date). You only see projects you own or where you’re assigned at least one task; same rule for creating tasks. Delete task = owner or whoever created the task.

**What it runs on**

- Go 1.22, Chi, pgx, JWT + bcrypt, slog, graceful shutdown.
- Postgres 16, migrations with golang-migrate (up/down).
- React 18, TypeScript, Vite, MUI, React Router.
- Root `docker-compose.yml`: Postgres, API (Dockerfile), SPA behind nginx.

---

## Why things are shaped this way

Monorepo (`backend/`, `frontend/`) so one compose file spins everything up. Chi + JWT middleware on protected routes only. SQL in a thin `store` layer over `pgxpool`—no ORM, schema lives in migrations.

Project list = you own it or you have an assigned task there. Assignees have to already be “in” the project (owner or on a task) so you can’t paste random UUIDs. `created_by` on tasks so delete can match the spec (owner or creator).

Validation errors are 400 with `fields`; 401 vs 403 kept separate. CORS uses `FRONTEND_ORIGIN` (default `http://localhost:3000`). No refresh tokens—JWT expires, SPA clears storage on 401 and sends you to login. Skipped orgs, comments on tasks, uploads, fancy roles.

Frontend: token + cached user in `localStorage`, guarded routes. Optimistic status updates with rollback on error. Kanban drag-and-drop, dark mode in localStorage, SSE for live updates (see below). Layout is responsive enough for phone vs desktop.

---

## Run it

Need Docker (Compose v2). You don’t need Go/Node on the machine if you use Compose.

```bash
git clone <repo-url>
cd <repo-folder>
cp .env.example .env
# set JWT_SECRET to something long and random; don’t commit .env
docker compose up --build
```

- UI: http://localhost:3000  
- API: http://localhost:8080 (`GET /health`)  
- Postgres: localhost:5432 unless you change `.env`

Migrations run when the API container starts.

---

## Migrations by hand

Inside the container they already run on boot (`MIGRATIONS_PATH` → `/app/migrations`). For a local Postgres without Docker:

```bash
cd backend
# migrate CLI: https://github.com/golang-migrate/migrate/tree/master/cmd/migrate
migrate -path migrations -database "$DATABASE_URL" up
```

Down:

```bash
migrate -path migrations -database "$DATABASE_URL" down
```

---

## Seed login

After migrate + seed:

| Field    | Value              |
|----------|--------------------|
| Email    | `test@example.com` |
| Password | `password123`      |

Seed uses `pgcrypto` bcrypt-style hashes so Go can check them the same way as registered users.

---

## API (quick reference)

Base: `http://localhost:8080` — JSON everywhere.

### Auth

**POST /auth/register** — JSON body with `name`, `email`, `password` → 201 + `{ token, user }`

**POST /auth/login** — JSON body with `email`, `password` → 200 + `{ token, user }`

Claims: `user_id`, `email`, `sub`, `exp` (~24h). Everything else: `Authorization: Bearer <token>`.

### Projects & tasks

| Method | Path | Notes |
|--------|------|--------|
| GET | `/projects?page=&limit=` | yours or assigned |
| POST | `/projects` | `{ name, description? }` |
| GET | `/projects/:id` | includes tasks |
| PATCH | `/projects/:id` | owner only |
| DELETE | `/projects/:id` | owner; cascades tasks |
| GET | `/projects/:id/tasks?...` | filters optional |
| POST | `/projects/:id/tasks` | default status `todo` |
| GET | `/projects/:id/members` | possible assignees |
| GET | `/projects/:id/stats` | counts by status / assignee |
| PATCH | `/tasks/:id` | owner, creator, or assignee |
| DELETE | `/tasks/:id` | owner or creator |
| POST | `/projects/:id/tasks/reorder` | `{ "columns": { "todo": [uuid...], ... } }` full board |

### Misc

| Method | Path | Notes |
|--------|------|--------|
| GET | `/health` | `{ "status": "ok" }` |
| GET | `/projects/:id/events?token=<JWT>` | SSE; EventSource can’t send headers so token is query param (don’t log that URL) |

Example create task:

```json
{
  "title": "Design homepage",
  "description": "optional",
  "priority": "high",
  "assignee_id": "uuid-or-omit",
  "due_date": "2026-04-15"
}
```

PATCH: omit fields you don’t change; `"assignee_id": null` or `"due_date": null` clears.

**Errors:** 400 + `fields`, 401/403/404 with `error` string as in handlers.

---

## Extra stuff I added

Not just “future work”—this repo already has:

- Kanban DnD + `sort_order` + `POST .../tasks/reorder`
- Optimistic task moves with revert on failure
- Light/dark toggle (saved locally)
- SSE `project_tasks_changed` → client refetches project
- 401 on any authed call clears session → `/login?reason=session_expired`

If I kept going: refresh tokens, richer roles, Playwright + more Go integration tests in CI, idempotent reorder, OpenAPI.

Rough edges I’m aware of: no email verification, assignee list is heuristic, integration tests need `TEST_DATABASE_URL`, single JWT without refresh.

---

## Bonus bits

- Paged `GET /projects` and `GET /projects/:id/tasks`
- `GET /projects/:id/stats`
- Integration tests under `backend/integration/` with `-tags=integration`

Check the frontend build: `cd frontend && npm ci && npm run build`, then maybe `npm run preview` and resize the window.

---

## Layout

```
.
├── backend/       # API, migrations, Dockerfile
├── frontend/      # Vite + MUI, Dockerfile, nginx
├── docker-compose.yml
└── .env.example
```
