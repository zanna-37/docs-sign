import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";

// The frontend is built into the Go embed directory so it ships inside the binary.
export default defineConfig({
  plugins: [react(), tailwindcss()],
  build: {
    outDir: "../internal/web/dist",
    // Don't empty the dir: it holds a committed placeholder.html that keeps the Go
    // //go:embed compiling even before the frontend is built. The Makefile clears stale
    // build artifacts (everything except the placeholder) before each build.
    emptyOutDir: false,
  },
  server: {
    // During `npm run dev`, proxy API calls to the Go server so the browser sees a
    // single origin (cookies work, no CORS needed).
    proxy: {
      "/api": "http://127.0.0.1:8080",
    },
  },
});
