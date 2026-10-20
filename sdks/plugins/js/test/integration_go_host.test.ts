/**
 * Full host⇄plugin round trips: the REAL synthetics probe controller
 * (pluginhost.NewController, the host production code uses) drives the
 * compiled reference JS plugin through the real go-plugin handshake, via
 * the shared Go hostdriver program.
 *
 * Jest is only the runner and the toolchain provisioner — every assertion
 * lives in sdks/plugins/hostdriver so Go and the JS suite agree on one
 * vocabulary. Skipped when no Go toolchain is installed (CI installs one).
 */

import { execFileSync } from "node:child_process";
import * as fs from "node:fs";
import * as os from "node:os";
import * as path from "node:path";

const sdkDir = path.resolve(__dirname, "..");
const repoRoot = path.resolve(sdkDir, "..", "..", "..");

const goBin = which("go");
const nodeBin = process.execPath;

function which(bin: string): string | null {
  const dirs = (process.env.PATH ?? "").split(path.delimiter);
  for (const dir of dirs) {
    const candidate = path.join(dir, bin);
    try {
      fs.accessSync(candidate, fs.constants.X_OK);
      return candidate;
    } catch {
      // keep looking
    }
  }
  return null;
}

const goAvailable = goBin !== null;

const describeHost = goAvailable ? describe : describe.skip;

describeHost("real hostdriver scenarios (node plugins)", () => {
  let driver: string;
  let probeExample: string;
  let funcExample: string;

  beforeAll(() => {
    expect(goBin).not.toBeNull();
    // Compile the JS examples fresh so the suite is self-contained.
    execFileSync(path.join(sdkDir, "node_modules", ".bin", "tsc"), ["-p", "tsconfig.json"], {
      cwd: sdkDir,
    });
    probeExample = path.join(sdkDir, "dist", "examples", "probe_echo.js");
    funcExample = path.join(sdkDir, "dist", "examples", "func_echo.js");
    expect(fs.existsSync(probeExample)).toBe(true);
    expect(fs.existsSync(funcExample)).toBe(true);

    driver = path.join(os.tmpdir(), "croupier-hostdriver-jest");
    execFileSync(
      goBin as string,
      ["build", "-o", driver, "./sdks/plugins/hostdriver"],
      { cwd: repoRoot },
    );
  }, 300_000);

  const drive = (mode: string, scenario: string, example: string): string =>
    execFileSync(
      driver,
      [
        "-mode", mode,
        "-scenario", scenario,
        "-command", nodeBin,
        "-args", example,
      ],
      {
        cwd: repoRoot,
        encoding: "utf8",
        timeout: 120_000,
        // go-plugin logs its own handshake trace; keep CI output readable.
        env: { ...process.env, GO_LOG_FORMAT: "standard", GO_LOG_LEVEL: "warn" },
      },
    );

  describe("probe mode (synthetics controller)", () => {
    it("probe-check-ok passes through the real handshake", () => {
      const out = drive("probe", "probe-check-ok", probeExample);
      expect(out).toContain("PASS probe-check-ok");
    });

    it("probe-check-fail surfaces the synthetic failure", () => {
      const out = drive("probe", "probe-check-fail", probeExample);
      expect(out).toContain("PASS probe-check-fail");
    });

    it("probe-version reports the plugin self-description", () => {
      const out = drive("probe", "probe-version", probeExample);
      expect(out).toContain("PASS probe-version");
    });
  });

  describe("func mode (sidecar function host)", () => {
    it("func-register registers the four declared functions", () => {
      const out = drive("func", "func-register", funcExample);
      expect(out).toContain("PASS func-register");
    });

    it("func-invoke carries the echo message back through the envelope", () => {
      const out = drive("func", "func-invoke", funcExample);
      expect(out).toContain("PASS func-invoke");
    });

    it("func-envelope keeps all five business failures on the closed set", () => {
      const out = drive("func", "func-envelope", funcExample);
      expect(out).toContain("PASS func-envelope");
    });

    it("func-timeout bounds the call and reports plugin_timeout", () => {
      const out = drive("func", "func-timeout", funcExample);
      expect(out).toContain("PASS func-timeout");
    });
  });

  describe("bench mode (function plugin IPC latency)", () => {
    it("bench-invoke prints inline and plugin-hop stats", () => {
      const out = execFileSync(
        driver,
        [
          "-mode", "funcbench",
          "-scenario", "bench-invoke",
          "-command", nodeBin,
          "-args", funcExample,
          "-n", "50",
          "-warmup", "5",
        ],
        {
          cwd: repoRoot,
          encoding: "utf8",
          timeout: 120_000,
          env: { ...process.env, GO_LOG_FORMAT: "standard", GO_LOG_LEVEL: "warn" },
        },
      );
      expect(out).toMatch(/^BENCH inline n=50 /m);
      expect(out).toMatch(/^BENCH plugin n=50 /m);
    });
  });

  it("refuses to serve without the host cookie", () => {
    const run = (): string =>
      execFileSync(nodeBin, [probeExample], {
        cwd: sdkDir,
        encoding: "utf8",
        timeout: 30_000,
      });
    expect(run).toThrow(/must be launched by a Croupier host/);
  });
});
