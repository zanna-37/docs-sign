import { useState } from "react";
import { Link } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { api, errMessage } from "../api/client";
import { useAuth } from "../auth/AuthContext";
import type { User } from "../api/types";
import { AuthShell } from "../components/AuthShell";
import { Button, ErrorText, Field, Input } from "../components/ui";

export function RecoveryPage() {
  const { setUser } = useAuth();
  const { t } = useTranslation();
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
      setUser(res.user);
    } catch (err) {
      setError(errMessage(err, t("recovery.failed")));
      setBusy(false);
    }
  };

  return (
    <AuthShell title={t("recovery.title")} subtitle={t("recovery.subtitle")}>
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
        <Field label={t("recovery.code")}>
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
          {busy ? t("recovery.submitting") : t("recovery.submit")}
        </Button>
      </form>
      <p className="mt-4 text-center text-sm text-gray-500">
        <Link to="/login" className="font-medium text-blue-600">
          {t("recovery.back")}
        </Link>
      </p>
    </AuthShell>
  );
}
