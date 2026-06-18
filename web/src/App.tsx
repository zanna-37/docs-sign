import { Navigate, Route, Routes } from "react-router-dom";
import { useAuth } from "./auth/AuthContext";
import { Layout } from "./components/Layout";
import { Spinner } from "./components/ui";
import { SetupPage } from "./pages/SetupPage";
import { LoginPage } from "./pages/LoginPage";
import { RecoveryPage } from "./pages/RecoveryPage";
import { ForceChangePasswordPage } from "./pages/ForceChangePasswordPage";
import { DocumentsPage } from "./pages/DocumentsPage";
import { SignaturesPage } from "./pages/SignaturesPage";
import { EditorPage } from "./pages/EditorPage";
import { TrashPage } from "./pages/TrashPage";
import { AccountPage } from "./pages/AccountPage";
import { AdminPage } from "./pages/AdminPage";

function FullScreen({ children }: { children: React.ReactNode }) {
  return (
    <div className="flex min-h-full items-center justify-center p-6">
      {children}
    </div>
  );
}

export function App() {
  const { loading, needsSetup, user } = useAuth();

  if (loading) {
    return (
      <FullScreen>
        <Spinner className="h-8 w-8" />
      </FullScreen>
    );
  }

  if (needsSetup) {
    return <SetupPage />;
  }

  if (!user) {
    return (
      <Routes>
        <Route path="/login" element={<LoginPage />} />
        <Route path="/recover" element={<RecoveryPage />} />
        <Route path="*" element={<Navigate to="/login" replace />} />
      </Routes>
    );
  }

  if (user.mustChangePassword) {
    return <ForceChangePasswordPage />;
  }

  return (
    <Routes>
      <Route element={<Layout />}>
        <Route index element={<Navigate to="/documents" replace />} />
        <Route path="/documents" element={<DocumentsPage />} />
        <Route path="/documents/:id/sign" element={<EditorPage />} />
        <Route path="/signatures" element={<SignaturesPage />} />
        <Route path="/trash" element={<TrashPage />} />
        <Route path="/account" element={<AccountPage />} />
        {user.isAdmin && <Route path="/admin" element={<AdminPage />} />}
        <Route path="*" element={<Navigate to="/documents" replace />} />
      </Route>
    </Routes>
  );
}
