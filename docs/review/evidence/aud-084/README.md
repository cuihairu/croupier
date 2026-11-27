# AUD-084 evidence — release APK network permission + emulator smoke

Date: 2026-11-28. Release APK built locally (Flutter 3.47.9 in a throwaway
copy of `mobile/` at worktree state with the manifest fix; repo lockfile and
CI-pinned Flutter 3.35.1 untouched). Emulator: AVD `aud084`, pixel_6,
system-images;android-36;google_apis;x86_64, API 36.

| File | Step |
| ---- | ---- |
| `01-first-launch-server-url.png` | Fresh install of `app-release.apk`, first-launch server-address page |
| `02-server-url-entered.png` | Server URL `http://192.168.5.5:18780` entered |
| `03-connection-test-passed-login-page.png` |「测试连接并继续」passed — real HTTP request succeeded, app advanced to the login page |
| `04-logged-in-scoped-approvals.png` | Logged in as `admin`, session scope `demo-game / demo`, approvals tab fetched its scoped list (empty) |

Static check on the built artifact (aapt2, build-tools 36.0.0):

```console
$ aapt2 dump permissions app-release.apk
package: com.cuihairu.croupier_mobile
uses-permission: name='android.permission.INTERNET'
permission: com.cuihairu.croupier_mobile.DYNAMIC_RECEIVER_NOT_EXPORTED_PERMISSION
uses-permission: name='com.cuihairu.croupier_mobile.DYNAMIC_RECEIVER_NOT_EXPORTED_PERMISSION'
```

Mutation proof on the real merge pipeline (`:app:processReleaseMainManifest`):
removing the `uses-permission` line from
`mobile/android/app/src/main/AndroidManifest.xml` re-merges to a manifest
without `android.permission.INTERNET` (red); restoring the line re-merges it
back (green, `grep -c` = 1), diff-identical source afterwards.
