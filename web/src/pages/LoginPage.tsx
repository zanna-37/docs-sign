import { useState } from "react";
import { Link } from "react-router-dom";
import { api, ApiError } from "../api/client";
import { useAuth } from "../auth/AuthContext";
import type { User } from "../api/types";
import { AuthShell } from "../components/AuthShell";
import { Button, ErrorText, Field, Input } from "../components/ui";

export function LoginPage() {
  const { setUser } = useAuth();
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError("");
    setBusy(true);
    try {
      const res = await api.post<{ user: User }>("/login", {
        username,
        password,
      });
      setUser(res.user);
    } catch (err) {
      setError(
        err instanceof ApiError ? err.message : "Could not log in.",
      );
      setBusy(false);
    }
  };

  return (
    <AuthShell title="Log in">
      <form onSubmit={submit} className="space-y-4">
        <Field label="Username">
          <Input
            value={username}
            onChange={(e) => setUsername(e.target.value)}
            autoFocus
            autoComplete="username"
            required
          />
        </Field>
        <Field label="Password">
          <Input
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            autoComplete="current-password"
            required
          />
        </Field>
        <ErrorText>{error}</ErrorText>
        <Button type="submit" className="w-full" disabled={busy}>
          {busy ? "Logging in…" : "Log in"}
        </Button>
      </form>
      <p className="mt-4 text-center text-sm text-gray-500">
        Forgot your password?{" "}
        <Link to="/recover" className="font-medium text-blue-600">
          Use recovery code
        </Link>
      </p>
    </AuthShell>
  );
}
