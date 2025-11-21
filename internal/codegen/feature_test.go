package codegen

import (
	"testing"
)

func TestGenerateGo_Features(t *testing.T) {
	tests := []struct {
		name   string
		src    string
		checks []string
	}{
		{
			name: "array_literal_and_index",
			src: `
package main

fn main() {
    let a = [1, 2, 3];
    let x = a[0];
    println(x);
}
`,
			checks: []string{"[]int{", "IndexExpr", "println"},
		},
		{
			name: "channel_new_and_send_recv",
			src: `
package main

fn main() {
    let ch = Channel::new[int]();
    ch <- 42;
    let v = <-ch;
    println(v);
}
`,
			checks: []string{"make(chan int", "<-", "println"},
		},
		{
			name: "select_statement",
			src: `
package main

fn main() {
    select {
        case let v = <-ch1:
            println(v);
        case ch2 <- 5:
            // do nothing
    }
}
`,
			checks: []string{"select", "case", "<-", "println"},
		},
		{
			name: "struct_literal",
			src: `
package main

type Point struct { x: int; y: int; }

fn main() {
    let p = Point { x: 1, y: 2 };
    println(p.x);
}
`,
			checks: []string{"Point", "struct", "println"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			RunCodegenTest(t, tt.src, tt.checks)
		})
	}
}
