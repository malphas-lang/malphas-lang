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
			checks: []string{"[]int{", "[", "fmt.Println"},
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
			checks: []string{"make(chan int", "<-", "fmt.Println"},
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
			checks: []string{"select", "case", "<-", "fmt.Println"},
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
			checks: []string{"Point", "struct", "fmt.Println"},
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
			name: "if_expression_basic",
			src: `
package main;

fn main() {
	let x = if true { 1 } else { 2 };
	println(x);
}
`,
			checks: []string{"var x int", "if true", "x = 1", "x = 2"},
		},
		{
			name: "if_expression_string",
			src: `
package main;

fn main() {
	let x = if true { "yes" } else { "no" };
	println(x);
}
`,
			checks: []string{"var x string", "if true", "x = \"yes\"", "x = \"no\""},
		},
		{
			name: "if_expression_multiple_branches",
			src: `
package main;

fn main() {
	let x = 42;
	let y = if x > 20 { 100 } else if x > 10 { 50 } else { 0 };
	println(y);
}
`,
			checks: []string{"var y int", "if x > 20", "else if x > 10", "y = 100", "y = 50", "y = 0"},
		},
		{
			name: "if_expression_with_statements",
			src: `
package main;

fn main() {
	let z = if true {
		let temp = 10;
		temp + 5
	} else {
		0
	};
	println(z);
}
`,
			checks: []string{"var z int", "if true", "temp := 10", "z = temp + 5", "z = 0"},
		},
		{
			name: "if_expression_bool",
			src: `
package main;

fn main() {
	let x = if true { true } else { false };
	println(x);
}
`,
			checks: []string{"var x bool", "if true", "x = true", "x = false"},
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
			checks: []string{"range arr", "fmt.Println(x)"},
		},
		{
			name: "nullable_types",
			src: `
package main;

fn main() {
	let x: int? = null;
	let y: int? = &5;
	let z = y.unwrap();
	let w = y.expect("should not panic");
	println(z);
}
`,
			checks: []string{"*int", "nil", "func[T any](t *T, msg string) T", "panic(msg)"},
		},
		{
			name: "mutable_reference",
			src: `
package main;

fn main() {
	let mut x = 1;
	let y = &mut x;
	println(y);
}
`,
			checks: []string{"&x"},
		},
		{
			name: "raw_pointers",
			src: `
package main;

unsafe fn use_raw_ptr(ptr: *int) -> int {
	let x = *ptr;
	x
}
`,
			checks: []string{"*int", "*ptr"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			RunCodegenTest(t, tt.src, tt.checks)
		})
	}
}
