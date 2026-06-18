import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useState,
  type ReactNode,
} from "react";
import { api, ApiError } from "../api/client";
import type { User } from "../api/types";
import { applyLanguage } from "../i18n";

interface AuthState {
  loading: boolean;
  needsSetup: boolean;
  user: User | null;
  setUser: (u: User | null) => void;
  refresh: () => Promise<void>;
  logout: () => Promise<void>;
}

const AuthCtx = createContext<AuthState | null>(null);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [loading, setLoading] = useState(true);
  const [needsSetup, setNeedsSetup] = useState(false);
  const [user, setUser] = useState<User | null>(null);

  const refresh = useCallback(async () => {
    setLoading(true);
    try {
      const status = await api.get<{ needsSetup: boolean }>("/setup/status");
      if (status.needsSetup) {
        setNeedsSetup(true);
        setUser(null);
        return;
      }
      setNeedsSetup(false);
      try {
        const me = await api.get<{ user: User }>("/me");
        setUser(me.user);
      } catch (e) {
        if (e instanceof ApiError && e.status === 401) {
          setUser(null);
        } else {
          throw e;
        }
      }
    } finally {
      setLoading(false);
    }
  }, []);

  const logout = useCallback(async () => {
    try {
      await api.post("/logout");
    } finally {
      setUser(null);
    }
  }, []);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  // Apply the signed-in user's language preference (empty follows the browser).
  useEffect(() => {
    applyLanguage(user?.language);
  }, [user]);

  return (
    <AuthCtx.Provider
      value={{ loading, needsSetup, user, setUser, refresh, logout }}
    >
      {children}
    </AuthCtx.Provider>
  );
}

export function useAuth(): AuthState {
  const ctx = useContext(AuthCtx);
  if (!ctx) throw new Error("useAuth must be used within AuthProvider");
  return ctx;
}
