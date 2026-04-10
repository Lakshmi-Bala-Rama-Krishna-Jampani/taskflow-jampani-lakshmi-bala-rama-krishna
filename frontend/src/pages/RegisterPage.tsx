import { useState } from "react";
import { Link as RouterLink, Navigate, useNavigate } from "react-router-dom";
import {
  Alert,
  Box,
  Button,
  Container,
  Link,
  Paper,
  Stack,
  TextField,
  Typography,
} from "@mui/material";
import { useAuth } from "../context/AuthContext";

export default function RegisterPage() {
  const { register, token } = useAuth();
  const nav = useNavigate();
  const [name, setName] = useState("");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({});
  const [loading, setLoading] = useState(false);

  if (token) return <Navigate to="/" replace />;

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError(null);
    setFieldErrors({});
    const fe: Record<string, string> = {};
    if (!name.trim()) fe.name = "is required";
    if (!email.trim()) fe.email = "is required";
    if (password.length < 8) fe.password = "must be at least 8 characters";
    if (Object.keys(fe).length) {
      setFieldErrors(fe);
      return;
    }
    setLoading(true);
    try {
      await register(name.trim(), email.trim(), password);
      nav("/", { replace: true });
    } catch (err: unknown) {
      const e = err as { body?: { error?: string; fields?: Record<string, string> }; message?: string };
      if (e.body?.fields) setFieldErrors(e.body.fields);
      setError(e.body?.error || e.message || "Registration failed");
    } finally {
      setLoading(false);
    }
  }

  return (
    <Container maxWidth="sm" sx={{ py: 8 }}>
      <Paper sx={{ p: { xs: 3, sm: 4 } }}>
        <Typography variant="h5" fontWeight={700} gutterBottom>
          Create account
        </Typography>
        <Typography variant="body2" color="text.secondary" sx={{ mb: 3 }}>
          Already have an account?{" "}
          <Link component={RouterLink} to="/login">
            Sign in
          </Link>
        </Typography>
        {error && (
          <Alert severity="error" sx={{ mb: 2 }}>
            {error}
          </Alert>
        )}
        <Box component="form" onSubmit={onSubmit}>
          <Stack spacing={2}>
            <TextField
              label="Name"
              autoComplete="name"
              fullWidth
              required
              value={name}
              onChange={(e) => setName(e.target.value)}
              error={Boolean(fieldErrors.name)}
              helperText={fieldErrors.name}
            />
            <TextField
              label="Email"
              type="email"
              autoComplete="email"
              fullWidth
              required
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              error={Boolean(fieldErrors.email)}
              helperText={fieldErrors.email}
            />
            <TextField
              label="Password"
              type="password"
              autoComplete="new-password"
              fullWidth
              required
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              error={Boolean(fieldErrors.password)}
              helperText={fieldErrors.password || "At least 8 characters"}
            />
            <Button type="submit" variant="contained" size="large" disabled={loading}>
              {loading ? "Creating…" : "Create account"}
            </Button>
          </Stack>
        </Box>
      </Paper>
    </Container>
  );
}
