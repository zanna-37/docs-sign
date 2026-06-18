import { useState } from "react";
import { useTranslation } from "react-i18next";
import { api, ApiError } from "../api/client";
import { useAuth } from "../auth/AuthContext";
import { AuthShell } from "../components/AuthShell";
import { RecoveryCodeDialog } from "../components/RecoveryCodeDialog";
import { Button, ErrorText, Field, Input } from "../components/ui";

// Shown when the session is flagged must-change-password (first login or after recovery).
export function ForceChangePasswordPage() {
  const { refresh, logout } = useAuth();
  const { t } = useTranslation();
  const [password, setPassword] = useState("");
  const [confirm, setConfirm] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const [recoveryCode, setRecoveryCode] = useState("");

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError("");
    if (password.length < 8) {
      setError(t("common.passwordTooShort"));
      return;
    }
    if (password !== confirm) {
      setError(t("common.passwordsDontMatch"));
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
      setError(err instanceof ApiError ? err.message : t("forceChange.failed"));
      setBusy(false);
    }
  };

  return (
    <AuthShell title={t("forceChange.title")} subtitle={t("forceChange.subtitle")}>
      <form onSubmit={submit} className="space-y-4">
        <Field label={t("common.newPassword")}>
          <Input
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            autoFocus
            autoComplete="new-password"
            required
          />
        </Field>
        <Field label={t("common.confirmPassword")}>
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
          {busy ? t("forceChange.submitting") : t("forceChange.submit")}
        </Button>
      </form>
      <p className="mt-4 text-center text-sm text-gray-500">
        <button
          className="font-medium text-gray-500 hover:text-gray-700"
          onClick={() => void logout()}
        >
          {t("forceChange.cancelLogout")}
        </button>
      </p>

      {recoveryCode && (
        <RecoveryCodeDialog code={recoveryCode} onClose={() => void refresh()} />
      )}
    </AuthShell>
  );
}
