#!/usr/bin/env node
/**
 * Proto drift guard for the Node.js plugin SDK.
 *
 * The SDK loads its contracts from protos/ at runtime, so those copies can
 * silently drift from the canonical definitions the Go hosts compile in.
 * This script asserts byte-identity for every contract that has an
 * in-repo canonical source, and a minimal shape for health.proto (copied
 * from grpc/grpc, which is not vendored in this repo).
 *
 * Exit 0 = no drift; nonzero with a per-file report otherwise. Run via
 * `pnpm verify:proto` and from CI (ci-sdk-plugins-js).
 */

"use strict";

const fs = require("node:fs");
const path = require("node:path");

const sdkDir = path.resolve(__dirname, "..");
const repoRoot = path.resolve(sdkDir, "..", "..", "..");

/** Files that must stay byte-identical to their canonical source. */
const MIRRORS = [
  {
    copy: "protos/probeplugin.proto",
    canonical: "agents/synthetics/pluginhost/protocol/probeplugin.proto",
  },
  {
    copy: "protos/funcplugin.proto",
    canonical: "agents/sidecar/funcplugin/protocol/funcplugin.proto",
  },
  {
    copy: "protos/plugin_stdio.proto",
    canonical: "sdks/plugins/python/protos/plugin_stdio.proto",
  },
  {
    copy: "protos/plugin_controller.proto",
    canonical: "sdks/plugins/python/protos/plugin_controller.proto",
  },
];

/** Invariants for the health service copy (no in-repo canonical). */
const HEALTH_PROTO = "protos/health.proto";
const HEALTH_SHAPES = [
  /package grpc\.health\.v1;/,
  /service Health \{/,
  /rpc Check\(HealthCheckRequest\) returns \(HealthCheckResponse\);/,
  /rpc Watch\(HealthCheckRequest\) returns \(stream HealthCheckResponse\);/,
];

function fail(message) {
  process.stderr.write(`verify-proto-drift: ${message}\n`);
  process.exitCode = 1;
}

let drift = false;

for (const { copy, canonical } of MIRRORS) {
  const copyPath = path.join(sdkDir, copy);
  const canonicalPath = path.join(repoRoot, canonical);
  if (!fs.existsSync(canonicalPath)) {
    fail(`${canonical} not found (repo layout change?)`);
    drift = true;
    continue;
  }
  const a = fs.readFileSync(copyPath);
  const b = fs.readFileSync(canonicalPath);
  if (!a.equals(b)) {
    fail(`${copy} drifted from ${canonical}; copy the canonical file again`);
    drift = true;
  }
}

const health = fs.readFileSync(path.join(sdkDir, HEALTH_PROTO), "utf8");
for (const shape of HEALTH_SHAPES) {
  if (!shape.test(health)) {
    fail(`${HEALTH_PROTO} lost its ${shape} shape; restore the grpc/grpc copy`);
    drift = true;
  }
}

if (drift) {
  process.stderr.write("proto drift detected\n");
} else {
  process.stdout.write("proto drift check: OK (4 mirrors + health shape)\n");
}
