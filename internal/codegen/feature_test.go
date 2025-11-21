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
package main;

fn main() {
    let a = [1, 2, 3];
    let x = a[0];
    println(x);
}
`,
			checks: []string{"[]int{", "[", "println"},
		},
		{
			name: "channel_new_and_send_recv",
			src: `
package main;

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
package main;

fn main() {
    let ch1 = Channel::new[int]();
    let ch2 = Channel::new[int]();
    select {
        case let v = <-ch1 => {
            println(v);
        }
        case ch2 <- 5 => {
            // do nothing
        }
    }
}
`,
			checks: []string{"select", "case", "<-", "println"},
		},
		{
			name: "struct_literal",
			src: `
package main;

struct Point { x: int, y: int }

fn main() {
    let p = Point { x: 1, y: 2 };
    println(p.x);
}
`,
			checks: []string{"Point", "struct", "println"},
		},
		{
			name: "match_expression",
			src: `
package main;

fn main() {
    let x = 1;
    let s = match x {
        1 => { "one" },
        2 => { "two" },
        _ => { "other" }
    };
    println(s);
}
`,
			checks: []string{"switch", "case 1:", "case 2:", "default:", "func()"},
		},
		{
			name: "if_expression",
			src: `
package main;

fn main() {
	let x = if true { 1 } else { 2 };
	println(x);
}
`,
			checks: []string{"if true", "func()"},
		},
		{
			name: "while_loop",
			src: `
package main;

fn main() {
	let x = 0;
	while x < 10 {
		x = x + 1;
	}
	println(x);
}
`,
			checks: []string{"for x < 10", "x = x + 1"},
		},
		{
			name: "for_loop",
			src: `
package main;

fn main() {
	let arr = [10, 20, 30];
	for x in arr {
		println(x);
	}
}
`,
			checks: []string{"range arr", "println(x)"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			RunCodegenTest(t, tt.src, tt.checks)
		})
	}
}
