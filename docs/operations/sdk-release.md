---
title: SDK Package Release
icon: send
order: 8
category:
  - Operations Manual
tag:
  - Release
  - SDK
---

# SDK Package Release

The six-language SDKs are published to each language's official package registry. **One standalone manual workflow per language** (`.github/workflows/release-sdk-<lang>.yml`, `workflow_dispatch` only, not auto-triggered on push) — a failed run can be re-sent independently without affecting the others; just re-run that one alone.

```text
Actions → Release SDK — Python / JS / Go / Java / C# / C++ → Run workflow
```

## 1. Workflow Inputs (identical across the six workflows)

| Input     | Type | Description                                                                                                                                                   |
| --------- | ---- | ------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `version` | Text | Release version. **Empty = latest release tag** (e.g. `v0.1.1`); can be specified explicitly, `v0.2.0` and `0.2.0` are equivalent spellings (auto-normalized) |

| Workflow             | Target                                   | Required Secrets                                  |
| -------------------- | ---------------------------------------- | ------------------------------------------------- |
| Release SDK — Python | PyPI                                     | `PYPI_API_TOKEN`                                  |
| Release SDK — JS     | npm                                      | `NPM_TOKEN`                                       |
| Release SDK — Go     | Go module tag (`sdks/go/vX.Y.Z`)         | None                                              |
| Release SDK — Java   | GitHub Packages (optional Maven Central) | None (Central requires `MAVEN_CENTRAL_*`+`GPG_*`) |
| Release SDK — C#     | NuGet                                    | `NUGET_API_KEY`                                   |
| Release SDK — C++    | GitHub Release source package            | None                                              |

## 2. Release Targets and Required Secrets

The version number is only injected into build artifacts and never written back to the repository (each language's manifest file stays unchanged).

| Language        | Target registry                  | Package name / coordinates             | Required Secrets                                                                                                | Behavior when unconfigured                                |
| --------------- | -------------------------------- | -------------------------------------- | --------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------- |
| Python          | [PyPI](https://pypi.org)         | `croupier` (pyproject)                 | `PYPI_API_TOKEN`                                                                                                | Job fails explicitly, naming the missing credential       |
| JS              | [npm](https://www.npmjs.com)     | `croupier-js-sdk`                      | `NPM_TOKEN`                                                                                                     | Same as above                                             |
| Go              | Go module proxy                  | `github.com/cuihairu/croupier/sdks/go` | None (tags `sdks/go/vX.Y.Z`)                                                                                    | No credentials needed; can be trial-run directly          |
| Java            | GitHub Packages (main repo)      | `io.github.cuihairu.croupier:*`        | None (built-in `GITHUB_TOKEN`)                                                                                  | No extra credentials needed                               |
| Java (optional) | Maven Central (OSSRH)            | Same as above                          | `MAVEN_CENTRAL_USERNAME`, `MAVEN_CENTRAL_PASSWORD`, `GPG_KEY_ID`, `GPG_PRIVATE_KEY`, `GPG_PRIVATE_KEY_PASSWORD` | Without them, only GitHub Packages is published, no error |
| C#              | [NuGet](https://www.nuget.org)   | `Croupier.Sdk`                         | `NUGET_API_KEY`                                                                                                 | Job fails explicitly, naming the missing credential       |
| C++             | GitHub Release source attachment | `croupier-cpp-sdk-<version>.tar.gz`    | None (built-in `GITHUB_TOKEN`)                                                                                  | No credentials needed; can be trial-run directly          |

## 3. Secrets Setup Steps

Repository path: **Settings → Secrets and variables → Actions → New repository secret**.

### PYPI_API_TOKEN (Python)

1. Register/log in to a PyPI account; before the first publish of the target package name, its availability must be confirmed on PyPI;
2. Account settings → API tokens → Add API token;
3. Choose scope Entire account (first time) or limit it to the `croupier` package; the token looks like `«redacted:pypi-…»...`;
4. Set Name to `PYPI_API_TOKEN`.

### NPM_TOKEN (JS)

1. Register/log in to npmjs.com (publishing `croupier-js-sdk` requires owning that package name, or choose a scoped name `@<scope>/croupier-js-sdk`);
2. Access Tokens → Generate New Token → **Granular Access Token** (check Read and write);
3. Set Name to `NPM_TOKEN`. Inside the workflow it takes effect as `NODE_AUTH_TOKEN`.

### NUGET_API_KEY (C#)

1. Register/log in to nuget.org (Microsoft account);
2. Account → API Keys → Create; set Package scope to `Croupier.Sdk`, Expiration per policy;
3. Set Name to `NUGET_API_KEY`.

### Maven Central (Java, optional)

The namespace `io.github.cuihairu` corresponds to the GitHub namespace, via the Sonatype Central Portal:

1. Log in to the Central Portal (central.sonatype.com) with a GitHub account and verify the `io.github.cuihairu` namespace;
2. Generate a Portal user token (Account → Generate User Token) → configure `MAVEN_CENTRAL_USERNAME` / `MAVEN_CENTRAL_PASSWORD` respectively;
3. GPG signing key: after `gpg --full-generate-key` (RSA 4096), export —
   - Key ID (8-digit or the full 40-character hex) → `GPG_KEY_ID`
   - The full output of `gpg --export-secret-keys --armor <KEYID>` → `GPG_PRIVATE_KEY`
   - The key passphrase → `GPG_PRIVATE_KEY_PASSWORD`
4. Once all five secrets are in place, the workflow automatically appends `-PforceRelease` to publish to Central as well; if any one is missing, only GitHub Packages is published.

## 4. Per-Language Release Mechanics

### Python (PyPI)

The `version` in `pyproject.toml` is injected via `sed` → `python -m build` produces sdist/wheel → `twine upload` (token auth).

### JS (npm)

`npm version <v> --no-git-tag-version` sets the version → `npm ci` → `npm run build` (if defined) → `npm publish --access public`.

### Go (module tag)

Go has no central registry; the release convention is a **submodule tag**: `sdks/go/vX.Y.Z` (module path `github.com/cuihairu/croupier/sdks/go`). If the tag already exists it is skipped without error; proxy.golang.org fetches by tag.

### Java (GitHub Packages / Maven Central)

`sdks/java/build.gradle` already adapts three build properties (plus environment variables):

| Property / environment variable                           | Purpose                                                                                                                                                    |
| --------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `-PoverrideVersion=<v>`                                   | Overrides the hard-coded `baseVersion` (the workflow injects the release version)                                                                          |
| `-PforceRelease`                                          | Manual releases without a tag also go through the release channel (signing + OSSRH repository)                                                             |
| `GITHUB_PACKAGES_URL`                                     | Overrides the GitHub Packages upload URL (the workflow points it back to the main `croupier` repo; the default is the standalone repo `croupier-sdk-java`) |
| `MAVEN_CENTRAL_USERNAME/PASSWORD`                         | OSSRH credentials                                                                                                                                          |
| `GPG_KEY_ID / GPG_PRIVATE_KEY / GPG_PRIVATE_KEY_PASSWORD` | Signing (signing is skipped automatically when the key is missing; local-build scenario only)                                                              |

### C# (NuGet)

`dotnet pack -c Release -p:Version=<v>` (`<Version>` in the csproj is not written back) → `dotnet nuget push` (`--skip-duplicate` allows retries).

### C++ (Release attachment)

C++ has no central package registry: `sdks/cpp` is packed into a tarball (excluding the build directory) and uploaded to the GitHub Release for that version; if the Release does not exist, a draft is created automatically.

## 5. First-Release Checklist

1. (As needed) configure the Section 3 secrets;
2. It is recommended to trial-run **Go + C++** first (both require zero external credentials) to verify the workflows and version resolution;
3. Confirm the target package names are not taken (PyPI `croupier` / npm `croupier-js-sdk` / NuGet `Croupier.Sdk`);
4. Before Java syncs to Central, confirm namespace verification and the full GPG secret set;
5. After release, search for the version in each registry: `pip index versions croupier`, `npm view croupier-js-sdk versions`, the NuGet page, `go list -m github.com/cuihairu/croupier/sdks/go@vX.Y.Z`.

## 6. Common Issues

| Symptom                                         | Resolution                                                                                                                |
| ----------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------- |
| Job fails immediately with `missing secret ...` | Configure the corresponding credential as prompted and re-run                                                             |
| PyPI 403 target package unavailable             | Package name taken or token scope excludes that package; first time requires Entire account scope                         |
| NuGet 409                                       | Version already published; `--skip-duplicate` already tolerates this — confirm whether the version number was incremented |
| Java published to the old standalone repo       | `GITHUB_PACKAGES_URL` was not injected; confirm it ran through this workflow rather than the old release.yml              |
| Go tag already exists, skipped                  | Re-publishing the same version is expected idempotent behavior; for a new version specify a new version input             |
