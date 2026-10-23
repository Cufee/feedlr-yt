import { spawn } from "node:child_process";
import { fileURLToPath } from "node:url";

const root = fileURLToPath(new URL("../", import.meta.url));
const children = new Set();
let stopping = false;

function stop(code) {
  if (stopping) {
    return;
  }
  stopping = true;
  process.exitCode = code;
  for (const child of children) {
    signal(child, "SIGTERM");
  }
  const timeout = setTimeout(() => {
    for (const child of children) {
      signal(child, "SIGKILL");
    }
  }, 5000);
  timeout.unref();
}

function signal(child, name) {
  if (!child.pid) {
    return;
  }
  try {
    if (process.platform === "win32") {
      child.kill(name);
    } else {
      process.kill(-child.pid, name);
    }
  } catch (error) {
    if (error.code !== "ESRCH") {
      console.error(`Could not stop development process: ${error.message}`);
    }
  }
}

function start(command, args, env) {
  const child = spawn(command, args, {
    cwd: root,
    env,
    stdio: "inherit",
    detached: process.platform !== "win32",
  });
  children.add(child);
  return child;
}

function localURL(value, label) {
  const url = new URL(value);
  if (
    url.protocol !== "http:" ||
    !["localhost", "127.0.0.1", "[::1]"].includes(url.hostname) ||
    url.username ||
    url.password ||
    url.pathname !== "/" ||
    url.search ||
    url.hash
  ) {
    throw new Error(`${label} must be a local HTTP origin.`);
  }
  return url;
}

process.on("SIGINT", () => stop(130));
process.on("SIGTERM", () => stop(143));

try {
  const port = process.env.PORT ?? "3000";
  if (!/^\d+$/.test(port) || Number(port) < 1 || Number(port) > 65535) {
    throw new Error("PORT must be between 1 and 65535.");
  }
  const goURL = localURL(process.env.FEEDLR_GO_URL ?? `http://127.0.0.1:${port}`, "FEEDLR_GO_URL");
  if (Number(goURL.port || "80") !== Number(port)) {
    throw new Error("FEEDLR_GO_URL must use the Go server's PORT.");
  }
  const viteURL = localURL(
    process.env.FEEDLR_VITE_URL ?? "http://127.0.0.1:5173",
    "FEEDLR_VITE_URL",
  );
  const env = {
    ...process.env,
    PORT: port,
    FEEDLR_GO_URL: goURL.origin,
    FEEDLR_VITE_URL: viteURL.origin,
  };
  const prepare = start("npm", ["run", "build:vendor"], env);
  const prepared = await new Promise((resolve, reject) => {
    prepare.once("error", reject);
    prepare.once("exit", (code) => {
      children.delete(prepare);
      resolve(code);
    });
  });
  if (prepared !== 0) {
    stop(prepared ?? 1);
  } else if (!stopping) {
    console.info(`Open ${goURL.origin} for the app; Vite serves hot updates at ${viteURL.origin}.`);
    for (const child of [
      start("air", [], env),
      start(
        "npm",
        [
          "--prefix",
          "web",
          "run",
          "dev",
          "--",
          "--host",
          viteURL.hostname.replace(/^\[|\]$/g, ""),
          "--port",
          viteURL.port || "80",
        ],
        env,
      ),
    ]) {
      child.once("error", (error) => {
        console.error(`Could not start development process: ${error.message}`);
        stop(1);
      });
      child.once("exit", (code) => {
        children.delete(child);
        if (!stopping) {
          stop(code || 1);
        }
      });
    }
  }
} catch (error) {
  console.error(error.message);
  stop(1);
}
