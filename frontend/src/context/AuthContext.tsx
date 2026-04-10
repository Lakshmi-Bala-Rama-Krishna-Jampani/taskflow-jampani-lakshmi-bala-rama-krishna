import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from "react";
import { api, getStoredToken, setOnUnauthorized, setStoredToken } from "../api/client";
import type { User } from "../types";

interface AuthState {
  user: User | null;
  token: string | null;
  loading: boolean;
}

interface AuthContextValue extends AuthState {
  login: (email: string, password: string) => Promise<void>;
  register: (name: string, email: string, password: string) => Promise<void>;
  logout: () => void;
}

const AuthContext = createContext<AuthContextValue | null>(null);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null);
  const [token, setToken] = useState<string | null>(() => getStoredToken());
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    const t = getStoredToken();
    if (!t) {
      setLoading(false);
      return;
    }
    setToken(t);
    try {
      const raw = localStorage.getItem("taskflow_user");
      if (raw) setUser(JSON.parse(raw) as User);
    } catch {
      /* ignore */
    }
    setLoading(false);
  }, []);

  useEffect(() => {
    const handleUnauthorized = () => {
      setStoredToken(null);
      setToken(null);
      try {
        localStorage.removeItem("taskflow_user");
      } catch {
        /* ignore */
      }
      setUser(null);
      const path = window.location.pathname;
      if (path !== "/login" && path !== "/register") {
        window.location.assign("/login?reason=session_expired");
      }
    };
    setOnUnauthorized(handleUnauthorized);
    return () => setOnUnauthorized(null);
  }, []);

  const persistUser = useCallback((u: User | null) => {
    setUser(u);
    try {
      if (u) localStorage.setItem("taskflow_user", JSON.stringify(u));
      else localStorage.removeItem("taskflow_user");
    } catch {
      /* ignore */
    }
  }, []);

  const login = useCallback(
    async (email: string, password: string) => {
      const res = await api.login({ email, password });
      setStoredToken(res.token);
      setToken(res.token);
      persistUser(res.user);
    },
    [persistUser],
  );

  const register = useCallback(
    async (name: string, email: string, password: string) => {
      const res = await api.register({ name, email, password });
      setStoredToken(res.token);
      setToken(res.token);
      persistUser(res.user);
    },
    [persistUser],
  );

  const logout = useCallback(() => {
    setStoredToken(null);
    setToken(null);
    persistUser(null);
  }, [persistUser]);

  const value = useMemo<AuthContextValue>(
    () => ({
      user,
      token,
      loading,
      login,
      register,
      logout,
    }),
    [user, token, loading, login, register, logout],
  );

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth(): AuthContextValue {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error("useAuth must be used within AuthProvider");
  return ctx;
}

export function useIsAuthenticated(): boolean {
  const { token } = useAuth();
  return Boolean(token);
}
