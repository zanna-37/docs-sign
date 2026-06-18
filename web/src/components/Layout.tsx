import { useState } from "react";
import { NavLink, Outlet, useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { useAuth } from "../auth/AuthContext";
import { Button } from "./ui";

function deskClass({ isActive }: { isActive: boolean }): string {
  return `shrink-0 whitespace-nowrap rounded-lg px-3 py-1.5 text-sm font-medium transition ${
    isActive ? "bg-blue-50 text-blue-700" : "text-gray-600 hover:bg-gray-100"
  }`;
}

function mobileClass({ isActive }: { isActive: boolean }): string {
  return `rounded-lg px-3 py-2 text-sm font-medium transition ${
    isActive ? "bg-blue-50 text-blue-700" : "text-gray-700 hover:bg-gray-100"
  }`;
}

export function Layout() {
  const { user, logout } = useAuth();
  const navigate = useNavigate();
  const { t } = useTranslation();
  const [open, setOpen] = useState(false);

  const doLogout = async () => {
    setOpen(false);
    await logout();
    navigate("/login", { replace: true });
  };

  const links = [
    { to: "/documents", label: t("nav.documents") },
    { to: "/signatures", label: t("nav.signatures") },
    { to: "/trash", label: t("nav.trash") },
    ...(user?.isAdmin ? [{ to: "/admin", label: t("nav.admin") }] : []),
  ];

  return (
    <div className="min-h-full">
      <header className="border-b border-gray-200 bg-white">
        <div className="mx-auto max-w-6xl px-4">
          <div className="flex items-center justify-between gap-4 py-3">
            <div className="flex min-w-0 items-center gap-6">
              <NavLink
                to="/documents"
                onClick={() => setOpen(false)}
                className="flex shrink-0 items-center gap-2 text-lg font-semibold text-gray-900 hover:opacity-80"
              >
                <span className="text-blue-600">✦</span> docs-sign
              </NavLink>
              <nav className="hidden items-center gap-1 sm:flex">
                {links.map((l) => (
                  <NavLink key={l.to} to={l.to} className={deskClass}>
                    {l.label}
                  </NavLink>
                ))}
              </nav>
            </div>

            <div className="hidden items-center gap-3 sm:flex">
              <NavLink
                to="/account"
                className="max-w-[10rem] truncate text-sm font-medium text-gray-600 hover:text-gray-900"
              >
                {user?.username}
              </NavLink>
              <Button variant="secondary" onClick={doLogout}>
                {t("nav.logout")}
              </Button>
            </div>

            <button
              className="rounded-lg p-2 text-gray-700 hover:bg-gray-100 sm:hidden"
              aria-label="Menu"
              aria-expanded={open}
              onClick={() => setOpen((v) => !v)}
            >
              <svg
                viewBox="0 0 24 24"
                width="22"
                height="22"
                fill="none"
                stroke="currentColor"
                strokeWidth="2"
                strokeLinecap="round"
              >
                {open ? (
                  <path d="M6 6l12 12M18 6L6 18" />
                ) : (
                  <path d="M4 7h16M4 12h16M4 17h16" />
                )}
              </svg>
            </button>
          </div>

          {open && (
            <nav className="flex flex-col gap-1 border-t border-gray-100 py-2 sm:hidden">
              {links.map((l) => (
                <NavLink
                  key={l.to}
                  to={l.to}
                  className={mobileClass}
                  onClick={() => setOpen(false)}
                >
                  {l.label}
                </NavLink>
              ))}
              <NavLink
                to="/account"
                className={mobileClass}
                onClick={() => setOpen(false)}
              >
                {user?.username}
              </NavLink>
              <button
                className="rounded-lg px-3 py-2 text-left text-sm font-medium text-gray-700 hover:bg-gray-100"
                onClick={doLogout}
              >
                {t("nav.logout")}
              </button>
            </nav>
          )}
        </div>
      </header>
      <main className="mx-auto max-w-6xl px-4 py-8">
        <Outlet />
      </main>
    </div>
  );
}
