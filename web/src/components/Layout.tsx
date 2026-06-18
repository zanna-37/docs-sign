import { NavLink, Outlet, useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { useAuth } from "../auth/AuthContext";
import { Button } from "./ui";

function navClass({ isActive }: { isActive: boolean }): string {
  return `shrink-0 whitespace-nowrap rounded-lg px-3 py-1.5 text-sm font-medium transition ${
    isActive ? "bg-blue-50 text-blue-700" : "text-gray-600 hover:bg-gray-100"
  }`;
}

export function Layout() {
  const { user, logout } = useAuth();
  const navigate = useNavigate();
  const { t } = useTranslation();

  const doLogout = async () => {
    await logout();
    navigate("/login", { replace: true });
  };

  return (
    <div className="min-h-full">
      <header className="border-b border-gray-200 bg-white">
        <div className="mx-auto flex max-w-6xl flex-wrap items-center justify-between gap-x-4 gap-y-2 px-4 py-3">
          <div className="flex min-w-0 items-center gap-3 sm:gap-6">
            <NavLink
              to="/documents"
              className="flex shrink-0 items-center gap-2 text-lg font-semibold text-gray-900 hover:opacity-80"
            >
              <span className="text-blue-600">✦</span> docs-sign
            </NavLink>
            <nav className="flex items-center gap-1 overflow-x-auto">
              <NavLink to="/documents" className={navClass}>
                {t("nav.documents")}
              </NavLink>
              <NavLink to="/signatures" className={navClass}>
                {t("nav.signatures")}
              </NavLink>
              <NavLink to="/trash" className={navClass}>
                {t("nav.trash")}
              </NavLink>
              {user?.isAdmin && (
                <NavLink to="/admin" className={navClass}>
                  {t("nav.admin")}
                </NavLink>
              )}
            </nav>
          </div>
          <div className="flex items-center gap-3">
            <NavLink
              to="/account"
              className="text-sm font-medium text-gray-600 hover:text-gray-900"
            >
              {user?.username}
            </NavLink>
            <Button variant="secondary" onClick={doLogout}>
              {t("nav.logout")}
            </Button>
          </div>
        </div>
      </header>
      <main className="mx-auto max-w-6xl px-4 py-8">
        <Outlet />
      </main>
    </div>
  );
}
