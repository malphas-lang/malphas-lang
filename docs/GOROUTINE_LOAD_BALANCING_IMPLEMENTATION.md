# Load Balancing Implementation Guide

## Starting Point: Load-Aware Distribution

This is the **easiest and safest** improvement to start with. It provides immediate value with minimal risk.

## Current Implementation

**Location**: `runtime/runtime.c`, line 868

```c
// Current: Simple round-robin
int queue_idx = legion->id % MAX_OS_THREADS;
```

**Problem**: Doesn't consider current load on each thread. All threads get equal distribution regardless of queue length.

## Step 1: Add Queue Length Helper Function

Add this function **before** `runtime_legion_start()` (around line 858):

```c
// Get approximate queue length for a thread (lock-free, approximate)
static int get_queue_length(int thread_id) {
    int head = atomic_load(&g_scheduler->queue_head[thread_id]);
    int tail = atomic_load(&g_scheduler->queue_tail[thread_id]);
    
    if (tail >= head) {
        return tail - head;
    } else {
        // Queue wrapped around
        return (LEGION_QUEUE_SIZE - head) + tail;
    }
}

// Find the thread with the least load
static int find_least_loaded_thread(void) {
    int best_thread = 0;
    int best_load = get_queue_length(0);
    
    // Check all threads to find the one with shortest queue
    for (int i = 1; i < MAX_OS_THREADS; i++) {
        int load = get_queue_length(i);
        if (load < best_load) {
            best_load = load;
            best_thread = i;
        }
    }
    
    return best_thread;
}
```

## Step 2: Update `runtime_legion_start()`

**Location**: `runtime/runtime.c`, line 860-888

**Replace** line 868:
```c
// OLD:
int queue_idx = legion->id % MAX_OS_THREADS;
```

**With**:
```c
// NEW: Load-aware distribution
int queue_idx = find_least_loaded_thread();
```

## Step 3: Test the Change

1. **Compile**: Make sure the runtime still compiles
2. **Run existing tests**: Ensure nothing breaks
3. **Create a simple benchmark**: Spawn many legions and verify they're distributed more evenly

## Expected Benefits

- **Better utilization**: Threads with shorter queues get more work
- **Reduced latency**: Legions wait less time in queues
- **Fairer distribution**: Load is balanced across all threads

## Next Steps (After This Works)

Once load-aware distribution is working, you can enhance it further:

1. **Periodic rebalancing**: Check for load imbalance and migrate legions
2. **Cache locality**: Keep related legions on the same thread
3. **NUMA awareness**: Consider CPU topology for multi-socket systems

## Testing

Create a simple test program:

```malphas
fn main() {
    let ch = Channel[i32]::new(100);
    
    // Spawn many legions
    for i in 0..100 {
        spawn {
            ch <- i;
        };
    }
    
    // Collect results
    for _ in 0..100 {
        let x = <-ch;
        println(x);
    }
}
```

Run this and observe that legions are distributed more evenly across threads compared to round-robin.

## Files to Modify

1. **`runtime/runtime.c`**:
   - Add `get_queue_length()` function (before `runtime_legion_start`)
   - Add `find_least_loaded_thread()` function (before `runtime_legion_start`)
   - Modify `runtime_legion_start()` line 868

That's it! This is a minimal, safe change that provides immediate value.

