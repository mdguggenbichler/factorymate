#!/usr/bin/env node
/**
 * Creates a draft GitHub release when VERSION at HEAD semver-increases vs last v* tag.
 * Run: node scripts/ci/create-draft-release.mjs [--dry-run]
 */
import { existsSync, readFileSync } from "node:fs";
import { spawnSync } from "node:child_process";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const root = join(dirname(fileURLToPath(import.meta.url)), "../..");
const VERSION_FILE = join(root, "VERSION");
const dryRun = process.argv.includes("--dry-run");

function run(cmd, args, options = {}) {
  return spawnSync(cmd, args, {
    cwd: root,
    encoding: "utf8",
    ...options,
  });
}

function fail(message, details = "") {
  console.error(message);
  if (details) {
    console.error(details.trim());
  }
  process.exit(1);
}

export function parseSemver(version) {
  const normalized = version.trim().replace(/^v/, "");
  const match = normalized.match(
    /^(\d+)\.(\d+)\.(\d+)(?:-([0-9A-Za-z.-]+))?(?:\+([0-9A-Za-z.-]+))?$/
  );
  if (!match) {
    throw new Error(`Invalid semver: ${version}`);
  }
  return {
    major: Number(match[1]),
    minor: Number(match[2]),
    patch: Number(match[3]),
    prerelease: match[4] ?? "",
    build: match[5] ?? "",
  };
}

export function compareSemver(left, right) {
  const a = parseSemver(left);
  const b = parseSemver(right);

  if (a.major !== b.major) {
    return a.major > b.major ? 1 : -1;
  }
  if (a.minor !== b.minor) {
    return a.minor > b.minor ? 1 : -1;
  }
  if (a.patch !== b.patch) {
    return a.patch > b.patch ? 1 : -1;
  }

  if (!a.prerelease && b.prerelease) {
    return 1;
  }
  if (a.prerelease && !b.prerelease) {
    return -1;
  }
  if (a.prerelease && b.prerelease) {
    return a.prerelease.localeCompare(b.prerelease);
  }

  return 0;
}

function readVersionAt(ref) {
  const result = run("git", ["show", `${ref}:VERSION`]);
  if (result.status === 0) {
    return result.stdout.trim();
  }

  if (ref === "HEAD" && existsSync(VERSION_FILE)) {
    return readFileSync(VERSION_FILE, "utf8").trim();
  }

  return null;
}

function getLastTag() {
  const result = run("git", ["tag", "--list", "v*", "--sort=-v:refname"]);
  if (result.status !== 0) {
    fail("Failed to list git tags", result.stderr);
  }

  const tags = result.stdout
    .split("\n")
    .map((tag) => tag.trim())
    .filter(Boolean);

  return tags[0] ?? null;
}

function gitLogSince(lastTag) {
  const args = lastTag
    ? ["log", "--pretty=format:%s (%h)", `${lastTag}..HEAD`]
    : ["log", "--pretty=format:%s (%h)", "HEAD"];
  const result = run("git", args);
  if (result.status !== 0) {
    fail("Failed to read git log", result.stderr);
  }
  return result.stdout.trim() || "No changes.";
}

function tagExists(tag) {
  const result = run("git", ["rev-parse", "--verify", `refs/tags/${tag}`]);
  return result.status === 0;
}

function releaseExists(tag) {
  const result = run("gh", ["release", "view", tag]);
  return result.status === 0;
}

function main() {
  const headVersion = readVersionAt("HEAD");
  if (!headVersion) {
    fail("VERSION missing at HEAD");
  }

  try {
    parseSemver(headVersion);
  } catch (error) {
    fail(error.message);
  }

  const lastTag = getLastTag();
  let previousVersion = "0.0.0";

  if (lastTag) {
    const versionAtTag = readVersionAt(lastTag);
    previousVersion = versionAtTag ?? lastTag.replace(/^v/, "");
    try {
      parseSemver(previousVersion);
    } catch (error) {
      fail(`Invalid semver on last tag ${lastTag}: ${error.message}`);
    }
  }

  if (compareSemver(headVersion, previousVersion) <= 0) {
    console.log(
      `Skip: VERSION ${headVersion} is not greater than ${previousVersion}` +
        (lastTag ? ` (last tag: ${lastTag})` : "")
    );
    process.exit(0);
  }

  const newTag = `v${headVersion}`;
  const title = `FactoryMate v${headVersion}`;
  const notes = gitLogSince(lastTag);

  if (dryRun) {
    console.log("DRY RUN — would create draft release:");
    console.log(`  tag: ${newTag}`);
    console.log(`  title: ${title}`);
    console.log(`  previous VERSION: ${previousVersion}`);
    console.log(`  last tag: ${lastTag ?? "(none)"}`);
    console.log("  notes preview:");
    console.log(notes.split("\n").slice(0, 5).join("\n") || "(empty)");
    process.exit(0);
  }

  if (!process.env.GH_TOKEN) {
    fail("GH_TOKEN is required outside --dry-run mode");
  }

  if (tagExists(newTag) || releaseExists(newTag)) {
    console.log(`Tag or release ${newTag} already exists; skipping`);
    process.exit(0);
  }

  const createTag = run("git", ["tag", newTag]);
  if (createTag.status !== 0) {
    fail(`Failed to create tag ${newTag}`, createTag.stderr);
  }

  const pushTag = run("git", ["push", "origin", newTag]);
  if (pushTag.status !== 0) {
    fail(`Failed to push tag ${newTag}`, pushTag.stderr);
  }

  const createRelease = run("gh", [
    "release",
    "create",
    newTag,
    "--draft",
    "--title",
    title,
    "--notes",
    notes,
  ]);

  if (createRelease.status !== 0) {
    fail(`Failed to create draft release for ${newTag}`, createRelease.stderr);
  }

  console.log(`Created draft release ${newTag}`);
}

const invokedDirectly =
  process.argv[1] &&
  fileURLToPath(import.meta.url) === join(process.argv[1]);

if (invokedDirectly) {
  main();
}
