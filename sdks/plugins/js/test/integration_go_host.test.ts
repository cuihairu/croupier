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

describeHost("real hostdriver probe scenarios (node plugin)", () => {
  let driver: string;
  let example: string;

  beforeAll(() => {
    expect(goBin).not.toBeNull();
    // Compile the JS example fresh so the suite is self-contained.
    execFileSync(path.join(sdkDir, "node_modules", ".bin", "tsc"), ["-p", "tsconfig.json"], {
      cwd: sdkDir,
    });
    example = path.join(sdkDir, "dist", "examples", "probe_echo.js");
    expect(fs.existsSync(example)).toBe(true);

    driver = path.join(os.tmpdir(), "croupier-hostdriver-jest");
    execFileSync(
      goBin as string,
      ["build", "-o", driver, "./sdks/plugins/hostdriver"],
      { cwd: repoRoot },
    );
  }, 300_000);

  const drive = (scenario: string): string =>
    execFileSync(
      driver,
      [
        "-mode", "probe",
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

  it("probe-check-ok passes through the real handshake", () => {
    const out = drive("probe-check-ok");
    expect(out).toContain("PASS probe-check-ok");
  });

  it("probe-check-fail surfaces the synthetic failure", () => {
    const out = drive("probe-check-fail");
    expect(out).toContain("PASS probe-check-fail");
  });

  it("probe-version reports the plugin self-description", () => {
    const out = drive("probe-version");
    expect(out).toContain("PASS probe-version");
  });

  it("refuses to serve without the host cookie", () => {
    const run = (): string =>
      execFileSync(nodeBin, [example], {
        cwd: sdkDir,
        encoding: "utf8",
        timeout: 30_000,
      });
    expect(run).toThrow(/must be launched by a Croupier host/);
  });
});
