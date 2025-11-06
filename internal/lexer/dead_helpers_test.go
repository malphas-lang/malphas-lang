package lexer

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestLexerHasNoDeadHelpers(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}

	lexerPath := filepath.Join(filepath.Dir(thisFile), "lexer.go")
	src, err := os.ReadFile(lexerPath)
	if err != nil {
		t.Fatalf("failed to read lexer.go: %v", err)
	}

	if strings.Contains(string(src), "stringResult") {
		t.Fatalf("dead helper stringResult still present in lexer.go")
	}
}
