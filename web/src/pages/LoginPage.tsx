import { useState } from "react";
import { Link } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { api, errMessage } from "../api/client";
import { useAuth } from "../auth/AuthContext";
import type { User } from "../api/types";
import { AuthShell } from "../components/AuthShell";
import { Button, ErrorText, Field, Input, PasswordInput } from "../components/ui";

export function LoginPage() {
  const { setUser } = useAuth();
  const { t } = useTranslation();
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  const submit = async (e: React.SyntheticEvent) => {
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
      setError(errMessage(err, t("login.failed")));
      setBusy(false);
    }
  };

  return (
    <AuthShell title={t("login.title")}>
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
            autoComplete="current-password"
            required
          />
        </Field>
        <ErrorText>{error}</ErrorText>
        <Button type="submit" className="w-full" disabled={busy}>
          {busy ? t("login.submitting") : t("login.submit")}
        </Button>
      </form>
      <p className="mt-4 text-center text-sm text-gray-500">
        {t("login.forgot")}{" "}
        <Link to="/recover" className="font-medium text-blue-600">
          {t("login.useRecovery")}
        </Link>
      </p>
    </AuthShell>
  );
}
