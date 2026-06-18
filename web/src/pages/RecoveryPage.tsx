import { useState } from "react";
import { Link } from "react-router-dom";
import { api, ApiError } from "../api/client";
import { useAuth } from "../auth/AuthContext";
import type { User } from "../api/types";
import { AuthShell } from "../components/AuthShell";
import { Button, ErrorText, Field, Input } from "../components/ui";

export function RecoveryPage() {
  const { setUser } = useAuth();
  const [username, setUsername] = useState("");
  const [code, setCode] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError("");
    setBusy(true);
    try {
      const res = await api.post<{ user: User }>("/recover", {
        username,
        recoveryCode: code,
      });
      // The recovery session forces a password reset; App routes there next.
      setUser(res.user);
    } catch (err) {
      setError(
        err instanceof ApiError ? err.message : "Recovery failed.",
      );
      setBusy(false);
    }
  };

  return (
    <AuthShell
      title="Recover access"
      subtitle="Enter your recovery code. You will then set a new password."
    >
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
        <Field label="Recovery code">
          <Input
            value={code}
            onChange={(e) => setCode(e.target.value)}
            placeholder="XXXX-XXXX-XXXX-…"
            className="font-mono"
            required
          />
        </Field>
        <ErrorText>{error}</ErrorText>
        <Button type="submit" className="w-full" disabled={busy}>
          {busy ? "Verifying…" : "Recover"}
        </Button>
      </form>
      <p className="mt-4 text-center text-sm text-gray-500">
        <Link to="/login" className="font-medium text-blue-600">
          Back to login
        </Link>
      </p>
    </AuthShell>
  );
}
