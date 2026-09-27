import { readFileSync } from "node:fs";
import { exportMessages } from "../src/i18n/export.ts";

// Usage: node scripts/export-messages.ts [--values] <names.json of host string names to catalog keys>
const options = process.argv.slice(2);
const values = options.includes("--values");
const [path] = options.filter((option) => option !== "--values");
if (!path) {
  throw new Error("Name the JSON file of string names and catalog keys.");
}
const names: unknown = JSON.parse(readFileSync(path, "utf8"));
if (typeof names !== "object" || names === null || Array.isArray(names)) {
  throw new Error(`${path} is not a JSON object of string names and keys.`);
}
process.stdout.write(
  `${JSON.stringify(exportMessages(names as Record<string, unknown>, { values }), null, 2)}\n`,
);
