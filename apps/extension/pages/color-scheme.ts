import { ask } from "../src/messages.ts";

const dark = matchMedia("(prefers-color-scheme: dark)");

async function report(): Promise<void> {
  await ask({ kind: "color-scheme", scheme: dark.matches ? "dark" : "light" });
}

dark.addEventListener("change", report);
report();
