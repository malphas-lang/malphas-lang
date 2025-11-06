package lexer

import (
	"os"
	"os/exec"
	"runtime/debug"
	"strings"
	"testing"
)

func TestNextTokenDoesNotOverflowStackWhenSkippingManyComments(t *testing.T) {
	if os.Getenv("LEXER_MANY_COMMENTS_HELPER") == "1" {
		runNextTokenManyCommentsHelper()
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=TestNextTokenDoesNotOverflowStackWhenSkippingManyComments")
	cmd.Env = append(os.Environ(), "LEXER_MANY_COMMENTS_HELPER=1")

	if err := cmd.Run(); err != nil {
		t.Fatalf("lexer stack overflow when skipping many comments: %v", err)
	}
}

func runNextTokenManyCommentsHelper() {
	// Force a very small goroutine stack so recursion quickly exceeds the limit.
	debug.SetMaxStack(1 << 15) // 32KB

	const commentCount = 4096

	var b strings.Builder
	b.Grow(len("// comment\n")*commentCount + len("let x = 42;"))
	for i := 0; i < commentCount; i++ {
		b.WriteString("// comment\n")
	}
	b.WriteString("let x = 42;")

	l := New(b.String())

	for {
		tok := l.NextToken()
		if tok.Type == EOF {
			break
		}
		if tok.Type == ILLEGAL {
			os.Exit(3)
		}
	}

	os.Exit(0)
}
