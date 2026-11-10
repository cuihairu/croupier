# Contract test entry points

One-command runners for the cross-SDK behavioral contract suites.

## Golden vectors

The single language-neutral contract file is
`proto/contracts/golden_vectors.json` (added by CR-FIX-05). Every SDK test
suite reads that same file and asserts identical bytes/semantics:

| SDK    | Suite                                                                  |
| ------ | ---------------------------------------------------------------------- |
| Go     | `sdks/go/pkg/croupier/protocol/golden_vectors_test.go`                 |
| Python | `sdks/python/tests/test_golden_vectors.py`                             |
| JS/TS  | `sdks/js/src/golden_vectors.test.ts`                                   |
| Java   | `sdks/java/src/test/java/.../GoldenVectorsTest.java`                   |
| C#     | `sdks/csharp/src/Croupier.Sdk.Tests/Transport/GoldenVectorsTests.cs`   |
| C++    | `sdks/cpp/tests/test_golden_vectors.cpp` (via `croupier-sdk-tests`)    |

Run everything (default set: Go / Python / JS / Java / C#):

```bash
scripts/contract/run-golden-vectors.sh
```

Useful flags:

- `--only go,python` — run a subset
- `--skip java` — exclude suites
- `--with-cpp` — include C++ (needs CMake + GTest; configures
  `sdks/cpp/build-contract` and runs the `GoldenVectors` ctest filter)
- `--strict` — missing toolchains become failures instead of skips
- `--list` — print suite names

Exit code: `0` when every run suite passes; `1` on any suite failure (or any
skip under `--strict`). Skipped suites are reported, not silently dropped.

Any change to `proto/**` that shifts wire behavior must keep this entry green
across all six suites — the per-SDK CI jobs run the same suites
individually; this script exists for one-shot local/CI verification.
