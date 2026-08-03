import { defineConfig } from "vitest/config";
import react from "@vitejs/plugin-react";

// https://vite.dev/config/
export default defineConfig({
  plugins: [react()],
  server: {
    // Dev-only: the built app is served same-origin by Go in production
    // (M1.6: "build statique servi par Go"), so this proxy exists purely
    // so `npm run dev` can talk to a real backend on :8080 without CORS
    // (engine/api's DevCORS flag is the other half of that story, for
    // anyone who prefers running the API with permissive CORS instead).
    proxy: {
      "/api": {
        target: "http://127.0.0.1:8080",
        changeOrigin: true,
      },
    },
  },
  test: {
    environment: "jsdom",
    setupFiles: ["./src/test/setup.ts"],
    globals: true,
  },
});
