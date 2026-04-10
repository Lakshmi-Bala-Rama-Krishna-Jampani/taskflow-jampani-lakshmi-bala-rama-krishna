import type { ApiError, Project, ProjectMember, Task, User } from "../types";

const TOKEN_KEY = "taskflow_token";

/** Set from AuthProvider so 401 responses clear session and redirect to login. */
let onUnauthorized: (() => void) | null = null;

export function setOnUnauthorized(handler: (() => void) | null): void {
  onUnauthorized = handler;
}

export function getApiBase(): string {
  const v = import.meta.env.VITE_API_URL;
  if (v && typeof v === "string" && v.length > 0) return v.replace(/\/$/, "");
  return "";
}

/** SSE URL (EventSource cannot send Authorization header; token is in query). */
export function projectEventsURL(projectId: string, token: string): string {
  const base = getApiBase();
  const path = `/projects/${encodeURIComponent(projectId)}/events`;
  const q = `?token=${encodeURIComponent(token)}`;
  if (base) return `${base}${path}${q}`;
  return `${window.location.origin}${path}${q}`;
}

export function getStoredToken(): string | null {
  try {
    return localStorage.getItem(TOKEN_KEY);
  } catch {
    return null;
  }
}

export function setStoredToken(token: string | null): void {
  try {
    if (token) localStorage.setItem(TOKEN_KEY, token);
    else localStorage.removeItem(TOKEN_KEY);
  } catch {
    /* ignore */
  }
}

async function parseJson<T>(res: Response): Promise<T> {
  const text = await res.text();
  if (!text) return {} as T;
  return JSON.parse(text) as T;
}

async function request<T>(
  path: string,
  init: RequestInit & { auth?: boolean } = {},
): Promise<T> {
  const base = getApiBase();
  const url = base ? `${base}${path}` : path;
  const headers = new Headers(init.headers);
  if (!headers.has("Content-Type") && init.body) {
    headers.set("Content-Type", "application/json");
  }
  if (init.auth !== false) {
    const t = getStoredToken();
    if (t) headers.set("Authorization", `Bearer ${t}`);
  }
  const res = await fetch(url, { ...init, headers });
  if (res.status === 204) return undefined as T;
  const data = await parseJson<ApiError & T>(res);
  if (!res.ok) {
    if (res.status === 401 && init.auth !== false) {
      try {
        onUnauthorized?.();
      } catch {
        /* ignore */
      }
    }
    const err = new Error((data as ApiError).error || res.statusText) as Error & {
      status: number;
      body: ApiError;
    };
    err.status = res.status;
    err.body = data as ApiError;
    throw err;
  }
  return data as T;
}

export const api = {
  async register(body: { name: string; email: string; password: string }) {
    return request<{ token: string; user: User }>("/auth/register", {
      method: "POST",
      body: JSON.stringify(body),
      auth: false,
    });
  },
  async login(body: { email: string; password: string }) {
    return request<{ token: string; user: User }>("/auth/login", {
      method: "POST",
      body: JSON.stringify(body),
      auth: false,
    });
  },
  async listProjects(page = 1, limit = 50) {
    const q = new URLSearchParams({ page: String(page), limit: String(limit) });
    return request<{ projects: Project[] }>(`/projects?${q}`);
  },
  async createProject(body: { name: string; description?: string }) {
    return request<Project>("/projects", { method: "POST", body: JSON.stringify(body) });
  },
  async getProject(id: string) {
    return request<Project & { tasks: Task[] }>(`/projects/${id}`);
  },
  async updateProject(id: string, body: { name?: string; description?: string }) {
    return request<Project>(`/projects/${id}`, {
      method: "PATCH",
      body: JSON.stringify(body),
    });
  },
  async deleteProject(id: string) {
    await request<undefined>(`/projects/${id}`, { method: "DELETE" });
  },
  async listTasks(projectId: string, filters?: { status?: string; assignee?: string }) {
    const q = new URLSearchParams();
    if (filters?.status) q.set("status", filters.status);
    if (filters?.assignee) q.set("assignee", filters.assignee);
    const suffix = q.toString() ? `?${q}` : "";
    return request<{ tasks: Task[] }>(`/projects/${projectId}/tasks${suffix}`);
  },
  async createTask(
    projectId: string,
    body: {
      title: string;
      description?: string;
      priority: string;
      assignee_id?: string | null;
      due_date?: string | null;
    },
  ) {
    return request<Task>(`/projects/${projectId}/tasks`, {
      method: "POST",
      body: JSON.stringify(body),
    });
  },
  async patchTask(id: string, body: Record<string, unknown>) {
    return request<Task>(`/tasks/${id}`, {
      method: "PATCH",
      body: JSON.stringify(body),
    });
  },
  async reorderTasks(projectId: string, columns: Record<string, string[]>) {
    await request<undefined>(`/projects/${projectId}/tasks/reorder`, {
      method: "POST",
      body: JSON.stringify({ columns }),
    });
  },
  async deleteTask(id: string) {
    await request<undefined>(`/tasks/${id}`, { method: "DELETE" });
  },
  async projectMembers(projectId: string) {
    return request<{ members: ProjectMember[] }>(`/projects/${projectId}/members`);
  },
  async projectStats(projectId: string) {
    return request<{
      by_status: Record<string, number>;
      by_assignee: { assignee_id: string | null; count: number }[];
    }>(`/projects/${projectId}/stats`);
  },
};
