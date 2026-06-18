import { useEffect, useState } from "react";
import { api, ApiError } from "../api/client";
import { useAuth } from "../auth/AuthContext";
import type { AdminUser } from "../api/types";
import { Button, Card, ErrorText, Field, Input, Spinner } from "../components/ui";
import { formatDate } from "../lib/format";

export function AdminPage() {
  const { user: me } = useAuth();
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
      setError(err instanceof ApiError ? err.message : "Failed to load.");
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
      setError(err instanceof ApiError ? err.message : "Create failed.");
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
      setError(err instanceof ApiError ? err.message : "Failed.");
    }
  };

  const reset = async (u: AdminUser) => {
    if (
      !confirm(
        `Reset "${u.username}"? This PERMANENTLY destroys all of their encrypted documents and signatures, and issues a new temporary password.`,
      )
    )
      return;
    const tmp = prompt(
      "Enter a temporary password for the reset account (min 8 chars):",
    );
    if (!tmp) return;
    try {
      await api.post(`/admin/users/${u.id}/reset`, { tempPassword: tmp });
      await reload();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Reset failed.");
    }
  };

  const remove = async (u: AdminUser) => {
    if (
      !confirm(
        `Delete "${u.username}" and all of their encrypted content? This cannot be undone.`,
      )
    )
      return;
    try {
      await api.del(`/admin/users/${u.id}`);
      await reload();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Delete failed.");
    }
  };

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-xl font-semibold text-gray-900">Users</h1>
        <p className="text-sm text-gray-500">
          Create and manage accounts. New users set their own password (and get a
          recovery code) on first login.
        </p>
      </div>

      <ErrorText>{error}</ErrorText>

      <Card className="p-6">
        <h2 className="mb-4 font-medium text-gray-900">Add user</h2>
        <form
          onSubmit={create}
          className="grid grid-cols-1 gap-4 sm:grid-cols-2"
        >
          <Field label="Username">
            <Input
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              required
            />
          </Field>
          <Field label="Temporary password">
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
            Grant admin privileges
          </label>
          <div className="sm:col-span-2">
            <Button type="submit" disabled={busy}>
              {busy ? "Creating…" : "Create user"}
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
                <p className="flex items-center gap-2 font-medium text-gray-800">
                  {u.username}
                  {u.isAdmin && (
                    <span className="rounded bg-blue-50 px-1.5 py-0.5 text-xs font-medium text-blue-700">
                      admin
                    </span>
                  )}
                  {u.status === "disabled" && (
                    <span className="rounded bg-gray-100 px-1.5 py-0.5 text-xs font-medium text-gray-500">
                      disabled
                    </span>
                  )}
                  {u.mustChangePassword && (
                    <span className="rounded bg-amber-50 px-1.5 py-0.5 text-xs font-medium text-amber-700">
                      pending first login
                    </span>
                  )}
                </p>
                <p className="text-xs text-gray-400">
                  created {formatDate(u.createdAt)}
                </p>
              </div>
              {u.id !== me?.id && (
                <div className="flex shrink-0 gap-1">
                  <Button variant="ghost" onClick={() => toggleStatus(u)}>
                    {u.status === "active" ? "Disable" : "Enable"}
                  </Button>
                  <Button variant="ghost" onClick={() => reset(u)}>
                    Reset
                  </Button>
                  <Button variant="ghost" onClick={() => remove(u)}>
                    Delete
                  </Button>
                </div>
              )}
              {u.id === me?.id && (
                <span className="text-xs text-gray-400">you</span>
              )}
            </div>
          ))}
        </Card>
      )}
    </div>
  );
}
