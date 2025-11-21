package codegen

import (
	"strings"
	"testing"
)

// RunCodegenTest parses, type‑checks and generates Go code for the given Malphas source.
// It then verifies that each of the expected substrings appears in the generated output.
// If any check fails, the test is marked as failed with a descriptive message.
func RunCodegenTest(t *testing.T, src string, checks []string) {
	t.Helper()
	out, err := GenerateGo(src)
	if err != nil {
		t.Fatalf("code generation failed: %v", err)
	}
	if strings.TrimSpace(out) == "" {
		t.Fatalf("generated output is empty")
	}
	for _, chk := range checks {
		if !strings.Contains(out, chk) {
			t.Errorf("expected generated code to contain %q, but it was missing.\nGenerated output:\n%s", chk, out)
		}
	}
}
