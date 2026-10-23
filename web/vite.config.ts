import { existsSync } from "node:fs";
import { relative } from "node:path";
import { fileURLToPath } from "node:url";
import babel from "@rolldown/plugin-babel";
import { storybookTest } from "@storybook/addon-vitest/vitest-plugin";
import tailwindcss from "@tailwindcss/vite";
import react, { reactCompilerPreset } from "@vitejs/plugin-react";
import { playwright } from "@vitest/browser-playwright";
import type { LoggerEvent } from "babel-plugin-react-compiler";
import type { Plugin } from "vite";
import { defineConfig } from "vitest/config";

const sourceRoot = fileURLToPath(new URL("./", import.meta.url));
const browserExecutable =
  process.env.PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH ??
  (existsSync("/run/current-system/sw/bin/chromium")
    ? "/run/current-system/sw/bin/chromium"
    : undefined);
const compiled = new Map<
  string,
  { file: string; name: string; line?: number; memoSlots: number }
>();
const skipped: { file: string; kind: string; line?: number; reason?: string }[] = [];
const emittedRuntime = new Set<string>();
function recordCompilation(filename: string | null, event: LoggerEvent) {
  if (!filename) {
    return;
  }
  const file = relative(sourceRoot, filename).replaceAll("\\", "/");
  if (!file.startsWith("src/") || file.includes(".stories.")) {
    return;
  }
  if (event.kind === "CompileSuccess") {
    const name = event.fnName ?? `<anonymous:${event.fnLoc?.start.line ?? 0}>`;
    compiled.set(`${file}:${name}`, {
      file,
      name,
      line: event.fnLoc?.start.line,
      memoSlots: event.memoSlots,
    });
  } else if (
    event.kind === "CompileSkip" ||
    event.kind === "CompileError" ||
    event.kind === "CompileDiagnostic" ||
    event.kind === "PipelineError"
  ) {
    skipped.push({
      file,
      kind: event.kind,
      line: event.fnLoc?.start.line,
      reason:
        event.kind === "CompileSkip"
          ? event.reason
          : "detail" in event
            ? JSON.stringify(event.detail)
            : event.data,
    });
  }
}
function compilerCoverage(): Plugin {
  return {
    name: "feedlr-compiler-coverage",
    apply: "build",
    enforce: "post",
    buildStart() {
      compiled.clear();
      skipped.length = 0;
      emittedRuntime.clear();
    },
    transform(code, id) {
      if (code.includes("react/compiler-runtime") && /\bimport\s*\{[^}]*\bc\s+as\b/.test(code)) {
        emittedRuntime.add(relative(sourceRoot, id.split("?")[0] ?? id).replaceAll("\\", "/"));
      }
    },
    generateBundle() {
      const critical = [
        "Button",
        "IconButton",
        "Field",
        "Input",
        "Switch",
        "Checkbox",
        "Dialog",
        "Menu",
        "Tabs",
        "YouTubePlayerView",
        "PodcastPlayerView",
      ];
      const entries = [...compiled.values()].sort((a, b) =>
        `${a.file}:${a.name}`.localeCompare(`${b.file}:${b.name}`),
      );
      const missing = critical.filter(
        (name) =>
          !entries.some(
            (entry) => entry.name === name && entry.memoSlots > 0 && emittedRuntime.has(entry.file),
          ),
      );
      if (missing.length > 0 || skipped.length > 0) {
        this.error(
          `React Compiler coverage missing for critical components: ${missing.join(", ")}. Check compiler diagnostics and emitted runtime imports. ${JSON.stringify(skipped)}`,
        );
      }
      this.emitFile({
        type: "asset",
        fileName: "compiler-report.json",
        source: JSON.stringify(
          {
            compiler: "babel-plugin-react-compiler",
            critical,
            compiled: entries,
            transformedModules: [...emittedRuntime].sort(),
            skipped,
          },
          null,
          2,
        ),
      });
    },
  };
}

export default defineConfig(({ command }) => ({
  base: command === "serve" ? "/" : "/ui-assets/",
  plugins: [
    react(),
    babel({ presets: [reactCompilerPreset({ logger: { logEvent: recordCompilation } })] }),
    tailwindcss(),
    compilerCoverage(),
  ],
  resolve: { alias: { "@": fileURLToPath(new URL("./src", import.meta.url)) } },
  optimizeDeps: { include: ["@simplewebauthn/browser", "@tanstack/react-router"] },
  build: {
    outDir: "../assets/ui",
    emptyOutDir: true,
    manifest: "manifest.json",
    rolldownOptions: {
      output: {
        entryFileNames: "assets/[name]-[hash:16].js",
        chunkFileNames: "assets/[name]-[hash:16].js",
        assetFileNames: "assets/[name]-[hash:16][extname]",
        codeSplitting: {
          groups: [
            {
              name: "react",
              test: /node_modules[\\/](react|react-dom|scheduler)[\\/]/,
              priority: 30,
            },
            { name: "data", test: /node_modules[\\/]@tanstack[\\/]/, priority: 20 },
            { name: "headless", test: /node_modules[\\/]@headlessui[\\/]/, priority: 15 },
            { name: "passkeys", test: /node_modules[\\/]@simplewebauthn[\\/]/, priority: 10 },
          ],
        },
      },
    },
  },
  server: {
    host: "127.0.0.1",
    port: 5173,
    strictPort: true,
    proxy: Object.fromEntries(
      ["/api", "/login/begin", "/login/finish", "/register", "/thumb", "/media", "/assets"].map(
        (path) => [
          path,
          { target: process.env.FEEDLR_GO_URL ?? "http://127.0.0.1:3000", changeOrigin: false },
        ],
      ),
    ),
  },
  test: {
    projects: [
      {
        extends: true,
        test: { name: "unit", environment: "node", include: ["src/**/*.test.{ts,tsx}"] },
      },
      {
        extends: true,
        plugins: [
          storybookTest({ configDir: fileURLToPath(new URL("./.storybook", import.meta.url)) }),
        ],
        test: {
          name: "storybook",
          browser: {
            enabled: true,
            provider: playwright({ launchOptions: { executablePath: browserExecutable } }),
            instances: [{ browser: "chromium" }],
          },
          setupFiles: [".storybook/vitest.setup.ts"],
        },
      },
    ],
  },
}));
