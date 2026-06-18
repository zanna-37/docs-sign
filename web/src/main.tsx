import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { createBrowserRouter, RouterProvider } from "react-router-dom";
import "./index.css";
import "./i18n";
import { AuthProvider } from "./auth/AuthContext";
import { DialogProvider } from "./components/Dialog";
import { App } from "./App";

// A data router (rather than <BrowserRouter>) so useBlocker works for the editor's
// unsaved-changes guard. App still renders its own <Routes> beneath this catch-all.
const router = createBrowserRouter([
  {
    path: "*",
    element: (
      <AuthProvider>
        <DialogProvider>
          <App />
        </DialogProvider>
      </AuthProvider>
    ),
  },
]);

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <RouterProvider router={router} />
  </StrictMode>,
);
