import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

// `npm run dev` proxies the API to a local `tonton serve` on :8080.
export default defineConfig({
  plugins: [react()],
  server: { proxy: { "/api": "http://localhost:8080", "/healthz": "http://localhost:8080" } },
  build: { outDir: "dist", assetsInlineLimit: 0, sourcemap: false },
});
