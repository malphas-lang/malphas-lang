// runtime/runtime.c
// Malphas Runtime Library Implementation

// Define feature test macros before including any headers
// _XOPEN_SOURCE is required on macOS and some other systems for ucontext
// routines
#define _XOPEN_SOURCE 600
// _DARWIN_C_SOURCE is required on macOS for MAP_ANONYMOUS
#define _DARWIN_C_SOURCE

#include "runtime.h"
#include <gc/gc.h> // Boehm GC
#include <pthread.h>
#include <stdatomic.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <time.h>
#include <unistd.h>
// #include <ucontext.h>  // Removed: deprecated on macOS
#include <signal.h>   // For stack overflow detection
#include <sys/mman.h> // For mmap for stack allocation

// Simple hash map implementation (for now, using a basic approach)
#define HASHMAP_INITIAL_SIZE 16

typedef struct HashMapEntry {
  String *key;
  void *value;
  struct HashMapEntry *next;
} HashMapEntry;

struct HashMap {
  HashMapEntry **buckets;
  size_t size;
  size_t capacity;
};

// Garbage collector initialization
// This should be called once at program startup
void runtime_gc_init(void) { GC_INIT(); }

// Memory allocation using Boehm GC
void *runtime_alloc(size_t size) {
  void *ptr = GC_malloc(size);
  if (!ptr) {
    fprintf(stderr, "runtime_alloc: out of memory\n");
    abort();
  }
  return ptr;
}

// String operations
String *runtime_string_new(const char *data, size_t len) {
  String *s = (String *)runtime_alloc(sizeof(String));
  s->len = len;
  s->data = (char *)runtime_alloc(len + 1);
  memcpy(s->data, data, len);
  s->data[len] = '\0';
  return s;
}

void runtime_string_free(String *s) {
  // With GC, we don't need to manually free memory
  // This function is kept for API compatibility but does nothing
  // The GC will automatically reclaim memory when objects are no longer
  // reachable
  (void)s; // Suppress unused parameter warning
}

const char *runtime_string_cstr(String *s) { return s ? s->data : ""; }

// String concatenation
String *runtime_string_concat(String *a, String *b) {
  if (!a && !b) {
    return runtime_string_new("", 0);
  }
  if (!a) {
    return runtime_string_new(b->data, b->len);
  }
  if (!b) {
    return runtime_string_new(a->data, a->len);
  }

  size_t total_len = a->len + b->len;
  String *result = (String *)runtime_alloc(sizeof(String));
  result->len = total_len;
  result->data = (char *)runtime_alloc(total_len + 1);

  memcpy(result->data, a->data, a->len);
  memcpy(result->data + a->len, b->data, b->len);
  result->data[total_len] = '\0';

  return result;
}

// Convert integer to string
String *runtime_string_from_i64(int64_t value) {
  char buffer[32];
  int len = snprintf(buffer, sizeof(buffer), "%lld", (long long)value);
  if (len < 0)
    len = 0;
  if (len >= (int)sizeof(buffer))
    len = sizeof(buffer) - 1;
  return runtime_string_new(buffer, len);
}

// Convert double to string
String *runtime_string_from_double(double value) {
  char buffer[64];
  int len = snprintf(buffer, sizeof(buffer), "%g", value);
  if (len < 0)
    len = 0;
  if (len >= (int)sizeof(buffer))
    len = sizeof(buffer) - 1;
  return runtime_string_new(buffer, len);
}

// Convert bool to string
String *runtime_string_from_bool(int8_t value) {
  if (value) {
    return runtime_string_new("true", 4);
  } else {
    return runtime_string_new("false", 5);
  }
}

// String formatting with {} placeholders
// Takes format string and up to 4 arguments (all as String*)
// Replaces {} with arguments in order
String *runtime_string_format(String *fmt, String *arg1, String *arg2,
                              String *arg3, String *arg4) {
  if (!fmt || !fmt->data) {
    return runtime_string_new("", 0);
  }

  String *args[] = {arg1, arg2, arg3, arg4};

  // Estimate result size (format string + all arguments)
  size_t result_size = fmt->len;
  int placeholder_count = 0;
  for (size_t i = 0; i < fmt->len; i++) {
    if (fmt->data[i] == '{' && i + 1 < fmt->len && fmt->data[i + 1] == '}') {
      placeholder_count++;
      if (placeholder_count <= 4 && args[placeholder_count - 1]) {
        result_size += args[placeholder_count - 1]->len;
      }
      result_size -= 2; // Remove {} from size
      i++;              // Skip the '}'
    }
  }

  // Allocate result buffer
  char *result_buf = (char *)runtime_alloc(result_size + 1);
  size_t result_pos = 0;
  int current_arg = 0;

  // Process format string
  for (size_t i = 0; i < fmt->len; i++) {
    if (fmt->data[i] == '{' && i + 1 < fmt->len && fmt->data[i + 1] == '}') {
      // Replace {} with argument
      if (current_arg < 4 && args[current_arg]) {
        String *arg = args[current_arg];
        memcpy(result_buf + result_pos, arg->data, arg->len);
        result_pos += arg->len;
      }
      current_arg++;
      i++; // Skip the '}'
    } else {
      result_buf[result_pos++] = fmt->data[i];
    }
  }

  result_buf[result_pos] = '\0';
  String *result = (String *)runtime_alloc(sizeof(String));
  result->len = result_pos;
  result->data = result_buf;

  return result;
}

// Print functions
void runtime_println_i64(int64_t value) { printf("%lld\n", value); }

void runtime_println_i32(int32_t value) { printf("%d\n", value); }

void runtime_println_i8(int8_t value) { printf("%d\n", value); }

void runtime_println_double(double value) { printf("%g\n", value); }

void runtime_println_bool(int8_t value) {
  printf("%s\n", value ? "true" : "false");
}

void runtime_println_string(String *s) {
  if (s && s->data) {
    printf("%s\n", s->data);
  } else {
    printf("(null)\n");
  }
}

// Slice operations (for Vec)
Slice *runtime_slice_new(size_t elem_size, size_t len, size_t cap) {
  if (cap < len)
    cap = len;
  if (cap == 0)
    cap = 1;

  Slice *slice = (Slice *)runtime_alloc(sizeof(Slice));
  slice->len = len;
  slice->cap = cap;
  slice->elem_size = elem_size;
  slice->data = runtime_alloc(elem_size * cap);
  memset(slice->data, 0, elem_size * cap);
  return slice;
}

void *runtime_slice_get(Slice *slice, size_t index) {
  if (!slice || index >= slice->len) {
    fprintf(stderr, "runtime_slice_get: index out of bounds\n");
    abort();
  }
  return (char *)slice->data + (index * slice->elem_size);
}

void runtime_slice_set(Slice *slice, size_t index, void *value) {
  if (!slice || index >= slice->len) {
    fprintf(stderr, "runtime_slice_set: index out of bounds\n");
    abort();
  }
  void *dest = (char *)slice->data + (index * slice->elem_size);
  memcpy(dest, value, slice->elem_size);
}

void runtime_slice_push(Slice *slice, void *value) {
  if (!slice) {
    fprintf(stderr, "runtime_slice_push: null slice\n");
    abort();
  }

  // Grow if needed
  if (slice->len >= slice->cap) {
    size_t new_cap = slice->cap * 2;
    if (new_cap == 0)
      new_cap = 1;
    // Use GC_realloc for growing slices
    slice->data = GC_realloc(slice->data, slice->elem_size * new_cap);
    if (!slice->data) {
      fprintf(stderr, "runtime_slice_push: out of memory\n");
      abort();
    }
    slice->cap = new_cap;
  }

  void *dest = (char *)slice->data + (slice->len * slice->elem_size);
  memcpy(dest, value, slice->elem_size);
  slice->len++;
}

size_t runtime_slice_len(Slice *slice) { return slice ? slice->len : 0; }

int8_t runtime_slice_is_empty(Slice *slice) {
  return (slice == NULL || slice->len == 0) ? 1 : 0;
}

size_t runtime_slice_cap(Slice *slice) { return slice ? slice->cap : 0; }

void runtime_slice_reserve(Slice *slice, size_t additional) {
  if (!slice) {
    fprintf(stderr, "runtime_slice_reserve: null slice\n");
    abort();
  }

  size_t needed = slice->len + additional;
  if (needed > slice->cap) {
    // Grow capacity to accommodate the additional elements
    size_t new_cap = slice->cap;
    while (new_cap < needed) {
      new_cap = new_cap * 2;
      if (new_cap == 0)
        new_cap = 1;
    }

    slice->data = GC_realloc(slice->data, slice->elem_size * new_cap);
    if (!slice->data) {
      fprintf(stderr, "runtime_slice_reserve: out of memory\n");
      abort();
    }
    slice->cap = new_cap;
  }
}

void runtime_slice_clear(Slice *slice) {
  if (!slice) {
    fprintf(stderr, "runtime_slice_clear: null slice\n");
    abort();
  }
  slice->len = 0;
  // Note: We don't free the data, just reset the length
  // This allows reuse of the allocated capacity
}

void *runtime_slice_pop(Slice *slice) {
  if (!slice || slice->len == 0) {
    return NULL; // Empty slice, return NULL
  }

  slice->len--;
  void *result = runtime_alloc(slice->elem_size);
  void *src = (char *)slice->data + (slice->len * slice->elem_size);
  memcpy(result, src, slice->elem_size);
  return result;
}

void runtime_slice_remove(Slice *slice, size_t index) {
  if (!slice || index >= slice->len) {
    fprintf(stderr, "runtime_slice_remove: index out of bounds\n");
    abort();
  }

  // Shift elements after index to the left
  size_t elems_to_move = slice->len - index - 1;
  if (elems_to_move > 0) {
    void *dest = (char *)slice->data + (index * slice->elem_size);
    void *src = (char *)dest + slice->elem_size;
    memmove(dest, src, elems_to_move * slice->elem_size);
  }
  slice->len--;
}

void runtime_slice_insert(Slice *slice, size_t index, void *value) {
  if (!slice) {
    fprintf(stderr, "runtime_slice_insert: null slice\n");
    abort();
  }

  if (index > slice->len) {
    fprintf(stderr, "runtime_slice_insert: index out of bounds\n");
    abort();
  }

  // Grow if needed
  if (slice->len >= slice->cap) {
    size_t new_cap = slice->cap * 2;
    if (new_cap == 0)
      new_cap = 1;
    slice->data = GC_realloc(slice->data, slice->elem_size * new_cap);
    if (!slice->data) {
      fprintf(stderr, "runtime_slice_insert: out of memory\n");
      abort();
    }
    slice->cap = new_cap;
  }

  // Shift elements from index to the right
  if (index < slice->len) {
    void *dest = (char *)slice->data + ((index + 1) * slice->elem_size);
    void *src = (char *)slice->data + (index * slice->elem_size);
    size_t elems_to_move = slice->len - index;
    memmove(dest, src, elems_to_move * slice->elem_size);
  }

  // Insert the new element
  void *dest = (char *)slice->data + (index * slice->elem_size);
  memcpy(dest, value, slice->elem_size);
  slice->len++;
}

Slice *runtime_slice_copy(Slice *slice) {
  if (!slice) {
    return NULL;
  }

  Slice *copy = (Slice *)runtime_alloc(sizeof(Slice));
  copy->len = slice->len;
  copy->cap = slice->cap;
  copy->elem_size = slice->elem_size;
  copy->data = runtime_alloc(slice->elem_size * slice->cap);
  memcpy(copy->data, slice->data, slice->elem_size * slice->len);
  // Zero out the rest of the capacity
  if (slice->len < slice->cap) {
    memset((char *)copy->data + (slice->elem_size * slice->len), 0,
           slice->elem_size * (slice->cap - slice->len));
  }
  return copy;
}

Slice *runtime_slice_subslice(Slice *slice, size_t start, size_t end) {
  if (!slice) {
    fprintf(stderr, "runtime_slice_subslice: null slice\n");
    abort();
  }

  if (start > end || end > slice->len) {
    fprintf(stderr,
            "runtime_slice_subslice: invalid range [%zu:%zu) for slice of "
            "length %zu\n",
            start, end, slice->len);
    abort();
  }

  // Create a new slice with a copy of the data
  // This ensures the subslice is independent and safe even if the original is
  // freed
  size_t sub_len = end - start;
  Slice *sub = (Slice *)runtime_alloc(sizeof(Slice));
  sub->len = sub_len;
  sub->cap = sub_len; // Capacity matches length for subslice
  sub->elem_size = slice->elem_size;
  sub->data = runtime_alloc(slice->elem_size * sub_len);

  // Copy the elements from the original slice
  void *src = (char *)slice->data + (start * slice->elem_size);
  memcpy(sub->data, src, slice->elem_size * sub_len);

  return sub;
}

// Simple hash function for strings
static size_t hash_string(String *key) {
  if (!key || !key->data)
    return 0;
  size_t hash = 5381;
  for (size_t i = 0; i < key->len; i++) {
    hash = ((hash << 5) + hash) + key->data[i];
  }
  return hash;
}

// String comparison
static int string_equal(String *a, String *b) {
  if (!a || !b)
    return a == b;
  if (a->len != b->len)
    return 0;
  return memcmp(a->data, b->data, a->len) == 0;
}

// Public string comparison function for LLVM codegen
int runtime_string_equal(String *a, String *b) { return string_equal(a, b); }

// HashMap operations
HashMap *runtime_hashmap_new(void) {
  HashMap *map = (HashMap *)runtime_alloc(sizeof(HashMap));
  map->capacity = HASHMAP_INITIAL_SIZE;
  map->size = 0;
  // Use GC_malloc instead of calloc, then zero-initialize
  map->buckets =
      (HashMapEntry **)runtime_alloc(map->capacity * sizeof(HashMapEntry *));
  memset(map->buckets, 0, map->capacity * sizeof(HashMapEntry *));
  return map;
}

void runtime_hashmap_put(HashMap *map, String *key, void *value) {
  if (!map || !key)
    return;

  size_t hash = hash_string(key);
  size_t index = hash % map->capacity;

  // Check if key exists
  HashMapEntry *entry = map->buckets[index];
  while (entry) {
    if (string_equal(entry->key, key)) {
      entry->value = value;
      return;
    }
    entry = entry->next;
  }

  // Insert new entry
  entry = (HashMapEntry *)runtime_alloc(sizeof(HashMapEntry));
  entry->key = key;
  entry->value = value;
  entry->next = map->buckets[index];
  map->buckets[index] = entry;
  map->size++;
}

void *runtime_hashmap_get(HashMap *map, String *key) {
  if (!map || !key)
    return NULL;

  size_t hash = hash_string(key);
  size_t index = hash % map->capacity;

  HashMapEntry *entry = map->buckets[index];
  while (entry) {
    if (string_equal(entry->key, key)) {
      return entry->value;
    }
    entry = entry->next;
  }
  return NULL;
}

int8_t runtime_hashmap_contains_key(HashMap *map, String *key) {
  if (!map || !key)
    return 0;

  size_t hash = hash_string(key);
  size_t index = hash % map->capacity;

  HashMapEntry *entry = map->buckets[index];
  while (entry) {
    if (string_equal(entry->key, key)) {
      return 1; // Key exists
    }
    entry = entry->next;
  }
  return 0; // Key does not exist
}

size_t runtime_hashmap_len(HashMap *map) { return map ? map->size : 0; }

int8_t runtime_hashmap_is_empty(HashMap *map) {
  return (map == NULL || map->size == 0) ? 1 : 0;
}

void runtime_hashmap_free(HashMap *map) {
  // With GC, we don't need to manually free memory
  // This function is kept for API compatibility but does nothing
  // The GC will automatically reclaim memory when objects are no longer
  // reachable
  (void)map; // Suppress unused parameter warning
}

// ============================================================================
// Legion (M:N Threading Model) - Infernal Scheduler
// ============================================================================
// Legions are lightweight concurrent entities spawned by the spawn keyword.
// Many legions are scheduled onto fewer OS threads by the infernal scheduler.
// NOTE: Legion struct must be defined before Channel struct because Channel
// functions access Legion members.

#define LEGION_STACK_SIZE (256 * 1024)     // 256KB initial stack
#define LEGION_STACK_MAX (2 * 1024 * 1024) // 2MB max stack size
#define LEGION_STACK_GUARD_SIZE (4096)     // Guard page size
#define MAX_OS_THREADS 4      // Number of OS threads in the thread pool
#define LEGION_QUEUE_SIZE 256 // Work-stealing queue size
#define WORK_STEAL_ATTEMPTS 3 // Number of queues to try when stealing

// Legion states
typedef enum {
  LEGION_STATE_RUNNABLE, // Ready to run
  LEGION_STATE_RUNNING,  // Currently executing
  LEGION_STATE_BLOCKED,  // Blocked on channel/IO
  LEGION_STATE_DEAD      // Completed
} LegionState;

// Forward declaration
static void legion_entry(Legion *legion);

// Context structure for green threads
#if defined(__aarch64__)
// ARM64: x19-x28, fp, lr, sp
typedef struct Context {
  uint64_t x19;
  uint64_t x20;
  uint64_t x21;
  uint64_t x22;
  uint64_t x23;
  uint64_t x24;
  uint64_t x25;
  uint64_t x26;
  uint64_t x27;
  uint64_t x28;
  uint64_t fp; // x29
  uint64_t lr; // x30
  uint64_t sp;
} Context;
#elif defined(__x86_64__)
// x86_64: rbx, rbp, r12-r15, rsp, rip
typedef struct Context {
  uint64_t rbx;
  uint64_t rbp;
  uint64_t r12;
  uint64_t r13;
  uint64_t r14;
  uint64_t r15;
  uint64_t rsp;
  uint64_t rip;
} Context;
#else
#error "Unsupported architecture"
#endif

// Context switching functions implemented in inline assembly
__attribute__((noinline)) static void malphas_context_switch(Context *from,
                                                             Context *to) {
#if defined(__aarch64__)
  __asm__ volatile(
      // Save current context to 'from'
      "stp x19, x20, [%0, #0]\n\t"
      "stp x21, x22, [%0, #16]\n\t"
      "stp x23, x24, [%0, #32]\n\t"
      "stp x25, x26, [%0, #48]\n\t"
      "stp x27, x28, [%0, #64]\n\t"
      "stp x29, x30, [%0, #80]\n\t" // fp, lr
      "mov x9, sp\n\t"
      "str x9, [%0, #96]\n\t"

      // Load new context from 'to'
      "ldp x19, x20, [%1, #0]\n\t"
      "ldp x21, x22, [%1, #16]\n\t"
      "ldp x23, x24, [%1, #32]\n\t"
      "ldp x25, x26, [%1, #48]\n\t"
      "ldp x27, x28, [%1, #64]\n\t"
      "ldp x29, x30, [%1, #80]\n\t"
      "ldr x9, [%1, #96]\n\t"
      "mov sp, x9\n\t"

      // Return (lr is restored to x30, so ret will jump to saved location)
      "ret\n\t"
      :
      : "r"(from), "r"(to)
      : "memory", "x9");
#elif defined(__x86_64__)
  __asm__ volatile(
      // Save current context to 'from'
      "movq %%rbx, 0(%0)\n\t"
      "movq %%rbp, 8(%0)\n\t"
      "movq %%r12, 16(%0)\n\t"
      "movq %%r13, 24(%0)\n\t"
      "movq %%r14, 32(%0)\n\t"
      "movq %%r15, 40(%0)\n\t"
      "movq %%rsp, 48(%0)\n\t"
      // Save return address (current instruction pointer is implicitly saved on
      // stack by call, but we need to save the return address from the stack to
      // our struct) Actually, for switch, we just save the return address that
      // is on the stack? No, standard way is to save a label or rely on 'ret'
      // using the value on stack. But here we are switching stacks.

      // Better approach for x86_64 switch:
      // 1. Save callee-saved regs.
      // 2. Save RSP.
      // 3. Load new RSP.
      // 4. Restore callee-saved regs.
      // 5. Ret (pops RIP from new stack).

      // Wait, 'from->rip' needs to be set to the return address of THIS call.
      // The 'call' instruction pushed RIP onto the stack.
      // So 'pop' into a temp reg and save it? No, we want 'ret' to work later.

      // Correct logic:
      // We don't explicitly save RIP in the struct during switch.
      // The RIP is on the stack. We save the SP *after* the call pushed RIP.
      // When we restore SP and 'ret', it pops the RIP.
      // So 'rip' field in Context is only used for initialization
      // (make_context).

      "movq %%rbx, 0(%0)\n\t"
      "movq %%rbp, 8(%0)\n\t"
      "movq %%r12, 16(%0)\n\t"
      "movq %%r13, 24(%0)\n\t"
      "movq %%r14, 32(%0)\n\t"
      "movq %%r15, 40(%0)\n\t"
      "movq %%rsp, 48(%0)\n\t"
      "leaq 1f(%%rip), %%rax\n\t" // Get address of label 1
      "movq %%rax, 56(%0)\n\t" // Save it as RIP (for consistency, though mostly
                               // unused in switch)

      // Load new context
      "movq 0(%1), %%rbx\n\t"
      "movq 8(%1), %%rbp\n\t"
      "movq 16(%1), %%r12\n\t"
      "movq 24(%1), %%r13\n\t"
      "movq 32(%1), %%r14\n\t"
      "movq 40(%1), %%r15\n\t"
      "movq 48(%1), %%rsp\n\t"

      // For newly created contexts, we might need to push RIP or jump.
      // If we use the 'ret' trick, the new stack must have the entry point at
      // the top. Let's assume 'make_context' sets up the stack correctly.

      "jmp *56(%1)\n\t" // Jump to saved RIP (or entry point)

      "1:\n\t"
      :
      : "r"(from), "r"(to)
      : "memory", "rax");
#endif
}

// Initialize a context
static void malphas_context_make(Context *ctx, void (*fn)(void *), void *arg,
                                 void *stack_base, size_t stack_size) {
  memset(ctx, 0, sizeof(Context));

  // Stack grows down
  uintptr_t sp = (uintptr_t)stack_base + stack_size;

  // Align stack to 16 bytes
  sp = sp & ~15;

#if defined(__aarch64__)
  ctx->sp = sp;
  ctx->fp = sp;             // Frame pointer starts at top of stack
  ctx->lr = (uint64_t)fn;   // Link register holds the function entry point
  ctx->x19 = (uint64_t)arg; // We'll use a wrapper to move arg to x0

  // We need a wrapper because 'fn' expects arg in x0, but context switch
  // restores callee-saved. We can put 'arg' in x19 (callee-saved) and have a
  // trampoline that moves x19 to x0. Or simpler: just set x0? No, x0 is not
  // callee-saved, it won't be restored by switch. So we MUST use a trampoline
  // or modify switch to restore x0 (bad). Let's use a trampoline helper.
#elif defined(__x86_64__)
  ctx->rsp = sp - 8; // Reserve space for return address (alignment)
  ctx->rip = (uint64_t)fn;
  ctx->rbx = (uint64_t)arg; // Pass arg in rbx (callee-saved)
#endif
}

// Trampoline to call the function with argument
static void legion_trampoline(void) {
#if defined(__aarch64__)
  // Recover arg from x19 (where we put it in make_context)
  // Recover fn from x20 (we'll store fn in x20 too)
  __asm__ volatile(
      "mov x0, x19\n\t" // arg
      "blr x20\n\t"     // call fn
                        // If fn returns, we should exit cleanly?
      // In our case, legion_entry calls the user fn and then handles death.
      // So 'fn' here is actually 'legion_entry', and 'arg' is 'Legion*'.
  );
#elif defined(__x86_64__)
  __asm__ volatile("movq %rbx, %rdi\n\t" // arg (System V ABI: first arg in rdi)
                   "callq *%r12\n\t"     // call fn (stored in r12)
  );
#endif
}

// Revised make_context to use trampoline
static void malphas_context_make_trampoline(Context *ctx, void (*fn)(void *),
                                            void *arg, void *stack_base,
                                            size_t stack_size) {
  memset(ctx, 0, sizeof(Context));
  uintptr_t sp = (uintptr_t)stack_base + stack_size;
  sp = sp & ~15;

#if defined(__aarch64__)
  ctx->sp = sp;
  ctx->fp = sp;
  ctx->lr = (uint64_t)legion_trampoline; // Start at trampoline
  ctx->x19 = (uint64_t)arg;              // arg for trampoline
  ctx->x20 = (uint64_t)fn;               // fn for trampoline
#elif defined(__x86_64__)
  ctx->rsp = sp - 8; // Alignment
  ctx->rip = (uint64_t)legion_trampoline;
  ctx->rbx = (uint64_t)arg; // arg
  ctx->r12 = (uint64_t)fn;  // fn
#endif
}
typedef struct Channel Channel;

// Legion structure - represents a spawned concurrent task
struct Legion {
  void (*fn)(void *);    // Function to execute
  void *arg;             // Argument to pass
  void *stack;           // Stack pointer (growable)
  void *stack_base;      // Base of allocated stack (for growth)
  size_t stack_size;     // Current stack size
  size_t stack_cap;      // Stack capacity
  Context ctx;           // Execution context (custom struct)
  Context ctx_saved;     // Saved context when blocked
  LegionState state;     // Current state
  struct Legion *next;   // For linked lists (run queue, etc.)
  pthread_cond_t cond;   // Condition variable for blocking
  pthread_mutex_t mutex; // Mutex for blocking operations
  Channel *blocked_on;   // Channel this legion is blocked on (if any)
  int id;                // Unique legion ID
  int thread_id;      // OS thread ID currently running this legion (-1 if none)
  int stack_overflow; // Flag for stack overflow detection
};

// Forward declaration for unblock_legion_from_channel (defined later)
static void unblock_legion_from_channel(Legion *legion);

// Channel implementation
struct Channel {
  void *buffer;              // Circular buffer
  size_t elem_size;          // Size of each element
  size_t capacity;           // Maximum capacity
  size_t head;               // Read position
  size_t tail;               // Write position
  size_t count;              // Number of elements
  pthread_mutex_t mutex;     // Mutex for synchronization
  pthread_cond_t not_full;   // Condition variable for not full
  pthread_cond_t not_empty;  // Condition variable for not empty
  atomic_int closed;         // 1 if closed, 0 otherwise
  Legion *blocked_senders;   // Linked list of blocked sending legions
  Legion *blocked_receivers; // Linked list of blocked receiving legions
};

Channel *runtime_channel_new(size_t elem_size, size_t capacity) {
  Channel *ch = (Channel *)runtime_alloc(sizeof(Channel));
  ch->elem_size = elem_size;
  ch->capacity = capacity;
  ch->head = 0;
  ch->tail = 0;
  ch->count = 0;
  ch->buffer = runtime_alloc(elem_size * capacity);
  pthread_mutex_init(&ch->mutex, NULL);
  pthread_cond_init(&ch->not_full, NULL);
  pthread_cond_init(&ch->not_empty, NULL);
  atomic_store(&ch->closed, 0);
  ch->blocked_senders = NULL;
  ch->blocked_receivers = NULL;
  return ch;
}

void runtime_channel_send(Channel *ch, void *value) {
  if (!ch)
    return;

  pthread_mutex_lock(&ch->mutex);

  // Wait until there's space or channel is closed
  while (ch->count >= ch->capacity && atomic_load(&ch->closed) == 0) {
    // If we're in a legion context, block the legion instead of using
    // pthread_cond_wait
    Legion *current = runtime_get_current_legion();
    if (current) {
      // Add to blocked senders list
      current->next = ch->blocked_senders;
      ch->blocked_senders = current;

      // Block the legion and release the channel mutex
      runtime_legion_block(current, ch);
      pthread_mutex_unlock(&ch->mutex);

      // Yield to scheduler (this will switch contexts)
      runtime_legion_yield();

      // When we resume, re-acquire the mutex
      pthread_mutex_lock(&ch->mutex);
      // Check again if we can proceed
      if (ch->count >= ch->capacity && atomic_load(&ch->closed) == 0) {
        continue; // Still blocked, wait again
      }
    } else {
      // Not in legion context, use traditional blocking
      pthread_cond_wait(&ch->not_full, &ch->mutex);
    }
  }

  // If closed, unlock and return
  if (atomic_load(&ch->closed) != 0) {
    pthread_mutex_unlock(&ch->mutex);
    return;
  }

  // Copy value into buffer
  void *dest = (char *)ch->buffer + (ch->tail * ch->elem_size);
  memcpy(dest, value, ch->elem_size);

  ch->tail = (ch->tail + 1) % ch->capacity;
  ch->count++;

  // Unblock a waiting receiver if any
  if (ch->blocked_receivers) {
    Legion *receiver = ch->blocked_receivers;
    ch->blocked_receivers = receiver->next;
    receiver->next = NULL;
    unblock_legion_from_channel(receiver);
  }

  // Signal that channel is not empty (wake up waiting legions/threads)
  pthread_cond_signal(&ch->not_empty);
  pthread_mutex_unlock(&ch->mutex);
}

void *runtime_channel_recv(Channel *ch) {
  if (!ch)
    return NULL;

  pthread_mutex_lock(&ch->mutex);

  // Wait until there's data or channel is closed
  while (ch->count == 0 && atomic_load(&ch->closed) == 0) {
    // If we're in a legion context, block the legion instead of using
    // pthread_cond_wait
    Legion *current = runtime_get_current_legion();
    if (current) {
      // Add to blocked receivers list
      current->next = ch->blocked_receivers;
      ch->blocked_receivers = current;

      // Block the legion and release the channel mutex
      runtime_legion_block(current, ch);
      pthread_mutex_unlock(&ch->mutex);

      // Yield to scheduler (this will switch contexts)
      runtime_legion_yield();

      // When we resume, re-acquire the mutex
      pthread_mutex_lock(&ch->mutex);
      // Check again if we can proceed
      if (ch->count == 0 && atomic_load(&ch->closed) == 0) {
        continue; // Still blocked, wait again
      }
    } else {
      // Not in legion context, use traditional blocking
      pthread_cond_wait(&ch->not_empty, &ch->mutex);
    }
  }

  // If closed and empty, return NULL
  if (ch->count == 0 && atomic_load(&ch->closed) != 0) {
    pthread_mutex_unlock(&ch->mutex);
    return NULL;
  }

  // Read value from buffer
  void *src = (char *)ch->buffer + (ch->head * ch->elem_size);
  void *result = runtime_alloc(ch->elem_size);
  memcpy(result, src, ch->elem_size);

  ch->head = (ch->head + 1) % ch->capacity;
  ch->count--;

  // Unblock a waiting sender if any
  if (ch->blocked_senders) {
    Legion *sender = ch->blocked_senders;
    ch->blocked_senders = sender->next;
    sender->next = NULL;
    unblock_legion_from_channel(sender);
  }

  // Signal that channel is not full (wake up waiting legions/threads)
  pthread_cond_signal(&ch->not_full);
  pthread_mutex_unlock(&ch->mutex);

  return result;
}

void runtime_channel_close(Channel *ch) {
  if (!ch)
    return;

  pthread_mutex_lock(&ch->mutex);
  atomic_store(&ch->closed, 1);
  // Wake up all waiting threads
  pthread_cond_broadcast(&ch->not_full);
  pthread_cond_broadcast(&ch->not_empty);
  pthread_mutex_unlock(&ch->mutex);
}

int8_t runtime_channel_is_closed(Channel *ch) {
  if (!ch)
    return 1;
  return (int8_t)atomic_load(&ch->closed);
}

// Non-blocking send: returns 1 if successful, 0 if would block
int8_t runtime_channel_try_send(Channel *ch, void *value) {
  if (!ch)
    return 0;

  pthread_mutex_lock(&ch->mutex);

  // If closed, unlock and return failure
  if (atomic_load(&ch->closed) != 0) {
    pthread_mutex_unlock(&ch->mutex);
    return 0;
  }

  // If full, unlock and return failure (non-blocking)
  if (ch->count >= ch->capacity) {
    pthread_mutex_unlock(&ch->mutex);
    return 0;
  }

  // Copy value into buffer
  void *dest = (char *)ch->buffer + (ch->tail * ch->elem_size);
  memcpy(dest, value, ch->elem_size);

  ch->tail = (ch->tail + 1) % ch->capacity;
  ch->count++;

  // Signal that channel is not empty
  pthread_cond_signal(&ch->not_empty);
  pthread_mutex_unlock(&ch->mutex);

  return 1;
}

// Non-blocking receive: returns 1 if successful, 0 if would block
// value is set to the received value if successful
int8_t runtime_channel_try_recv(Channel *ch, void **value) {
  if (!ch || !value)
    return 0;

  pthread_mutex_lock(&ch->mutex);

  // If empty and not closed, unlock and return failure (non-blocking)
  if (ch->count == 0 && atomic_load(&ch->closed) == 0) {
    pthread_mutex_unlock(&ch->mutex);
    return 0;
  }

  // If closed and empty, return failure
  if (ch->count == 0 && atomic_load(&ch->closed) != 0) {
    pthread_mutex_unlock(&ch->mutex);
    *value = NULL;
    return 0;
  }

  // Read value from buffer
  void *src = (char *)ch->buffer + (ch->head * ch->elem_size);
  void *result = runtime_alloc(ch->elem_size);
  memcpy(result, src, ch->elem_size);

  ch->head = (ch->head + 1) % ch->capacity;
  ch->count--;

  // Signal that channel is not full
  pthread_cond_signal(&ch->not_full);
  pthread_mutex_unlock(&ch->mutex);

  *value = result;
  return 1;
}

// Wait on condition variable for send to become possible
// NOTE: Caller must hold the channel's mutex before calling this
// The mutex will be released while waiting and re-acquired before returning
void runtime_channel_wait_for_send(Channel *ch) {
  if (!ch)
    return;
  // Mutex must already be held by caller
  while (ch->count >= ch->capacity && atomic_load(&ch->closed) == 0) {
    pthread_cond_wait(&ch->not_full, &ch->mutex);
  }
}

// Wait on condition variable for recv to become possible
// NOTE: Caller must hold the channel's mutex before calling this
// The mutex will be released while waiting and re-acquired before returning
void runtime_channel_wait_for_recv(Channel *ch) {
  if (!ch)
    return;
  // Mutex must already be held by caller
  while (ch->count == 0 && atomic_load(&ch->closed) == 0) {
    pthread_cond_wait(&ch->not_empty, &ch->mutex);
  }
}

// Sleep for specified nanoseconds (for select polling with timeout)
void runtime_nanosleep(long nanoseconds) {
  struct timespec req;
  req.tv_sec = nanoseconds / 1000000000L;
  req.tv_nsec = nanoseconds % 1000000000L;
  nanosleep(&req, NULL);
}

// ============================================================================
// Legion (M:N Threading Model) - Infernal Scheduler (continued)
// ============================================================================
// The Legion struct is defined above (before Channel) to allow Channel
// functions to access Legion members. Scheduler implementation continues below.

// Scheduler structure
typedef struct {
  pthread_t threads[MAX_OS_THREADS]; // OS thread pool
  int num_threads;                   // Actual number of threads
  Legion *run_queue[MAX_OS_THREADS]
                   [LEGION_QUEUE_SIZE]; // Per-thread run queues (work-stealing)
  atomic_int queue_head[MAX_OS_THREADS];       // Head of each queue (atomic for
                                               // work-stealing)
  atomic_int queue_tail[MAX_OS_THREADS];       // Tail of each queue (atomic for
                                               // work-stealing)
  pthread_mutex_t queue_mutex[MAX_OS_THREADS]; // Mutex for each queue
  pthread_cond_t queue_cond[MAX_OS_THREADS];   // Condition for queue
  atomic_int active_legions;                   // Number of active legions
  atomic_int shutdown;                         // Shutdown flag
  Legion *current_legion[MAX_OS_THREADS]; // Currently running legion per thread
  pthread_key_t thread_local_id;          // Thread-local storage for thread ID
} Scheduler;

static Scheduler *g_scheduler = NULL;
static atomic_int g_legion_id_counter = 0;

// Thread-local storage for current thread ID
static void init_thread_local_id(void) {
  pthread_key_create(&g_scheduler->thread_local_id, NULL);
}

static int get_thread_id(void) {
  void *id = pthread_getspecific(g_scheduler->thread_local_id);
  if (id == NULL) {
    return -1;
  }
  return (int)(intptr_t)id;
}

static void set_thread_id(int id) {
  pthread_setspecific(g_scheduler->thread_local_id, (void *)(intptr_t)id);
}

// Get the currently running legion
Legion *runtime_get_current_legion(void) {
  if (!g_scheduler) {
    return NULL;
  }

  int thread_id = get_thread_id();
  if (thread_id < 0 || thread_id >= MAX_OS_THREADS) {
    return NULL;
  }

  return g_scheduler->current_legion[thread_id];
}

// Initialize the infernal scheduler
void runtime_scheduler_init(void) {
  if (g_scheduler != NULL) {
    return; // Already initialized
  }

  g_scheduler = (Scheduler *)runtime_alloc(sizeof(Scheduler));
  g_scheduler->num_threads = MAX_OS_THREADS;
  atomic_store(&g_scheduler->active_legions, 0);
  atomic_store(&g_scheduler->shutdown, 0);

  // Initialize thread-local storage
  pthread_key_create(&g_scheduler->thread_local_id, NULL);

  for (int i = 0; i < MAX_OS_THREADS; i++) {
    atomic_store(&g_scheduler->queue_head[i], 0);
    atomic_store(&g_scheduler->queue_tail[i], 0);
    pthread_mutex_init(&g_scheduler->queue_mutex[i], NULL);
    pthread_cond_init(&g_scheduler->queue_cond[i], NULL);
    g_scheduler->current_legion[i] = NULL;
  }

  // Start OS thread pool
  for (int i = 0; i < MAX_OS_THREADS; i++) {
    int *thread_id = (int *)runtime_alloc(sizeof(int));
    *thread_id = i;
    pthread_create(&g_scheduler->threads[i], NULL,
                   (void *(*)(void *))runtime_scheduler_run, thread_id);
  }
}

// Allocate stack with guard pages for overflow detection
static void *allocate_stack_with_guard(size_t size) {
  // Allocate stack + guard pages
  size_t total_size = size + LEGION_STACK_GUARD_SIZE * 2;
  void *mem = mmap(NULL, total_size, PROT_READ | PROT_WRITE,
                   MAP_PRIVATE | MAP_ANONYMOUS, -1, 0);
  if (mem == MAP_FAILED) {
    return NULL;
  }

  // Protect guard pages
  mprotect(mem, LEGION_STACK_GUARD_SIZE, PROT_NONE);
  mprotect((char *)mem + size + LEGION_STACK_GUARD_SIZE,
           LEGION_STACK_GUARD_SIZE, PROT_NONE);

  // Return pointer to usable stack area (after first guard page)
  return (char *)mem + LEGION_STACK_GUARD_SIZE;
}

// Create a new legion (spawned entity)
Legion *runtime_legion_spawn(void (*fn)(void *), void *arg, size_t stack_size) {
  if (stack_size == 0) {
    stack_size = LEGION_STACK_SIZE;
  }
  if (stack_size > LEGION_STACK_MAX) {
    stack_size = LEGION_STACK_MAX;
  }

  Legion *legion = (Legion *)runtime_alloc(sizeof(Legion));
  legion->fn = fn;
  legion->arg = arg;
  legion->stack_size = stack_size;
  legion->stack_cap = stack_size;

  // Allocate stack with guard pages
  legion->stack_base = allocate_stack_with_guard(stack_size);
  if (!legion->stack_base) {
    // Fallback to regular allocation
    legion->stack_base = runtime_alloc(stack_size);
    legion->stack = legion->stack_base;
  } else {
    legion->stack = legion->stack_base;
  }

  legion->state = LEGION_STATE_RUNNABLE;
  legion->next = NULL;
  legion->id = atomic_fetch_add(&g_legion_id_counter, 1);
  legion->thread_id = -1;
  legion->stack_overflow = 0;
  legion->blocked_on = NULL;
  pthread_cond_init(&legion->cond, NULL);
  pthread_mutex_init(&legion->mutex, NULL);

  // Initialize context
  pthread_mutex_init(&legion->mutex, NULL);

  // Initialize context
  malphas_context_make_trampoline(&legion->ctx, (void (*)(void *))legion_entry,
                                  legion, legion->stack, stack_size);

  return legion;

  return legion;
}

// Push legion to local queue (lock-free, called by owner thread)
static int push_to_local_queue(int thread_id, Legion *legion) {
  int tail = atomic_load(&g_scheduler->queue_tail[thread_id]);
  int next_tail = (tail + 1) % LEGION_QUEUE_SIZE;

  // Check if queue is full
  if (next_tail == atomic_load(&g_scheduler->queue_head[thread_id])) {
    return 0; // Queue full
  }

  g_scheduler->run_queue[thread_id][tail] = legion;
  atomic_store(&g_scheduler->queue_tail[thread_id], next_tail);
  return 1;
}

// Pop legion from local queue (lock-free, called by owner thread)
static Legion *pop_from_local_queue(int thread_id) {
  int head = atomic_load(&g_scheduler->queue_head[thread_id]);
  int tail = atomic_load(&g_scheduler->queue_tail[thread_id]);

  if (head == tail) {
    return NULL; // Queue empty
  }

  Legion *legion = g_scheduler->run_queue[thread_id][head];
  atomic_store(&g_scheduler->queue_head[thread_id],
               (head + 1) % LEGION_QUEUE_SIZE);
  return legion;
}

// Steal from another thread's queue (lock-free work-stealing)
static Legion *steal_from_queue(int victim_thread_id) {
  int head = atomic_load(&g_scheduler->queue_head[victim_thread_id]);
  int tail = atomic_load(&g_scheduler->queue_tail[victim_thread_id]);

  if (head == tail) {
    return NULL; // Queue empty
  }

  // Try to increment head atomically
  int expected = head;
  int next_head = (head + 1) % LEGION_QUEUE_SIZE;
  if (atomic_compare_exchange_strong(&g_scheduler->queue_head[victim_thread_id],
                                     &expected, next_head)) {
    return g_scheduler->run_queue[victim_thread_id][head];
  }

  return NULL; // Failed to steal (race condition)
}

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

// Start a legion (add to scheduler)
void runtime_legion_start(Legion *legion) {
  if (!legion || !g_scheduler) {
    return;
  }

  atomic_fetch_add(&g_scheduler->active_legions, 1);

  // Add to a queue for load balancing - use load-aware distribution
  // instead of simple round-robin for better thread utilization
  int queue_idx = find_least_loaded_thread();

  // Try lock-free push first
  if (!push_to_local_queue(queue_idx, legion)) {
    // Queue full, use mutex-protected path
    pthread_mutex_lock(&g_scheduler->queue_mutex[queue_idx]);
    int tail = atomic_load(&g_scheduler->queue_tail[queue_idx]);
    int next_tail = (tail + 1) % LEGION_QUEUE_SIZE;
    if (next_tail != atomic_load(&g_scheduler->queue_head[queue_idx])) {
      g_scheduler->run_queue[queue_idx][tail] = legion;
      atomic_store(&g_scheduler->queue_tail[queue_idx], next_tail);
    }
    pthread_cond_signal(&g_scheduler->queue_cond[queue_idx]);
    pthread_mutex_unlock(&g_scheduler->queue_mutex[queue_idx]);
  } else {
    // Signal waiting thread
    pthread_mutex_lock(&g_scheduler->queue_mutex[queue_idx]);
    pthread_cond_signal(&g_scheduler->queue_cond[queue_idx]);
    pthread_mutex_unlock(&g_scheduler->queue_mutex[queue_idx]);
  }
}

// Scheduler context storage (per thread)
static __thread Context g_scheduler_context;

// Set scheduler context for current thread
static void set_scheduler_context(Context *ctx) { g_scheduler_context = *ctx; }

// Get scheduler context for current thread
static Context *get_scheduler_context(void) { return &g_scheduler_context; }

// Legion entry point (called when context is switched to)
static void legion_entry(Legion *legion) {
  // Set up signal handler for stack overflow
  struct sigaction sa;
  sa.sa_handler = SIG_DFL;
  sigemptyset(&sa.sa_mask);
  sa.sa_flags = SA_ONSTACK;
  sigaction(SIGSEGV, &sa, NULL);

  // Execute the function
  legion->fn(legion->arg);

  // Function completed - mark as dead
  legion->state = LEGION_STATE_DEAD;
  atomic_fetch_sub(&g_scheduler->active_legions, 1);

  // Switch back to scheduler
  int thread_id = legion->thread_id;
  if (thread_id >= 0 && thread_id < MAX_OS_THREADS) {
    g_scheduler->current_legion[thread_id] = NULL;
    legion->thread_id = -1;
  }

  // Return to scheduler (context switch)
  // Return to scheduler (context switch)
  Context *scheduler_ctx = get_scheduler_context();
  if (scheduler_ctx) {
    // We don't have a 'from' context here because we are dying.
    // But context_switch expects one. We can use a dummy or the legion's ctx.
    // Since the legion is dead, its ctx won't be used again.
    malphas_context_switch(&legion->ctx, scheduler_ctx);
  }
}

// Grow stack if needed
static int grow_legion_stack(Legion *legion) {
  if (legion->stack_size >= LEGION_STACK_MAX) {
    return 0; // Already at max size
  }

  size_t new_size = legion->stack_size * 2;
  if (new_size > LEGION_STACK_MAX) {
    new_size = LEGION_STACK_MAX;
  }

  // For now, we'll use a simple approach: reallocate
  // In a production system, you'd use virtual memory tricks
  void *new_stack = runtime_alloc(new_size);
  if (!new_stack) {
    return 0;
  }

  // Copy old stack to new (simplified - real implementation would be more
  // complex)
  memcpy(new_stack, legion->stack, legion->stack_size);

  legion->stack = new_stack;
  legion->stack_base = new_stack;
  legion->stack_size = new_size;
  legion->stack_cap = new_size;

  // Update context
  // Update context
  // For custom context, we just need to update SP if we were running?
  // But we only grow when NOT running (or when we can migrate).
  // Actually, growing stack with custom context is harder because SP is inside
  // the opaque blob. For now, let's disable stack growth or just reset the
  // context if it's not running.

  // If we are here, we are likely re-initializing.
  // But wait, 'grow_legion_stack' is not called in the current code?
  // It seems unused or internal.
  // If we simply update the stack pointer in the struct, it might be enough if
  // we reset the context. But preserving execution state across stack move is
  // very hard without ucontext. Let's just update the struct fields for now.

  // Re-initialize context if we are just setting it up?
  // No, this function copies stack.
  // With custom context, the SP in 'ctx' points to the OLD stack.
  // We need to adjust it by the offset.
  long offset = (char *)new_stack - (char *)legion->stack;

#if defined(__aarch64__)
  legion->ctx.sp += offset;
  legion->ctx.fp += offset;
#elif defined(__x86_64__)
  legion->ctx.rsp += offset;
  legion->ctx.rbp += offset;
#endif

  return 1;
}

// Yield control to scheduler (cooperative)
void runtime_legion_yield(void) {
  int thread_id = get_thread_id();
  if (thread_id < 0 || thread_id >= MAX_OS_THREADS) {
    return; // Not running in scheduler context
  }

  Legion *current = g_scheduler->current_legion[thread_id];
  if (!current) {
    return;
  }

  // Save current context
  // We do this by switching TO the scheduler, which saves FROM (current).
  // So we don't need explicit getcontext here.

  // Save context and switch back to scheduler
  // The scheduler will resume this legion later

  // Save context and switch back to scheduler
  // The scheduler will resume this legion later
  current->state = LEGION_STATE_RUNNABLE;

  // Push back to queue
  if (!push_to_local_queue(thread_id, current)) {
    // Queue full, try another queue
    for (int i = 0; i < MAX_OS_THREADS; i++) {
      if (i != thread_id && push_to_local_queue(i, current)) {
        break;
      }
    }
  }

  g_scheduler->current_legion[thread_id] = NULL;
  current->thread_id = -1;

  // Context switch back to scheduler
  // Context switch back to scheduler
  Context *scheduler_ctx = get_scheduler_context();
  if (scheduler_ctx) {
    malphas_context_switch(&current->ctx_saved, scheduler_ctx);
  }
}

// Block a legion (called when blocking on channel)
void runtime_legion_block(Legion *legion, Channel *channel) {
  if (!legion)
    return;

  int thread_id = get_thread_id();
  if (thread_id >= 0 && thread_id < MAX_OS_THREADS) {
    g_scheduler->current_legion[thread_id] = NULL;
    legion->thread_id = -1;
  }

  legion->state = LEGION_STATE_BLOCKED;
  legion->blocked_on = channel;

  atomic_fetch_sub(&g_scheduler->active_legions, 1);
}

// Unblock a blocked legion and add it back to the scheduler
static void unblock_legion_from_channel(Legion *legion) {
  if (!legion || legion->state != LEGION_STATE_BLOCKED) {
    return;
  }

  legion->state = LEGION_STATE_RUNNABLE;
  legion->blocked_on = NULL;
  atomic_fetch_add(&g_scheduler->active_legions, 1);

  // Add back to scheduler
  runtime_legion_start(legion);
}

// Unblock a legion (called when channel operation completes)
void runtime_legion_unblock(Legion *legion) {
  if (!legion || legion->state != LEGION_STATE_BLOCKED) {
    return;
  }

  legion->state = LEGION_STATE_RUNNABLE;
  legion->blocked_on = NULL;
  atomic_fetch_add(&g_scheduler->active_legions, 1);

  // Add back to scheduler
  runtime_legion_start(legion);
}

// Scheduler main loop (runs on each OS thread)
void *runtime_scheduler_run(void *arg) {
  int thread_id = *(int *)arg;
  runtime_alloc(sizeof(int)); // Free the arg (GC will handle it)
  set_thread_id(thread_id);

  runtime_alloc(sizeof(int)); // Free the arg (GC will handle it)
  set_thread_id(thread_id);

  Context scheduler_ctx;
  // We don't need to initialize scheduler_ctx, it will be populated by
  // context_switch
  set_scheduler_context(&scheduler_ctx);

  while (!atomic_load(&g_scheduler->shutdown)) {
    Legion *legion = NULL;

    // 1. Try to pop from local queue
    legion = pop_from_local_queue(thread_id);

    // 2. If local queue empty, try work-stealing
    if (!legion) {
      for (int attempt = 0; attempt < WORK_STEAL_ATTEMPTS; attempt++) {
        int victim = (thread_id + attempt + 1) % MAX_OS_THREADS;
        legion = steal_from_queue(victim);
        if (legion) {
          break;
        }
      }
    }

    // 3. If still no work, wait on condition variable
    if (!legion) {
      pthread_mutex_lock(&g_scheduler->queue_mutex[thread_id]);

      // Double-check queue is still empty
      legion = pop_from_local_queue(thread_id);
      if (!legion) {
        // Wait for work or shutdown
        struct timespec timeout;
        clock_gettime(CLOCK_REALTIME, &timeout);
        timeout.tv_nsec += 10000000; // 10ms timeout
        if (timeout.tv_nsec >= 1000000000) {
          timeout.tv_sec++;
          timeout.tv_nsec -= 1000000000;
        }

        pthread_cond_timedwait(&g_scheduler->queue_cond[thread_id],
                               &g_scheduler->queue_mutex[thread_id], &timeout);

        // Try one more time after wakeup
        legion = pop_from_local_queue(thread_id);
      }

      pthread_mutex_unlock(&g_scheduler->queue_mutex[thread_id]);
    }

    // 4. Execute legion if we have one
    if (legion && legion->state == LEGION_STATE_RUNNABLE) {
      g_scheduler->current_legion[thread_id] = legion;
      legion->thread_id = thread_id;
      legion->state = LEGION_STATE_RUNNING;

      // Set up context for legion execution
      // It's already set up by spawn or previous yield

      // Save scheduler context and switch to legion
      Context *saved_ctx = &scheduler_ctx;
      malphas_context_switch(saved_ctx, &legion->ctx);

      // We return here when legion yields or completes
      // The legion may have yielded, so check if it's still current
      if (g_scheduler->current_legion[thread_id] == legion) {
        g_scheduler->current_legion[thread_id] = NULL;
        legion->thread_id = -1;

        // If legion completed, it already decremented active_legions
        // If it yielded, it should have been re-queued
        if (legion->state == LEGION_STATE_DEAD) {
          // Legion completed - nothing to do
        } else if (legion->state == LEGION_STATE_RUNNABLE) {
          // Legion yielded - already re-queued
        }
      }
    } else if (!legion && atomic_load(&g_scheduler->active_legions) == 0) {
      // No active legions, sleep a bit
      usleep(1000);
    }
  }

  return NULL;
}

// Shutdown scheduler
void runtime_scheduler_shutdown(void) {
  if (!g_scheduler) {
    return;
  }

  atomic_store(&g_scheduler->shutdown, 1);

  // Wait for all threads
  for (int i = 0; i < g_scheduler->num_threads; i++) {
    pthread_join(g_scheduler->threads[i], NULL);
  }

  // Cleanup
  for (int i = 0; i < MAX_OS_THREADS; i++) {
    pthread_mutex_destroy(&g_scheduler->queue_mutex[i]);
    pthread_cond_destroy(&g_scheduler->queue_cond[i]);
  }
}
