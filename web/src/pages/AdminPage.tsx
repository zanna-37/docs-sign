import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { api, ApiError } from "../api/client";
import { useAuth } from "../auth/AuthContext";
import type { AdminUser } from "../api/types";
import { Button, Card, ErrorText, Field, Input, Spinner } from "../components/ui";
import { useDialog } from "../components/Dialog";
import { formatDate } from "../lib/format";

export function AdminPage() {
  const { user: me } = useAuth();
  const { t } = useTranslation();
  const dialog = useDialog();
  const [users, setUsers] = useState<AdminUser[] | null>(null);
  const [error, setError] = useState("");

  const [username, setUsername] = useState("");
  const [tempPassword, setTempPassword] = useState("");
  const [isAdmin, setIsAdmin] = useState(false);
  const [busy, setBusy] = useState(false);

  const reload = async () => {
    try {
      const res = await api.get<{ users: AdminUser[] }>("/admin/users");
      setUsers(res.users ?? []);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t("common.failedLoad"));
    }
  };

  useEffect(() => {
    void reload();
  }, []);

  const create = async (e: React.FormEvent) => {
    e.preventDefault();
    setError("");
    setBusy(true);
    try {
      await api.post("/admin/users", { username, tempPassword, isAdmin });
      setUsername("");
      setTempPassword("");
      setIsAdmin(false);
      await reload();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t("admin.createFailed"));
    } finally {
      setBusy(false);
    }
  };

  const toggleStatus = async (u: AdminUser) => {
    const status = u.status === "active" ? "disabled" : "active";
    try {
      await api.post(`/admin/users/${u.id}/status`, { status });
      await reload();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t("account.updateFailed"));
    }
  };

  const reset = async (u: AdminUser) => {
    if (
      !(await dialog.confirm({
        title: t("admin.confirmReset", { name: u.username }),
        confirmLabel: t("admin.reset"),
        danger: true,
      }))
    )
      return;
    const tmp = await dialog.prompt({
      title: t("admin.resetPrompt"),
      confirmLabel: t("admin.reset"),
    });
    if (!tmp) return;
    try {
      await api.post(`/admin/users/${u.id}/reset`, { tempPassword: tmp });
      await reload();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t("admin.resetFailed"));
    }
  };

  const remove = async (u: AdminUser) => {
    if (
      !(await dialog.confirm({
        title: t("admin.confirmDelete", { name: u.username }),
        confirmLabel: t("common.delete"),
        danger: true,
      }))
    )
      return;
    try {
      await api.del(`/admin/users/${u.id}`);
      await reload();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t("common.deleteFailed"));
    }
  };

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-xl font-semibold text-gray-900">
          {t("admin.title")}
        </h1>
        <p className="text-sm text-gray-500">{t("admin.subtitle")}</p>
      </div>

      <ErrorText>{error}</ErrorText>

      <Card className="p-6">
        <h2 className="mb-4 font-medium text-gray-900">{t("admin.addUser")}</h2>
        <form
          onSubmit={create}
          className="grid grid-cols-1 gap-4 sm:grid-cols-2"
        >
          <Field label={t("common.username")}>
            <Input
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              required
            />
          </Field>
          <Field label={t("admin.tempPassword")}>
            <Input
              value={tempPassword}
              onChange={(e) => setTempPassword(e.target.value)}
              required
            />
          </Field>
          <label className="flex items-center gap-2 text-sm text-gray-700">
            <input
              type="checkbox"
              checked={isAdmin}
              onChange={(e) => setIsAdmin(e.target.checked)}
            />
            {t("admin.grantAdmin")}
          </label>
          <div className="sm:col-span-2">
            <Button type="submit" disabled={busy}>
              {busy ? t("admin.creating") : t("admin.createUser")}
            </Button>
          </div>
        </form>
      </Card>

      {users === null ? (
        <Spinner />
      ) : (
        <Card className="divide-y divide-gray-100">
          {users.map((u) => (
            <div
              key={u.id}
              className="flex flex-wrap items-center justify-between gap-3 p-4"
            >
              <div className="min-w-0">
                <p className="flex flex-wrap items-center gap-2 font-medium text-gray-800">
                  {u.username}
                  {u.isAdmin && (
                    <span className="rounded bg-blue-50 px-1.5 py-0.5 text-xs font-medium text-blue-700">
                      {t("admin.badgeAdmin")}
                    </span>
                  )}
                  {u.status === "disabled" && (
                    <span className="rounded bg-gray-100 px-1.5 py-0.5 text-xs font-medium text-gray-500">
                      {t("admin.badgeDisabled")}
                    </span>
                  )}
                  {u.mustChangePassword && (
                    <span className="rounded bg-amber-50 px-1.5 py-0.5 text-xs font-medium text-amber-700">
                      {t("admin.badgePending")}
                    </span>
                  )}
                </p>
                <p className="text-xs text-gray-400">
                  {t("admin.created", { date: formatDate(u.createdAt) })}
                </p>
              </div>
              {u.id !== me?.id ? (
                <div className="flex shrink-0 gap-1">
                  <Button variant="ghost" onClick={() => toggleStatus(u)}>
                    {u.status === "active"
                      ? t("admin.disable")
                      : t("admin.enable")}
                  </Button>
                  <Button variant="ghost" onClick={() => reset(u)}>
                    {t("admin.reset")}
                  </Button>
                  <Button variant="ghost" onClick={() => remove(u)}>
                    {t("common.delete")}
                  </Button>
                </div>
              ) : (
                <span className="text-xs text-gray-400">{t("admin.you")}</span>
              )}
            </div>
          ))}
        </Card>
      )}
    </div>
  );
}
