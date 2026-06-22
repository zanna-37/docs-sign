import { useState } from "react";
import { useTranslation } from "react-i18next";
import { api, errMessage } from "../api/client";
import { useAuth } from "../auth/AuthContext";
import { validateNewPassword } from "../lib/validation";
import { AuthShell } from "../components/AuthShell";
import { RecoveryCodeDialog } from "../components/RecoveryCodeDialog";
import { Button, ErrorText, Field, Input, PasswordInput } from "../components/ui";

export function SetupPage() {
  const { refresh } = useAuth();
  const { t } = useTranslation();
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [confirm, setConfirm] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const [recoveryCode, setRecoveryCode] = useState("");

  const submit = async (e: React.SyntheticEvent) => {
    e.preventDefault();
    setError("");
    const invalid = validateNewPassword(password, confirm);
    if (invalid) {
      setError(t(invalid));
      return;
    }
    setBusy(true);
    try {
      const res = await api.post<{ recoveryCode: string }>("/setup", {
        username,
        password,
      });
      setRecoveryCode(res.recoveryCode);
    } catch (err) {
      setError(errMessage(err, t("setup.failed")));
    } finally {
      setBusy(false);
    }
  };

  return (
    <AuthShell title={t("setup.title")} subtitle={t("setup.subtitle")}>
      <form onSubmit={submit} className="space-y-4">
        <Field label={t("common.username")}>
          <Input
            value={username}
            onChange={(e) => setUsername(e.target.value)}
            autoFocus
            autoComplete="username"
            required
          />
        </Field>
        <Field label={t("common.password")}>
          <PasswordInput
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            autoComplete="new-password"
            required
          />
        </Field>
        <Field label={t("common.confirmPassword")}>
          <PasswordInput
            value={confirm}
            onChange={(e) => setConfirm(e.target.value)}
            autoComplete="new-password"
            required
          />
        </Field>
        <ErrorText>{error}</ErrorText>
        <Button type="submit" className="w-full" disabled={busy}>
          {busy ? t("setup.creating") : t("setup.create")}
        </Button>
      </form>

      {recoveryCode && (
        <RecoveryCodeDialog code={recoveryCode} onClose={() => void refresh()} />
      )}
    </AuthShell>
  );
}
