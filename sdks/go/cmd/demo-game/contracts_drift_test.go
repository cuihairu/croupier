package main

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// The demo-game contract surface is a verbatim copy of gameops-demo's. This
// test is the three-way drift lock: if any side changes its function set the
// build fails, so the persistent demo process can never silently diverge
// from the reference demo or the web template bindings.
//
//   A: examples/gameops-demo/contracts.go      (reference demo, 34 functions)
//   B: cmd/demo-game/contracts.go              (this binary, 34 functions)
//   C: web gameOpsTemplates.tsx bindings       (template surface, 33 functions)
//
// Invariants: A == B, C ⊂ B, B \ C == {report.generate}, and exactly one
// descriptor on each Go side declares Execution "task".

var (
	descriptorIDRe    = regexp.MustCompile(`ID:\s*"([a-z_]+\.[a-z_]+)"`)
	descriptorLineRe  = regexp.MustCompile(`^\s*\{croupier\.FunctionDescriptor\{ID:`)
	templateIDRe      = regexp.MustCompile(`\{\s*id:\s*'([a-z_]+\.[a-z_]+)',\s*role`)
	taskExecutionRe   = regexp.MustCompile(`Execution:\s*"task"`)
	descriptorFileMat = regexp.MustCompile(`\{croupier\.FunctionDescriptor\{`)
)

func descriptorIDs(t *testing.T, path string) []string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	seen := map[string]bool{}
	for _, line := range strings.Split(string(raw), "\n") {
		if !descriptorLineRe.MatchString(line) {
			continue
		}
		m := descriptorIDRe.FindStringSubmatch(line)
		if m == nil {
			t.Fatalf("descriptor line in %s without parseable ID: %s", path, line)
		}
		seen[m[1]] = true
	}
	ids := make([]string, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func countTaskDescriptors(t *testing.T, path string) int {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, line := range strings.Split(string(raw), "\n") {
		if descriptorFileMat.MatchString(line) && taskExecutionRe.MatchString(line) {
			n++
		}
	}
	return n
}

var (
	// Paths from cmd/demo-game: examples lives under sdks/go, web under the
	// repo root (sdks/go/cmd/demo-game → sdks/go → sdks → root).
	gameopsDemoContracts = filepath.Join("..", "..", "examples", "gameops-demo", "contracts.go")
	templateBindings     = filepath.Join("..", "..", "..", "..", "web", "src", "components", "PageRenderer", "templates", "gameOpsTemplates.tsx")
)

func TestContractSurfaceMatchesGameOpsDemo(t *testing.T) {
	refIDs := descriptorIDs(t, gameopsDemoContracts)
	ownIDs := descriptorIDs(t, "contracts.go")

	if len(refIDs) != 34 {
		t.Fatalf("gameops-demo exposes %d functions, want 34", len(refIDs))
	}
	if len(ownIDs) != len(refIDs) {
		t.Fatalf("demo-game exposes %d functions, gameops-demo has %d", len(ownIDs), len(refIDs))
	}
	for i := range refIDs {
		if ownIDs[i] != refIDs[i] {
			t.Fatalf("function set diverged: demo-game has %q where gameops-demo has %q", ownIDs[i], refIDs[i])
		}
	}
}

func TestContractSurfaceCoversTemplateBindings(t *testing.T) {
	raw, err := os.ReadFile(templateBindings)
	if err != nil {
		t.Fatalf("read template bindings: %v", err)
	}
	seen := map[string]bool{}
	for _, m := range templateIDRe.FindAllStringSubmatch(string(raw), -1) {
		seen[m[1]] = true
	}
	if len(seen) != 33 {
		t.Fatalf("template bindings expose %d functions, want 33", len(seen))
	}

	own := map[string]bool{}
	for _, id := range descriptorIDs(t, "contracts.go") {
		own[id] = true
	}
	for id := range seen {
		if !own[id] {
			t.Fatalf("template binding %q is not served by demo-game", id)
		}
	}
	extra := []string{}
	for id := range own {
		if !seen[id] {
			extra = append(extra, id)
		}
	}
	if len(extra) != 1 || extra[0] != "report.generate" {
		t.Fatalf("demo-game beyond template surface = %v, want only report.generate", extra)
	}
}

func TestReportGenerateIsTheOnlyTaskContract(t *testing.T) {
	if n := countTaskDescriptors(t, "contracts.go"); n != 1 {
		t.Fatalf("demo-game has %d task-mode descriptors, want exactly 1 (report.generate)", n)
	}
	if n := countTaskDescriptors(t, gameopsDemoContracts); n != 1 {
		t.Fatalf("gameops-demo has %d task-mode descriptors, want exactly 1", n)
	}
}
