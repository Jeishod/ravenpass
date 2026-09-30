import { execFileSync } from "node:child_process";
import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { defineConfig, type EnvironmentConfig } from "@rsbuild/core";
import { pluginReact } from "@rsbuild/plugin-react";
import { pluginTailwindcss } from "@rsbuild/plugin-tailwindcss";

type HostName = "desktop" | "mobile";

interface Host {
  port: number;
  window: Record<string, string>;
}

const hosts: Record<HostName, Host> = {
  desktop: {
    port: 9245,
    window: { index: "./src/main.tsx", confirm: "./src/confirm.tsx" },
  },
  mobile: {
    port: 9246,
    window: { index: "./src/main.tsx" },
  },
};

const autofillEntry = { autofill: "./src/autofill.tsx" };

function required(name: string): string {
  const value = process.env[name];
  if (!value) {
    throw new Error(`${name} must be set; build through the app's Makefile`);
  }
  return value;
}

function hostName(): HostName {
  const name = required("RAVENPASS_HOST");
  if (name !== "desktop" && name !== "mobile") {
    throw new Error(`RAVENPASS_HOST must be desktop or mobile, not ${name}`);
  }
  return name;
}

function buildMetadata(): {
  version: string;
  build: string;
  released: boolean;
} {
  const version = readFileSync(
    new URL("../../version.txt", import.meta.url),
    "utf8",
  ).trim();
  const built = commit();
  return {
    version,
    build: built.slice(0, 7),
    released:
      git("rev-parse", "--verify", `refs/tags/v${version}^{commit}`) === built,
  };
}

function commit(): string {
  return process.env.RAVENPASS_COMMIT || git("rev-parse", "HEAD") || "unknown";
}

/** Empty where git fails, as outside a checkout. */
function git(...args: string[]): string {
  try {
    return execFileSync("git", args, {
      encoding: "utf8",
      stdio: ["ignore", "pipe", "ignore"],
    }).trim();
  } catch {
    return "";
  }
}

const devServer = process.env.RAVENPASS_DEV_SERVER === "1";
const host = hosts[hostName()];

const environments: Record<string, EnvironmentConfig> = {
  web: {
    source: { entry: host.window },
    output: { distPath: { root: resolve(required("RAVENPASS_OUT_DIR")) } },
  },
};
if (!devServer) {
  environments.autofill = {
    source: { entry: autofillEntry },
    output: {
      distPath: { root: resolve(required("RAVENPASS_AUTOFILL_OUT_DIR")) },
    },
  };
}

export default defineConfig({
  source: {
    define: { RAVENPASS_BUILD: JSON.stringify(buildMetadata()) },
  },
  plugins: [pluginReact(), pluginTailwindcss()],
  server: {
    host: "127.0.0.1",
    port: Number(process.env.WAILS_VITE_PORT) || host.port,
    strictPort: true,
  },
  dev: {
    // The phone reaches the dev server over `adb reverse`; its https page would otherwise ask for wss.
    client: {
      protocol: "ws",
      host: "127.0.0.1",
      port: "<port>",
    },
  },
  environments,
  html: {
    title: "Ravenpass",
    meta: {
      viewport: "width=device-width, initial-scale=1, viewport-fit=cover",
      // Sonner, Vaul, Radix and react-easy-crop insert <style> elements; the dev server's HMR socket is ws://127.0.0.1.
      "content-security-policy": {
        "http-equiv": "Content-Security-Policy",
        content: [
          "default-src 'self'",
          "script-src 'self'",
          "style-src 'self' 'unsafe-inline'",
          "img-src 'self' data:",
          devServer
            ? "connect-src 'self' ws://127.0.0.1:*"
            : "connect-src 'self'",
          "font-src 'self' data:",
          "object-src 'none'",
          // Android's window.wails reaches every frame, so no page may hold one, a data: frame included.
          "frame-src 'none'",
          "base-uri 'none'",
          "form-action 'none'",
        ].join("; "),
      },
    },
  },
  output: {
    assetPrefix: "./",
    cleanDistPath: !devServer,
    module: true,
  },
});
