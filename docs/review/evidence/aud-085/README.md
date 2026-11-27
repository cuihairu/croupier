# AUD-085 evidence — rehearsal vs managed-release signing split

Date: 2026-11-28. Builds ran in the same throwaway `mobile/` copy used by
AUD-084 (Flutter 3.47.9, Gradle 8.14 / AGP 8.11.1 / Kotlin 2.2.20 in the
copy; repo pins untouched). Test keystore (pkcs12, storepass=keypass,
alias `croupier-test`, SHA-256
`68:e1:ba:2d:4d:41:62:b3:69:b2:90:24:59:7c:58:c0:ce:c0:bf:13:d1:c5:1d:af:3f:7b:96:fc:d6:12:ca:27`)
is local-only under `~/aud084-work/keys/release-test.jks`.

## Verification matrix (all executed locally, emulator-5554 API 36)

| # | Scenario | Command shape | Result |
| - | -------- | ------------- | ------ |
| 1 | Red: release build with no keystore, no rehearsal flag | `./gradlew assembleRelease` (no `CROUPIER_RELEASE_*`, no `CROUPIER_REHEARSAL_BUILD`) | exit 1, `GradleException: release build has no managed signing config: set CROUPIER_RELEASE_KEYSTORE (+ …), or set CROUPIER_REHEARSAL_BUILD=1 …` — aborts in `doFirst` before packaging |
| 2 | Managed signing | env `CROUPIER_RELEASE_KEYSTORE(+PASSWORD/ALIAS/KEY_PASSWORD)` | `BUILD SUCCESSFUL`; `apksigner verify --print-certs` → `CN=croupier release test, O=croupier, C=CN`, SHA-256 `68e1ba2d…12ca27` = the managed test key |
| 3 | Managed artifact installs & boots | `adb uninstall` (drop AUD-084 debug build) → `adb install` → launch | `Success`; `topResumedActivity=…croupier_mobile/.MainActivity`; first-launch server page rendered (screencap) |
| 4 | Same-key version upgrade | `flutter.versionCode=2` rebuild, same key, `adb install -r` | `Success`; `dumpsys package … versionCode=2` |
| 5 | Key-rotation negative | second keystore (alias `otherkey`, SHA-256 `0614683d…eefad`) rebuild, `adb install -r` | `INSTALL_FAILED_UPDATE_INCOMPATIBLE: … signatures do not match newer version` — Android-side guard intact |
| 6 | Rehearsal flag | `CROUPIER_REHEARSAL_BUILD=1`, no keystore env | `BUILD SUCCESSFUL`; WARN `croupier: rehearsal build — app-release.apk is DEBUG-signed, not for distribution`; `apksigner` → `CN=Android Debug` |

## Guard placement

`mobile/android/app/build.gradle.kts` enforces the split in `doFirst` of
`assembleRelease`/`bundleRelease`/`packageRelease`, so debug builds and IDE
sync are unaffected; the red proof (scenario 1) failed in ~20-41 s instead
of producing a debug-signed release APK. Signing selection itself:
`signingConfig = signingConfigs.findByName("managedRelease") ?:
signingConfigs.getByName("debug")` — the managed config exists exactly when
a keystore path was supplied.

## CI wiring (config-level, not dispatched)

`.github/workflows/ci-mobile.yml` `apk` job: `workflow_dispatch` input
`rehearsal` (boolean, default **true**) drives `CROUPIER_REHEARSAL_BUILD`;
`rehearsal=false` restores the keystore from secrets
(`MOBILE_RELEASE_KEYSTORE_B64/PASSWORD/ALIAS/KEY_PASSWORD`, missing secret
→ `::error::` + exit 1), prints the signing cert fingerprint via
`apksigner verify --print-certs`, and names the artifact
`croupier-mobile-rehearsal-debugsigned-apk` vs `croupier-mobile-release-apk`.
YAML parse-validated locally; the job is `workflow_dispatch`-only and was
not dispatched (CI secrets not configured — see boundary below).

## Known boundaries

- The managed keystore used here is a **local test key**; repository secrets
  (`MOBILE_RELEASE_*`) are not configured yet, so CI default stays
  `rehearsal=true` (debug-signed artifact, explicitly named) and a
  `rehearsal=false` dispatch fails with an actionable error until secrets
  are provisioned. This is the intended fail-closed behavior.
- Build toolchain is the throwaway copy (Flutter 3.47.9 / Gradle 8.14 /
  AGP 8.11.1 / Kotlin 2.2.20); repo pins (3.35.1 / 8.12 / 8.9.1 / 2.1.0)
  untouched — same boundary as AUD-084.
- The `versionCode=2` upgrade-test bump was copy-local (`local.properties`);
  the repo default version stays pubspec-derived.
- Emulator-only verification (API 36 x86_64); no physical device.
