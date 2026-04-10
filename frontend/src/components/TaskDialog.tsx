import { useEffect, useState } from "react";
import {
  Alert,
  Button,
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
} from "@mui/material";
import type { ProjectMember, Task, TaskPriority, TaskStatus } from "../types";

const statuses: TaskStatus[] = ["todo", "in_progress", "done"];
const priorities: TaskPriority[] = ["low", "medium", "high"];

interface Props {
  open: boolean;
  onClose: () => void;
  members: ProjectMember[];
  task: Task | null;
  onSaved: (t: Task) => void;
  createTask: (body: {
    title: string;
    description?: string;
    priority: string;
    assignee_id?: string | null;
    due_date?: string | null;
  }) => Promise<Task>;
  patchTask: (id: string, body: Record<string, unknown>) => Promise<Task>;
}

export default function TaskDialog({
  open,
  onClose,
  members,
  task,
  onSaved,
  createTask,
  patchTask,
}: Props) {
  const [title, setTitle] = useState("");
  const [description, setDescription] = useState("");
  const [status, setStatus] = useState<TaskStatus>("todo");
  const [priority, setPriority] = useState<TaskPriority>("medium");
  const [assigneeId, setAssigneeId] = useState<string>("");
  const [dueDate, setDueDate] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    if (!open) return;
    setError(null);
    if (task) {
      setTitle(task.title);
      setDescription(task.description || "");
      setStatus(task.status);
      setPriority(task.priority);
      setAssigneeId(task.assignee_id || "");
      setDueDate(task.due_date || "");
    } else {
      setTitle("");
      setDescription("");
      setStatus("todo");
      setPriority("medium");
      setAssigneeId("");
      setDueDate("");
    }
  }, [open, task]);

  async function save() {
    setError(null);
    if (!title.trim()) {
      setError("Title is required");
      return;
    }
    setSaving(true);
    try {
      if (task) {
        const body: Record<string, unknown> = {
          title: title.trim(),
          description: description.trim() || null,
          status,
          priority,
          due_date: dueDate ? dueDate : null,
        };
        if (assigneeId) body.assignee_id = assigneeId;
        else body.assignee_id = null;
        const t = await patchTask(task.id, body);
        onSaved(t);
      } else {
        const t = await createTask({
          title: title.trim(),
          description: description.trim() || undefined,
          priority,
          assignee_id: assigneeId || undefined,
          due_date: dueDate || undefined,
        });
        onSaved(t);
      }
      onClose();
    } catch (e: unknown) {
      const err = e as { body?: { fields?: Record<string, string> }; message?: string };
      setError(err.message || "Could not save task");
    } finally {
      setSaving(false);
    }
  }

  return (
    <Dialog open={open} onClose={() => !saving && onClose()} fullWidth maxWidth="sm">
      <DialogTitle>{task ? "Edit task" : "New task"}</DialogTitle>
      <DialogContent>
        <Stack spacing={2} sx={{ mt: 1 }}>
          {error && <Alert severity="error">{error}</Alert>}
          <TextField label="Title" fullWidth required value={title} onChange={(e) => setTitle(e.target.value)} />
          <TextField
            label="Description"
            fullWidth
            multiline
            minRows={2}
            value={description}
            onChange={(e) => setDescription(e.target.value)}
          />
          {task && (
            <FormControl fullWidth>
              <InputLabel id="status-label">Status</InputLabel>
              <Select
                labelId="status-label"
                label="Status"
                value={status}
                onChange={(e) => setStatus(e.target.value as TaskStatus)}
              >
                {statuses.map((s) => (
                  <MenuItem key={s} value={s}>
                    {s.replace("_", " ")}
                  </MenuItem>
                ))}
              </Select>
            </FormControl>
          )}
          <FormControl fullWidth>
            <InputLabel id="pri-label">Priority</InputLabel>
            <Select
              labelId="pri-label"
              label="Priority"
              value={priority}
              onChange={(e) => setPriority(e.target.value as TaskPriority)}
            >
              {priorities.map((p) => (
                <MenuItem key={p} value={p}>
                  {p}
                </MenuItem>
              ))}
            </Select>
          </FormControl>
          <FormControl fullWidth>
            <InputLabel id="as-label">Assignee</InputLabel>
            <Select
              labelId="as-label"
              label="Assignee"
              value={assigneeId}
              onChange={(e) => setAssigneeId(e.target.value as string)}
            >
              <MenuItem value="">
                <em>Unassigned</em>
              </MenuItem>
              {members.map((m) => (
                <MenuItem key={m.id} value={m.id}>
                  {m.name} ({m.email})
                </MenuItem>
              ))}
            </Select>
          </FormControl>
          <TextField
            label="Due date"
            type="date"
            fullWidth
            InputLabelProps={{ shrink: true }}
            value={dueDate}
            onChange={(e) => setDueDate(e.target.value)}
          />
        </Stack>
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose} disabled={saving}>
          Cancel
        </Button>
        <Button variant="contained" onClick={() => void save()} disabled={saving}>
          {saving ? "Saving…" : "Save"}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
