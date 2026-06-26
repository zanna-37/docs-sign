import { useState } from "react";
import { Link } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { api, errMessage } from "../api/client";
import { useAuth } from "../auth/AuthContext";
import type { User } from "../api/types";
import { AuthShell } from "../components/AuthShell";
import {
  Button,
  Checkbox,
  ErrorText,
  Field,
  Input,
  PasswordInput,
} from "../components/ui";

// localStorage key for the last-used username, pre-filled on the next visit.
const REMEMBERED_USER_KEY = "docs-sign:rememberedUsername";

export function LoginPage() {
  const { setUser } = useAuth();
  const { t } = useTranslation();
  const [username, setUsername] = useState(
    () => localStorage.getItem(REMEMBERED_USER_KEY) ?? "",
  );
  const [password, setPassword] = useState("");
  const [remember, setRemember] = useState(true);
  // Captured once at mount: with a remembered username, focus the password instead.
  const [prefilled] = useState(() => !!localStorage.getItem(REMEMBERED_USER_KEY));
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
      // Remember the username for next time, or forget it when unchecked.
      if (remember) localStorage.setItem(REMEMBERED_USER_KEY, username);
      else localStorage.removeItem(REMEMBERED_USER_KEY);
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
            autoFocus={!prefilled}
            autoComplete="username"
            required
          />
        </Field>
        <Field label={t("common.password")}>
          <PasswordInput
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            autoFocus={prefilled}
            autoComplete="current-password"
            required
          />
        </Field>
        <Checkbox
          label={t("login.remember")}
          checked={remember}
          onChange={(e) => setRemember(e.target.checked)}
        />
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
