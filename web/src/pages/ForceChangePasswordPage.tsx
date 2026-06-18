import { useState } from "react";
import { api, ApiError } from "../api/client";
import { useAuth } from "../auth/AuthContext";
import { AuthShell } from "../components/AuthShell";
import { RecoveryCodeDialog } from "../components/RecoveryCodeDialog";
import { Button, ErrorText, Field, Input } from "../components/ui";

// Shown when the session is flagged must-change-password (first login or after recovery).
export function ForceChangePasswordPage() {
  const { refresh, logout } = useAuth();
  const [password, setPassword] = useState("");
  const [confirm, setConfirm] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const [recoveryCode, setRecoveryCode] = useState("");

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError("");
    if (password.length < 8) {
      setError("Password must be at least 8 characters.");
      return;
    }
    if (password !== confirm) {
      setError("Passwords do not match.");
      return;
    }
    setBusy(true);
    try {
      const res = await api.post<{ recoveryCode?: string }>(
        "/account/password",
        { newPassword: password },
      );
      if (res.recoveryCode) {
        setRecoveryCode(res.recoveryCode);
      } else {
        await refresh();
      }
    } catch (err) {
      setError(
        err instanceof ApiError ? err.message : "Could not set password.",
      );
      setBusy(false);
    }
  };

  return (
    <AuthShell
      title="Set a new password"
      subtitle="You must choose a new password before continuing."
    >
      <form onSubmit={submit} className="space-y-4">
        <Field label="New password">
          <Input
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            autoFocus
            autoComplete="new-password"
            required
          />
        </Field>
        <Field label="Confirm password">
          <Input
            type="password"
            value={confirm}
            onChange={(e) => setConfirm(e.target.value)}
            autoComplete="new-password"
            required
          />
        </Field>
        <ErrorText>{error}</ErrorText>
        <Button type="submit" className="w-full" disabled={busy}>
          {busy ? "Saving…" : "Save password"}
        </Button>
      </form>
      <p className="mt-4 text-center text-sm text-gray-500">
        <button
          className="font-medium text-gray-500 hover:text-gray-700"
          onClick={() => void logout()}
        >
          Cancel and log out
        </button>
      </p>

      {recoveryCode && (
        <RecoveryCodeDialog
          code={recoveryCode}
          onClose={() => void refresh()}
        />
      )}
    </AuthShell>
  );
}
