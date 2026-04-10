import { useCallback, useEffect, useState } from "react";
import { Link as RouterLink } from "react-router-dom";
import {
  Alert,
  Box,
  Button,
  Card,
  CardActionArea,
  CardContent,
  CircularProgress,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  Stack,
  TextField,
  Typography,
} from "@mui/material";
import AddIcon from "@mui/icons-material/Add";
import { api } from "../api/client";
import type { Project } from "../types";

export default function ProjectsPage() {
  const [projects, setProjects] = useState<Project[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [open, setOpen] = useState(false);
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [createError, setCreateError] = useState<string | null>(null);
  const [creating, setCreating] = useState(false);

  const load = useCallback(async () => {
    setError(null);
    setLoading(true);
    try {
      const res = await api.listProjects();
      setProjects(res.projects);
    } catch (e: unknown) {
      const err = e as { message?: string };
      setError(err.message || "Failed to load projects");
      setProjects([]);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  async function createProject() {
    setCreateError(null);
    if (!name.trim()) {
      setCreateError("Name is required");
      return;
    }
    setCreating(true);
    try {
      const p = await api.createProject({
        name: name.trim(),
        description: description.trim() || undefined,
      });
      setProjects((prev) => (prev ? [p, ...prev] : [p]));
      setOpen(false);
      setName("");
      setDescription("");
    } catch (e: unknown) {
      const err = e as { body?: { fields?: Record<string, string> }; message?: string };
      setCreateError(err.body?.fields?.name || err.message || "Could not create project");
    } finally {
      setCreating(false);
    }
  }

  if (loading) {
    return (
      <Box display="flex" justifyContent="center" py={6}>
        <CircularProgress />
      </Box>
    );
  }

  return (
    <Stack spacing={3}>
      <Box display="flex" flexDirection={{ xs: "column", sm: "row" }} gap={2} alignItems={{ sm: "center" }}>
        <Typography variant="h4" fontWeight={700} sx={{ flex: 1 }}>
          Projects
        </Typography>
        <Button variant="contained" startIcon={<AddIcon />} onClick={() => setOpen(true)} fullWidth sx={{ maxWidth: { sm: 220 } }}>
          New project
        </Button>
      </Box>
      {error && (
        <Alert severity="error" onClose={() => setError(null)}>
          {error}
        </Alert>
      )}
      {projects && projects.length === 0 && !error && (
        <Card variant="outlined">
          <CardContent>
            <Typography fontWeight={600}>No projects yet</Typography>
            <Typography variant="body2" color="text.secondary" sx={{ mt: 1 }}>
              Create a project to start adding tasks and collaborating.
            </Typography>
            <Button sx={{ mt: 2 }} variant="outlined" onClick={() => setOpen(true)}>
              Create your first project
            </Button>
          </CardContent>
        </Card>
      )}
      <Box
        display="grid"
        gridTemplateColumns={{ xs: "1fr", sm: "repeat(2, 1fr)", md: "repeat(3, 1fr)" }}
        gap={2}
      >
        {(projects || []).map((p) => (
          <Card key={p.id} variant="outlined">
            <CardActionArea component={RouterLink} to={`/projects/${p.id}`}>
              <CardContent>
                <Typography variant="h6" fontWeight={600}>
                  {p.name}
                </Typography>
                {p.description && (
                  <Typography variant="body2" color="text.secondary" sx={{ mt: 1 }} noWrap>
                    {p.description}
                  </Typography>
                )}
              </CardContent>
            </CardActionArea>
          </Card>
        ))}
      </Box>

      <Dialog open={open} onClose={() => !creating && setOpen(false)} fullWidth maxWidth="sm">
        <DialogTitle>New project</DialogTitle>
        <DialogContent>
          <Stack spacing={2} sx={{ mt: 1 }}>
            {createError && <Alert severity="error">{createError}</Alert>}
            <TextField
              label="Name"
              fullWidth
              required
              value={name}
              onChange={(e) => setName(e.target.value)}
            />
            <TextField
              label="Description"
              fullWidth
              multiline
              minRows={2}
              value={description}
              onChange={(e) => setDescription(e.target.value)}
            />
          </Stack>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setOpen(false)} disabled={creating}>
            Cancel
          </Button>
          <Button variant="contained" onClick={() => void createProject()} disabled={creating}>
            {creating ? "Creating…" : "Create"}
          </Button>
        </DialogActions>
      </Dialog>
    </Stack>
  );
}
