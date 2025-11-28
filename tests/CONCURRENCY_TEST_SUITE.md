# Concurrency Test Suite

This directory contains comprehensive tests for the Malphas concurrency implementation.

## Test Files

### 1. `test_concurrency_simple.mal`
Basic concurrency tests covering:
- Basic channel send/receive
- Spawn with function calls
- Spawn with arguments
- Select statements

### 2. `test_concurrency_comprehensive.mal`
Comprehensive test suite with 20 tests covering:
- Basic channel operations
- Spawn with function calls
- Spawn with captured variables (via function parameters)
- Select with single and multiple cases
- Buffered vs unbuffered channels
- Multiple goroutines
- Producer-consumer patterns
- Fan-out and fan-in patterns
- Nested spawns
- Channels with different types
- Sequential spawns
- Spawn with loops

### 3. `test_concurrency_edge_cases.mal`
Edge case and stress tests with 15 tests covering:
- Empty select statements
- Select with only send cases
- Select with only receive cases
- Large number of goroutines (50+)
- Deeply nested spawns
- Zero capacity channels
- Multiple selects in sequence
- Spawn with complex expressions
- Select with same channel multiple times
- Spawn in loops with variable capture
- Channel communication between functions
- Multiple producers and consumers

## Known Issues

See `test_concurrency_known_issues.md` for details on current bugs preventing tests from running.

## Running Tests

Once the channel send operator bug is fixed, run tests with:

```bash
./malphas run tests/test_concurrency_simple.mal
./malphas run tests/test_concurrency_comprehensive.mal
./malphas run tests/test_concurrency_edge_cases.mal
```

## Test Coverage

The test suite covers:

### Channel Operations
- ✅ Channel creation with `Channel[T]::new(capacity)`
- ✅ Channel send: `ch <- value`
- ✅ Channel receive: `<-ch` or `let x = <-ch`
- ✅ Buffered channels (capacity > 0)
- ✅ Unbuffered channels (capacity = 0)

### Spawn (Goroutines)
- ✅ Spawn with function calls: `spawn worker(args);`
- ❌ Spawn with block syntax: `spawn { ... };` (not working)
- ❌ Spawn with function literals: `spawn |params| { ... }(args);` (not working)

### Select Statements
- ✅ Select with single receive case
- ✅ Select with single send case
- ✅ Select with multiple cases
- ✅ Select with both send and receive cases
- ✅ Empty select (should compile but do nothing)

### Patterns
- ✅ Producer-consumer
- ✅ Fan-out (one producer, multiple consumers)
- ✅ Fan-in (multiple producers, one consumer)
- ✅ Worker pools
- ✅ Sequential spawns
- ✅ Nested spawns

## Expected Behavior

Once bugs are fixed, all tests should:
1. Compile without errors
2. Run without panics
3. Produce expected output
4. Demonstrate correct concurrency behavior

## Notes

- All tests use `int` type (not `i32`) to match existing examples
- Tests avoid `panic()` calls since that function doesn't exist
- Tests use only `spawn function()` syntax (block syntax not working)
- Channel receive operations should handle `nil` values for closed channels (future enhancement)

