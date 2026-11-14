import { defineConfig } from "mygo-cli";

export default defineConfig({
  name: "U Store",
  slug: "universal-store",
  identifier: "com.universalstore.app",
  version: "0.1.0",
  license: "MIT",
  copyright: "© U Store contributors",
  linux: { comment: "A universal software store that works across every Linux distribution.", categories: ["PackageManager", "System"] },
  // `mygo dev` runs devCommand and loads devUrl; `mygo build` runs
  // buildCommand and embeds frontendDist into the app.
  devUrl: "http://localhost:5173",
  devCommand: "npm run dev:web",
  buildCommand: "npm run build:web",
  frontendDist: "dist",
  bindings: "src/mygo.ts",
  out: "build",
});
