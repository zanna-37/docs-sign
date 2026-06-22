import { useState } from "react";
import { useTranslation } from "react-i18next";
import { api, errMessage } from "../api/client";
import { useAuth } from "../auth/AuthContext";
import type { User } from "../api/types";
import { applyLanguage } from "../i18n";
import { validateNewPassword } from "../lib/validation";
import { useDialog } from "../components/Dialog";
import { RecoveryCodeDialog } from "../components/RecoveryCodeDialog";
import { Button, Card, ErrorText, Field, Input, PasswordInput } from "../components/ui";

function Notice({ children }: Readonly<{ children: React.ReactNode }>) {
  if (!children) return null;
  return (
    <p className="rounded-lg bg-green-50 px-3 py-2 text-sm text-green-700">
      {children}
    </p>
  );
}

export function AccountPage() {
  const { user, setUser } = useAuth();
  const { t } = useTranslation();
  const dialog = useDialog();

  const [username, setUsername] = useState(user?.username ?? "");
  const [usernameNotice, setUsernameNotice] = useState("");
  const [usernameError, setUsernameError] = useState("");

  const [password, setPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [pwError, setPwError] = useState("");
  const [pwNotice, setPwNotice] = useState("");
  const [busy, setBusy] = useState(false);

  const [recoveryCode, setRecoveryCode] = useState("");
  const [error, setError] = useState("");

  const changeUsername = async (e: React.SyntheticEvent) => {
    e.preventDefault();
    setUsernameError("");
    setUsernameNotice("");
    try {
      const res = await api.post<{ user: User }>("/account/username", {
        username,
      });
      setUser(res.user);
      setUsernameNotice(t("account.usernameUpdated"));
    } catch (err) {
      setUsernameError(
        errMessage(err, t("account.updateFailed")),
      );
    }
  };

  const changeLanguage = async (language: string) => {
    setError("");
    // Apply immediately for snappy feedback; the server persists it.
    applyLanguage(language);
    try {
      const res = await api.post<{ user: User }>("/account/language", {
        language,
      });
      setUser(res.user);
    } catch (err) {
      setError(errMessage(err, t("account.updateFailed")));
    }
  };

  const changePassword = async (e: React.SyntheticEvent) => {
    e.preventDefault();
    setPwError("");
    setPwNotice("");
    const invalid = validateNewPassword(password, confirmPassword);
    if (invalid) {
      setPwError(t(invalid));
      return;
    }
    setBusy(true);
    try {
      await api.post("/account/password", { newPassword: password });
      setPassword("");
      setConfirmPassword("");
      setPwNotice(t("account.passwordUpdated"));
    } catch (err) {
      setPwError(errMessage(err, t("account.updateFailed")));
    } finally {
      setBusy(false);
    }
  };

  const regenerate = async () => {
    if (
      !(await dialog.confirm({
        title: t("account.confirmRegenerate"),
        confirmLabel: t("account.generateRecovery"),
        danger: true,
      }))
    )
      return;
    setError("");
    try {
      const res = await api.post<{ recoveryCode: string }>(
        "/account/recovery-code",
      );
      setRecoveryCode(res.recoveryCode);
    } catch (err) {
      setError(errMessage(err, t("account.updateFailed")));
    }
  };

  return (
    <div className="mx-auto max-w-xl space-y-6">
      <div>
        <h1 className="text-xl font-semibold text-gray-900">
          {t("account.title")}
        </h1>
        <p className="text-sm text-gray-500">
          {t("account.signedInAs")} <strong>{user?.username}</strong>
          {user?.isAdmin ? ` (${t("account.admin")})` : ""}.
        </p>
      </div>

      <Card className="space-y-3 p-6">
        <h2 className="font-medium text-gray-900">{t("account.language")}</h2>
        <p className="text-sm text-gray-500">{t("account.languageBody")}</p>
        <select
          value={user?.language ?? ""}
          onChange={(e) => void changeLanguage(e.target.value)}
          className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm outline-none focus:border-blue-500 focus:ring-2 focus:ring-blue-100"
        >
          <option value="">{t("account.langAuto")}</option>
          <option value="en">{t("account.langEn")}</option>
          <option value="it">{t("account.langIt")}</option>
        </select>
      </Card>

      <Card className="space-y-4 p-6">
        <h2 className="font-medium text-gray-900">
          {t("account.changeUsername")}
        </h2>
        <form onSubmit={changeUsername} className="space-y-4">
          <Field label={t("common.username")}>
            <Input
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              autoComplete="username"
            />
          </Field>
          {usernameError && <ErrorText>{usernameError}</ErrorText>}
          <Notice>{usernameNotice}</Notice>
          <Button type="submit" disabled={username === user?.username}>
            {t("account.updateUsername")}
          </Button>
        </form>
      </Card>

      <Card className="space-y-4 p-6">
        <h2 className="font-medium text-gray-900">
          {t("account.changePassword")}
        </h2>
        <form onSubmit={changePassword} className="space-y-4">
          <Field label={t("common.newPassword")}>
            <PasswordInput
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              autoComplete="new-password"
            />
          </Field>
          <Field label={t("common.confirmPassword")}>
            <PasswordInput
              value={confirmPassword}
              onChange={(e) => setConfirmPassword(e.target.value)}
              autoComplete="new-password"
            />
          </Field>
          {pwError && <ErrorText>{pwError}</ErrorText>}
          <Notice>{pwNotice}</Notice>
          <Button type="submit" disabled={busy}>
            {busy ? t("account.saving") : t("account.updatePassword")}
          </Button>
        </form>
      </Card>

      <Card className="space-y-3 p-6">
        <h2 className="font-medium text-gray-900">
          {t("account.recoveryCode")}
        </h2>
        <p className="text-sm text-gray-500">{t("account.recoveryBody")}</p>
        {error && <ErrorText>{error}</ErrorText>}
        <Button variant="secondary" onClick={regenerate}>
          {t("account.generateRecovery")}
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
