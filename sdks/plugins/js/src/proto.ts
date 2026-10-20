/** Dynamic protobuf loading for the plugin IPC contracts.
 *
 * The contracts are loaded from the .proto copies shipped in protos/ at
 * runtime (no codegen step): probeplugin.proto / plugin_stdio.proto /
 * plugin_controller.proto are the wire truth and scripts/verify-proto-drift.js
 * keeps them byte-identical to the canonical Go sources.
 *
 * proto-loader's loadSync returns a FLAT package definition keyed by fully
 * qualified names; this module picks the pieces the SDK registers and fails
 * loudly when a contract shape changes underneath it.
 */

import * as fs from "node:fs";
import * as path from "node:path";
import * as grpc from "@grpc/grpc-js";
import { loadSync } from "@grpc/proto-loader";
import type { PackageDefinition } from "@grpc/proto-loader";

/** Wire shape of one host->plugin check (snake_case -> camelCase). */
export interface CheckRequestShape {
  target: string;
  timeoutMs: number;
  payloadJson: string;
}

/** Wire shape of one plugin->host probe outcome. */
export interface CheckResultShape {
  ok: boolean;
  status: number;
  latencyMs: number;
  error: string;
  detail: string;
  atUnixMs: number;
}

export interface DescribeResponseShape {
  name: string;
  version: string;
}

/** The loaded service definitions the SDK registers. */
export interface ProtoModules {
  probeService: grpc.ServiceDefinition;
  controllerService: grpc.ServiceDefinition;
  stdioService: grpc.ServiceDefinition;
  healthService: grpc.ServiceDefinition;
  /** grpc.health.v1.HealthCheckResponse.SERVING (numeric enum value). */
  servingStatus: number;
}

const NAMES = {
  probeService: "croupier.probeplugin.v1.ProbePlugin",
  stdioService: "plugin.GRPCStdio",
  controllerService: "plugin.GRPCController",
  healthService: "grpc.health.v1.Health",
} as const;

const PROTO_FILES = [
  "probeplugin.proto",
  "plugin_stdio.proto",
  "plugin_controller.proto",
  "health.proto",
];

/** Locate protos/ from a built or a source checkout. */
export function resolveProtosDir(startDir: string): string {
  const candidates = [
    path.join(startDir, "..", "protos"),
    path.join(startDir, "..", "..", "protos"),
    path.join(startDir, "protos"),
  ];
  for (const dir of candidates) {
    const resolved = path.resolve(dir);
    if (fs.existsSync(path.join(resolved, "probeplugin.proto"))) {
      return resolved;
    }
  }
  throw new Error(
    "croupier plugin SDK: protos/probeplugin.proto not found relative to " +
      path.resolve(startDir),
  );
}

let cached: ProtoModules | null = null;

/** Load the plugin contracts once and cache the descriptors. */
export function loadProtos(protosDir?: string): ProtoModules {
  if (cached !== null) {
    return cached;
  }
  const dir = protosDir ?? resolveProtosDir(__dirname);
  const def: PackageDefinition = loadSync(PROTO_FILES, {
    includeDirs: [dir],
    keepCase: false,
    // protobufjs compares against the Number constructor at conversion time
    // (longs === Number), so this must be the global, not a string.
    longs: Number,
    defaults: true,
  });

  const service = (name: string): grpc.ServiceDefinition => {
    const entry = def[name];
    if (entry === undefined || isTypeDefinition(entry)) {
      throw new Error(`croupier plugin SDK: service ${name} missing from protos`);
    }
    return entry as grpc.ServiceDefinition;
  };

  // ServingStatus is a nested enum, so the flat definition carries no entry
  // for it; SERVING=1 is fixed by grpc.health.v1 (guarded by protos/health.proto
  // and asserted in test/probeplugin.test.ts).
  const servingStatus = 1;

  cached = {
    probeService: service(NAMES.probeService),
    controllerService: service(NAMES.controllerService),
    stdioService: service(NAMES.stdioService),
    healthService: service(NAMES.healthService),
    servingStatus,
  };
  return cached;
}

/** Message/enum entries carry format+type; services map methods directly. */
function isTypeDefinition(entry: unknown): boolean {
  if (typeof entry !== "object" || entry === null) {
    return true;
  }
  return "format" in entry && "type" in entry;
}
