import { useState } from "react";
import { api, ApiError } from "../api/client";
import { useAuth } from "../auth/AuthContext";
import { RecoveryCodeDialog } from "../components/RecoveryCodeDialog";
import { Button, Card, ErrorText, Field, Input } from "../components/ui";

export function AccountPage() {
  const { user } = useAuth();
  const [password, setPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [busy, setBusy] = useState(false);
  const [recoveryCode, setRecoveryCode] = useState("");

  const changePassword = async (e: React.FormEvent) => {
    e.preventDefault();
    setError("");
    setNotice("");
    if (password.length < 8) {
      setError("Password must be at least 8 characters.");
      return;
    }
    if (password !== confirmPassword) {
      setError("Passwords do not match.");
      return;
    }
    setBusy(true);
    try {
      await api.post("/account/password", { newPassword: password });
      setPassword("");
      setConfirmPassword("");
      setNotice("Password updated.");
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Update failed.");
    } finally {
      setBusy(false);
    }
  };

  const regenerate = async () => {
    if (
      !confirm(
        "Generate a new recovery code? Your old code will stop working immediately.",
      )
    )
      return;
    setError("");
    try {
      const res = await api.post<{ recoveryCode: string }>(
        "/account/recovery-code",
      );
      setRecoveryCode(res.recoveryCode);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed.");
    }
  };

  return (
    <div className="mx-auto max-w-xl space-y-6">
      <div>
        <h1 className="text-xl font-semibold text-gray-900">Account</h1>
        <p className="text-sm text-gray-500">
          Signed in as <strong>{user?.username}</strong>
          {user?.isAdmin ? " (admin)" : ""}.
        </p>
      </div>

      <Card className="space-y-4 p-6">
        <h2 className="font-medium text-gray-900">Change password</h2>
        <form onSubmit={changePassword} className="space-y-4">
          <Field label="New password">
            <Input
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              autoComplete="new-password"
            />
          </Field>
          <Field label="Confirm password">
            <Input
              type="password"
              value={confirmPassword}
              onChange={(e) => setConfirmPassword(e.target.value)}
              autoComplete="new-password"
            />
          </Field>
          {error && <ErrorText>{error}</ErrorText>}
          {notice && (
            <p className="rounded-lg bg-green-50 px-3 py-2 text-sm text-green-700">
              {notice}
            </p>
          )}
          <Button type="submit" disabled={busy}>
            {busy ? "Saving…" : "Update password"}
          </Button>
        </form>
      </Card>

      <Card className="space-y-3 p-6">
        <h2 className="font-medium text-gray-900">Recovery code</h2>
        <p className="text-sm text-gray-500">
          Generate a fresh one-time recovery code. This invalidates any previous
          code.
        </p>
        <Button variant="secondary" onClick={regenerate}>
          Generate new recovery code
        </Button>
      </Card>

      {recoveryCode && (
        <RecoveryCodeDialog
          code={recoveryCode}
          onClose={() => setRecoveryCode("")}
        />
      )}
    </div>
  );
}
