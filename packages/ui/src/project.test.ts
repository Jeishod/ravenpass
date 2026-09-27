import assert from "node:assert/strict";
import test from "node:test";
import { problemReportAddress, releaseNotesAddress } from "./project.ts";

const released = { version: "0.2.0", build: "abc1234", released: true };
const development = { version: "0.2.0", build: "def5678", released: false };

test("a released build opens its version's release notes", () => {
  assert.equal(
    releaseNotesAddress(released),
    "https://github.com/dortanes/ravenpass/releases/tag/v0.2.0",
  );
});

test("a build no release names opens the list of releases", () => {
  assert.equal(
    releaseNotesAddress(development),
    "https://github.com/dortanes/ravenpass/releases",
  );
});

test("a problem report fills the bug form's technical fields", () => {
  assert.equal(
    problemReportAddress(released, {
      platform: "Android",
      osVersion: "15 (API 35)",
    }),
    "https://github.com/dortanes/ravenpass/issues/new?template=bug_report.yml&version=0.2.0&build=abc1234&app=Android+app&platform=Android+15+%28API+35%29",
  );
});

test("a problem report without the system's details carries the build alone", () => {
  const fields = new URL(problemReportAddress(development, null)).searchParams;
  assert.deepEqual(Object.fromEntries(fields), {
    template: "bug_report.yml",
    version: "0.2.0",
    build: "def5678",
  });
});

test("a platform the form does not offer fills no app", () => {
  const fields = new URL(
    problemReportAddress(released, { platform: "constructor", osVersion: "" }),
  ).searchParams;
  assert.equal(fields.get("app"), null);
  assert.equal(fields.get("platform"), "constructor");
});

test("a field's value cannot add another field", () => {
  const fields = new URL(
    problemReportAddress(
      { version: "1.0&title=x", build: "a#b", released: true },
      { platform: "macOS", osVersion: "15.4" },
    ),
  ).searchParams;
  assert.equal(fields.get("version"), "1.0&title=x");
  assert.equal(fields.get("build"), "a#b");
  assert.equal(fields.get("title"), null);
  assert.equal(fields.get("app"), "macOS app");
  assert.equal(fields.get("platform"), "macOS 15.4");
});
