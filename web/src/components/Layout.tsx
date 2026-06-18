import { NavLink, Outlet, useNavigate } from "react-router-dom";
import { useAuth } from "../auth/AuthContext";
import { Button } from "./ui";

function navClass({ isActive }: { isActive: boolean }): string {
  return `rounded-lg px-3 py-1.5 text-sm font-medium transition ${
    isActive ? "bg-blue-50 text-blue-700" : "text-gray-600 hover:bg-gray-100"
  }`;
}

export function Layout() {
  const { user, logout } = useAuth();
  const navigate = useNavigate();

  const doLogout = async () => {
    await logout();
    navigate("/login", { replace: true });
  };

  return (
    <div className="min-h-full">
      <header className="border-b border-gray-200 bg-white">
        <div className="mx-auto flex max-w-6xl items-center justify-between gap-4 px-4 py-3">
          <div className="flex items-center gap-6">
            <span className="flex items-center gap-2 text-lg font-semibold text-gray-900">
              <span className="text-blue-600">✦</span> docs-sign
            </span>
            <nav className="flex items-center gap-1">
              <NavLink to="/documents" className={navClass}>
                Documents
              </NavLink>
              <NavLink to="/signatures" className={navClass}>
                Signatures
              </NavLink>
              <NavLink to="/history" className={navClass}>
                History
              </NavLink>
              {user?.isAdmin && (
                <NavLink to="/admin" className={navClass}>
                  Admin
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
              Log out
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
