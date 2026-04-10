import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useNavigate, useParams } from "react-router-dom";
import { useAuth } from "../context/AuthContext";
import {
  Alert,
  Box,
  Button,
  Card,
  CardContent,
  Chip,
  CircularProgress,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  FormControl,
  InputLabel,
  MenuItem,
  Select,
  Stack,
  TextField,
  Typography,
} from "@mui/material";
import AddIcon from "@mui/icons-material/Add";
import ArrowBackIcon from "@mui/icons-material/ArrowBack";
import { api, projectEventsURL } from "../api/client";
import KanbanBoard from "../components/KanbanBoard";
import TaskDialog from "../components/TaskDialog";
import type { ProjectMember, Task, TaskStatus } from "../types";

const statusOrder: TaskStatus[] = ["todo", "in_progress", "done"];

function labelStatus(s: TaskStatus): string {
  switch (s) {
    case "todo":
      return "To do";
    case "in_progress":
      return "In progress";
    case "done":
      return "Done";
    default:
      return s;
  }
}

export default function ProjectDetailPage() {
  const { id } = useParams<{ id: string }>();
  const nav = useNavigate();
  const { token } = useAuth();
  const [projectName, setProjectName] = useState<string>("");
  const [description, setDescription] = useState<string | null>(null);
  const [tasks, setTasks] = useState<Task[]>([]);
  const [members, setMembers] = useState<ProjectMember[]>([]);
  const [stats, setStats] = useState<{ by_status: Record<string, number> } | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [filterStatus, setFilterStatus] = useState<string>("");
  const [filterAssignee, setFilterAssignee] = useState<string>("");
  const [dialogOpen, setDialogOpen] = useState(false);
  const [editing, setEditing] = useState<Task | null>(null);
  const [editProjectOpen, setEditProjectOpen] = useState(false);
  const [projNameEdit, setProjNameEdit] = useState("");
  const [projDescEdit, setProjDescEdit] = useState("");
  const [savingProj, setSavingProj] = useState(false);

  const load = useCallback(async () => {
    if (!id) return;
    setLoading(true);
    setError(null);
    try {
      const p = await api.getProject(id);
      setProjectName(p.name);
      setDescription(p.description ?? null);
      const filters: { status?: string; assignee?: string } = {};
      if (filterStatus) filters.status = filterStatus;
      if (filterAssignee) filters.assignee = filterAssignee;
      if (!filterStatus && !filterAssignee) {
        setTasks(p.tasks || []);
      } else {
        const res = await api.listTasks(id, filters);
        setTasks(res.tasks);
      }
      const [m, st] = await Promise.all([api.projectMembers(id), api.projectStats(id)]);
      setMembers(m.members);
      setStats({ by_status: st.by_status });
    } catch (e: unknown) {
      const err = e as { message?: string };
      setError(err.message || "Failed to load project");
    } finally {
      setLoading(false);
    }
  }, [id, filterStatus, filterAssignee]);

  useEffect(() => {
    void load();
  }, [load]);

  const loadRef = useRef(load);
  loadRef.current = load;

  useEffect(() => {
    if (!token || !id) return;
    const url = projectEventsURL(id, token);
    const es = new EventSource(url);
    es.onmessage = (ev) => {
      try {
        const data = JSON.parse(ev.data) as { type?: string };
        if (data.type === "project_tasks_changed") void loadRef.current();
      } catch {
        /* ignore */
      }
    };
    es.onerror = () => {
      es.close();
    };
    return () => es.close();
  }, [token, id]);

  const grouped = useMemo(() => {
    const g: Record<TaskStatus, Task[]> = { todo: [], in_progress: [], done: [] };
    for (const t of tasks) {
      if (g[t.status]) g[t.status].push(t);
    }
    return g;
  }, [tasks]);

  async function optimisticStatus(task: Task, next: TaskStatus) {
    const previous = tasks;
    setTasks((list) => list.map((x) => (x.id === task.id ? { ...x, status: next } : x)));
    try {
      const updated = await api.patchTask(task.id, { status: next });
      setTasks((list) => list.map((x) => (x.id === task.id ? updated : x)));
      if (id) {
        const st = await api.projectStats(id);
        setStats({ by_status: st.by_status });
      }
    } catch {
      setTasks(previous);
      setError("Could not update status — reverted");
    }
  }

  const handleReorder = useCallback(
    async (columns: Record<TaskStatus, string[]>) => {
      if (!id) return;
      await api.reorderTasks(id, columns);
      await load();
    },
    [id, load],
  );

  async function saveProject() {
    if (!id) return;
    setSavingProj(true);
    try {
      const p = await api.updateProject(id, { name: projNameEdit, description: projDescEdit });
      setProjectName(p.name);
      setDescription(p.description ?? null);
      setEditProjectOpen(false);
    } catch (e: unknown) {
      const err = e as { message?: string };
      setError(err.message || "Update failed");
    } finally {
      setSavingProj(false);
    }
  }

  async function deleteProject() {
    if (!id || !window.confirm("Delete this project and all tasks?")) return;
    try {
      await api.deleteProject(id);
      nav("/");
    } catch (e: unknown) {
      const err = e as { message?: string };
      setError(err.message || "Delete failed");
    }
  }

  if (!id) return null;

  if (loading) {
    return (
      <Box display="flex" justifyContent="center" py={6}>
        <CircularProgress />
      </Box>
    );
  }

  return (
    <Stack spacing={3}>
      <Button startIcon={<ArrowBackIcon />} onClick={() => nav("/")} sx={{ alignSelf: "flex-start" }}>
        All projects
      </Button>
      {error && (
        <Alert severity="error" onClose={() => setError(null)}>
          {error}
        </Alert>
      )}
      <Stack spacing={1}>
        <Typography variant="h4" fontWeight={700}>
          {projectName || "Project"}
        </Typography>
        {description && (
          <Typography variant="body1" color="text.secondary">
            {description}
          </Typography>
        )}
        <Stack direction={{ xs: "column", sm: "row" }} gap={1} flexWrap="wrap">
          <Button
            variant="outlined"
            size="small"
            onClick={() => {
              setProjNameEdit(projectName);
              setProjDescEdit(description || "");
              setEditProjectOpen(true);
            }}
          >
            Edit project
          </Button>
          <Button color="error" variant="outlined" size="small" onClick={() => void deleteProject()}>
            Delete project
          </Button>
          <Button
            variant="contained"
            startIcon={<AddIcon />}
            onClick={() => {
              setEditing(null);
              setDialogOpen(true);
            }}
          >
            New task
          </Button>
        </Stack>
      </Stack>

      {stats && (
        <Stack direction="row" gap={1} flexWrap="wrap">
          {statusOrder.map((s) => (
            <Chip key={s} label={`${labelStatus(s)}: ${stats.by_status[s] ?? 0}`} variant="outlined" />
          ))}
        </Stack>
      )}

      <Stack direction={{ xs: "column", md: "row" }} spacing={2}>
        <FormControl sx={{ minWidth: 180 }} size="small">
          <InputLabel id="fs">Status filter</InputLabel>
          <Select
            labelId="fs"
            label="Status filter"
            value={filterStatus}
            onChange={(e) => setFilterStatus(e.target.value)}
          >
            <MenuItem value="">All</MenuItem>
            {statusOrder.map((s) => (
              <MenuItem key={s} value={s}>
                {labelStatus(s)}
              </MenuItem>
            ))}
          </Select>
        </FormControl>
        <FormControl sx={{ minWidth: 220 }} size="small">
          <InputLabel id="fa">Assignee filter</InputLabel>
          <Select
            labelId="fa"
            label="Assignee filter"
            value={filterAssignee}
            onChange={(e) => setFilterAssignee(e.target.value)}
          >
            <MenuItem value="">All</MenuItem>
            {members.map((m) => (
              <MenuItem key={m.id} value={m.id}>
                {m.name}
              </MenuItem>
            ))}
          </Select>
        </FormControl>
      </Stack>

      {tasks.length === 0 && (
        <Card variant="outlined">
          <CardContent>
            <Typography fontWeight={600}>No tasks match</Typography>
            <Typography variant="body2" color="text.secondary" sx={{ mt: 1 }}>
              {filterStatus || filterAssignee
                ? "Try clearing filters or create a new task."
                : "Create a task to get started."}
            </Typography>
          </CardContent>
        </Card>
      )}

      {!filterStatus && !filterAssignee ? (
        <KanbanBoard
          tasks={tasks}
          onReorder={handleReorder}
          onStatusChange={(task, next) => void optimisticStatus(task, next)}
          onEdit={(task) => {
            setEditing(task);
            setDialogOpen(true);
          }}
        />
      ) : (
        <Box
          display="grid"
          gridTemplateColumns={{ xs: "1fr", md: "repeat(3, 1fr)" }}
          gap={2}
          alignItems="start"
        >
          {statusOrder.map((col) => (
            <Stack key={col} spacing={1.5}>
              <Typography variant="subtitle2" color="text.secondary" fontWeight={700}>
                {labelStatus(col)}
              </Typography>
              {(filterStatus && filterStatus !== col ? [] : grouped[col]).map((task) => (
                <Card key={task.id} variant="outlined">
                  <CardContent sx={{ "&:last-child": { pb: 2 } }}>
                    <Typography fontWeight={600}>{task.title}</Typography>
                    {task.description && (
                      <Typography variant="body2" color="text.secondary" sx={{ mt: 0.5 }}>
                        {task.description}
                      </Typography>
                    )}
                    <Stack direction="row" gap={1} flexWrap="wrap" sx={{ mt: 1 }} alignItems="center">
                      <Chip size="small" label={task.priority} />
                      <FormControl size="small" sx={{ minWidth: 140 }}>
                        <InputLabel id={`st-${task.id}`}>Status</InputLabel>
                        <Select
                          labelId={`st-${task.id}`}
                          label="Status"
                          value={task.status}
                          onChange={(e) => void optimisticStatus(task, e.target.value as TaskStatus)}
                        >
                          {statusOrder.map((s) => (
                            <MenuItem key={s} value={s}>
                              {labelStatus(s)}
                            </MenuItem>
                          ))}
                        </Select>
                      </FormControl>
                    </Stack>
                    <Stack direction="row" gap={1} sx={{ mt: 1.5 }}>
                      <Button
                        size="small"
                        onClick={() => {
                          setEditing(task);
                          setDialogOpen(true);
                        }}
                      >
                        Edit
                      </Button>
                    </Stack>
                  </CardContent>
                </Card>
              ))}
            </Stack>
          ))}
        </Box>
      )}

      <TaskDialog
        open={dialogOpen}
        onClose={() => setDialogOpen(false)}
        members={members}
        task={editing}
        onSaved={(t) => {
          setTasks((list) => {
            const ix = list.findIndex((x) => x.id === t.id);
            if (ix === -1) return [...list, t];
            const copy = [...list];
            copy[ix] = t;
            return copy;
          });
          if (id) void api.projectStats(id).then((st) => setStats({ by_status: st.by_status }));
        }}
        createTask={(body) => api.createTask(id, body)}
        patchTask={(tid, body) => api.patchTask(tid, body)}
      />

      <Dialog open={editProjectOpen} onClose={() => !savingProj && setEditProjectOpen(false)} fullWidth maxWidth="sm">
        <DialogTitle>Edit project</DialogTitle>
        <DialogContent>
          <Stack spacing={2} sx={{ mt: 1 }}>
            <TextField label="Name" fullWidth value={projNameEdit} onChange={(e) => setProjNameEdit(e.target.value)} />
            <TextField
              label="Description"
              fullWidth
              multiline
              minRows={2}
              value={projDescEdit}
              onChange={(e) => setProjDescEdit(e.target.value)}
            />
          </Stack>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setEditProjectOpen(false)} disabled={savingProj}>
            Cancel
          </Button>
          <Button variant="contained" onClick={() => void saveProject()} disabled={savingProj}>
            {savingProj ? "Saving…" : "Save"}
          </Button>
        </DialogActions>
      </Dialog>
    </Stack>
  );
}
