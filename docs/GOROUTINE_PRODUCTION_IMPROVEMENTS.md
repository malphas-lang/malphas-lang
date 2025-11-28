# Goroutine/Legion Production Improvements

## Overview

The current goroutine (legion) implementation provides a functional foundation for M:N threading in Malphas. This document outlines production-grade improvements that should be considered for future enhancements.

**Current Status**: The foundation is complete and functional. The following improvements are recommended for production use but are not required for the current implementation.

## 1. Sophisticated Stack Growth Using Virtual Memory

### Current Implementation

The current stack growth mechanism (`grow_legion_stack()` in `runtime/runtime.c`) uses a simple reallocation approach:

```c
// Current: Simple reallocation
void* new_stack = runtime_alloc(new_size);
memcpy(new_stack, legion->stack, legion->stack_size);
```

**Limitations:**
- Requires copying the entire stack when growing
- Inefficient for large stacks
- May cause pauses during stack growth
- Doesn't leverage OS virtual memory capabilities

### Production Approach

Use virtual memory tricks similar to Go's runtime:

1. **Segmented Stacks**: Allocate stack in segments, grow by adding new segments
2. **Virtual Memory Mapping**: Use `mmap()` with `MAP_GROWSDOWN` or similar flags
3. **Copy-on-Grow**: Only copy active portions of the stack
4. **Guard Pages**: Already implemented, but can be enhanced for better overflow detection

**Implementation Notes:**
- Use `mremap()` on Linux for efficient stack growth
- Implement stack copying that only copies the active portion (from stack pointer to base)
- Consider using `MAP_GROWSDOWN` flag for automatic growth (with care for security)
- Implement stack shrinking when legions become idle

**References:**
- Go runtime: `runtime/stack.go`
- Rust async runtime stack management
- LLVM coroutine stack growth strategies

## 2. Platform-Specific Assembly for Context Switching

### Current Implementation

The current implementation uses `ucontext_t` from `<ucontext.h>`:

```c
ucontext_t ctx;           // Execution context (for context switching)
ucontext_t ctx_saved;     // Saved context when blocked
// ...
swapcontext(&current->ctx_saved, scheduler_ctx);
```

**Limitations:**
- `ucontext_t` is deprecated on some platforms (macOS)
- Slower than hand-written assembly
- Less control over what gets saved/restored
- May not be available on all target platforms

### Production Approach

Implement platform-specific assembly for context switching:

1. **x86-64 Assembly**: Save/restore registers manually
2. **ARM64 Assembly**: Platform-specific register handling
3. **RISC-V Support**: For emerging platforms
4. **Fallback**: Keep `ucontext_t` as fallback for unsupported platforms

**Benefits:**
- Faster context switches (critical for performance)
- More control over what gets saved
- Better compatibility across platforms
- Can optimize for specific CPU features

**Implementation Notes:**
- Create platform-specific files: `runtime/context_amd64.s`, `runtime/context_arm64.s`
- Implement `runtime_legion_switch()` in assembly
- Save minimal register set (callee-saved registers)
- Handle signal masks appropriately
- Test on multiple platforms

**References:**
- Go runtime: `runtime/asm_*.s` files
- Rust async runtime context switching
- Boost.Context library implementation

## 3. Preemption Support

### Current Implementation

Currently, preemption is **cooperative** via `yield()`:

```c
// Yield control to scheduler (cooperative)
void runtime_legion_yield(void) {
    // ... saves context and switches back to scheduler
}
```

**Limitations:**
- Legions must explicitly call `yield()` to be preempted
- Long-running CPU-bound tasks can starve other legions
- No automatic time slicing
- Can lead to unfair scheduling

### Production Approach

Implement **preemptive scheduling**:

1. **Signal-Based Preemption**: Use `SIGPROF` or `SIGURG` to interrupt running legions
2. **Timer Interrupts**: Set up periodic timers to trigger preemption checks
3. **Safe Points**: Only preempt at safe points (function calls, loop headers)
4. **Stack Scanning**: Ensure GC can scan stacks during preemption

**Implementation Notes:**
- Install signal handler for preemption signal
- Mark safe points in generated code (or use function prologues)
- Save/restore signal masks during context switches
- Coordinate with garbage collector for stack scanning
- Consider using `pthread_sigmask()` for signal delivery control

**Challenges:**
- Must ensure preemption doesn't corrupt program state
- Need to handle preemption during system calls
- Coordinate with channel operations and other blocking primitives
- Performance overhead of signal handling

**References:**
- Go runtime: `runtime/preempt.go`, `runtime/signal_unix.go`
- Java HotSpot VM preemption
- Erlang/OTP scheduler preemption

## 4. Better Load Balancing Strategies

### Current Implementation

Current load balancing uses simple round-robin distribution:

```c
// Add to a queue for load balancing
int queue_idx = legion->id % MAX_OS_THREADS;
```

**Limitations:**
- Doesn't consider current load on each thread
- Doesn't account for legion characteristics (CPU-bound vs IO-bound)
- No dynamic load adjustment
- Simple work-stealing but could be improved

### Production Approach

Implement **sophisticated load balancing**:

1. **Work-Stealing Improvements**:
   - Adaptive work-stealing thresholds
   - Steal from most loaded threads first
   - Consider cache locality when stealing

2. **Load-Aware Scheduling**:
   - Track queue lengths per thread
   - Distribute new legions to least loaded threads
   - Rebalance when load imbalance is detected

3. **Legion Affinity**:
   - Keep related legions on same thread (cache locality)
   - Migrate legions when load is imbalanced
   - Consider NUMA topology for multi-socket systems

4. **Adaptive Thread Pool**:
   - Dynamically adjust number of OS threads based on load
   - Create threads on demand
   - Shrink thread pool when idle

**Implementation Notes:**
- Add load metrics to scheduler (queue lengths, active legions per thread)
- Implement periodic rebalancing
- Consider using `sched_getaffinity()` for CPU topology
- Add configuration for load balancing strategy

**References:**
- Go runtime: `runtime/proc.go` (work-stealing scheduler)
- Java ForkJoinPool load balancing
- Cilk++ work-stealing scheduler

## Implementation Priority

### Phase 1: High Impact, Medium Effort
1. **Platform-Specific Assembly** - Significant performance improvement
2. **Better Load Balancing** - Improves fairness and utilization

### Phase 2: High Impact, High Effort
3. **Sophisticated Stack Growth** - Important for memory efficiency
4. **Preemption Support** - Critical for fairness and responsiveness

## Testing Considerations

For each improvement, consider:

1. **Performance Benchmarks**: Measure context switch overhead, stack growth cost
2. **Fairness Tests**: Ensure no legion starvation
3. **Stress Tests**: Many legions, long-running tasks
4. **Platform Coverage**: Test on Linux, macOS, Windows (if supported)
5. **Regression Tests**: Ensure existing functionality still works

## Migration Path

1. **Keep Current Implementation**: The current implementation is functional and can remain as-is
2. **Add Improvements Incrementally**: Each improvement can be added independently
3. **Feature Flags**: Consider compile-time flags to enable/disable improvements
4. **Backward Compatibility**: Ensure improvements don't break existing code

## Conclusion

The current goroutine/legion implementation provides a solid foundation. The improvements outlined above would bring it to production-grade quality, but they are not required for the current use case. Consider implementing them based on:

- Performance requirements
- Target platforms
- Resource constraints
- Development priorities

The foundation is complete and functional. These improvements can be added incrementally as needed.

