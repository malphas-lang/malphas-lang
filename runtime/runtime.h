// runtime/runtime.h
// Malphas Runtime Library Header

#ifndef RUNTIME_H
#define RUNTIME_H

#include <stdint.h>
#include <stddef.h>
#include <stdio.h>
#include <string.h>
#include <stdlib.h>
#include <gc/gc.h>  // Boehm GC

// String type
typedef struct {
    size_t len;
    char* data;
} String;

// Slice type (generic, used for Vec)
typedef struct {
    void* data;
    size_t len;
    size_t cap;
    size_t elem_size;
} Slice;

// HashMap type (simplified)
typedef struct HashMap HashMap;

// Channel type
typedef struct Channel Channel;

// Legion (user-level concurrent entity, spawned by spawn keyword) type
// Named after the demonic host - many legions can run concurrently
typedef struct Legion Legion;

// Garbage collector initialization
void runtime_gc_init(void);

// Memory allocation
void* runtime_alloc(size_t size);

// String operations
String* runtime_string_new(const char* data, size_t len);
void runtime_string_free(String* s);
const char* runtime_string_cstr(String* s);
int runtime_string_equal(String* a, String* b);  // Returns 1 if equal, 0 otherwise
String* runtime_string_concat(String* a, String* b);  // Concatenate two strings
String* runtime_string_from_i64(int64_t value);  // Convert int64 to string
String* runtime_string_from_double(double value);  // Convert double to string
String* runtime_string_from_bool(int8_t value);  // Convert bool to string
String* runtime_string_format(String* fmt, String* arg1, String* arg2, String* arg3, String* arg4);  // Format string with {} placeholders

// Print functions
void runtime_println_i64(int64_t value);
void runtime_println_i32(int32_t value);
void runtime_println_i8(int8_t value);
void runtime_println_double(double value);
void runtime_println_bool(int8_t value);  // i1 in LLVM, int8_t in C
void runtime_println_string(String* s);

// Slice operations (for Vec)
Slice* runtime_slice_new(size_t elem_size, size_t len, size_t cap);
void* runtime_slice_get(Slice* slice, size_t index);
void runtime_slice_set(Slice* slice, size_t index, void* value);
void runtime_slice_push(Slice* slice, void* value);
size_t runtime_slice_len(Slice* slice);
int8_t runtime_slice_is_empty(Slice* slice);  // Returns 1 if empty, 0 otherwise
size_t runtime_slice_cap(Slice* slice);  // Get capacity
void runtime_slice_reserve(Slice* slice, size_t additional);  // Reserve additional capacity
void runtime_slice_clear(Slice* slice);  // Clear all elements (set len to 0)
void* runtime_slice_pop(Slice* slice);  // Remove and return last element (returns NULL if empty)
void runtime_slice_remove(Slice* slice, size_t index);  // Remove element at index
void runtime_slice_insert(Slice* slice, size_t index, void* value);  // Insert element at index
Slice* runtime_slice_copy(Slice* slice);  // Create a copy of the slice
Slice* runtime_slice_subslice(Slice* slice, size_t start, size_t end);  // Create sub-slice [start:end)

// HashMap operations
HashMap* runtime_hashmap_new(void);
void runtime_hashmap_put(HashMap* map, String* key, void* value);
void* runtime_hashmap_get(HashMap* map, String* key);
int8_t runtime_hashmap_contains_key(HashMap* map, String* key);  // Returns 1 if key exists, 0 otherwise
size_t runtime_hashmap_len(HashMap* map);  // Returns the number of key-value pairs
int8_t runtime_hashmap_is_empty(HashMap* map);  // Returns 1 if empty, 0 otherwise
void runtime_hashmap_free(HashMap* map);

// Channel operations
Channel* runtime_channel_new(size_t elem_size, size_t capacity);  // Create a new channel
void runtime_channel_send(Channel* ch, void* value);  // Send a value to channel (blocks if full)
void* runtime_channel_recv(Channel* ch);  // Receive a value from channel (blocks if empty)
void runtime_channel_close(Channel* ch);  // Close the channel
int8_t runtime_channel_is_closed(Channel* ch);  // Returns 1 if closed, 0 otherwise
int8_t runtime_channel_try_send(Channel* ch, void* value);  // Try to send (non-blocking), returns 1 if successful, 0 if would block
int8_t runtime_channel_try_recv(Channel* ch, void** value);  // Try to receive (non-blocking), returns 1 if successful, 0 if would block
void runtime_channel_wait_for_send(Channel* ch);  // Wait on condition variable for send to become possible (must hold mutex)
void runtime_channel_wait_for_recv(Channel* ch);  // Wait on condition variable for recv to become possible (must hold mutex)
void runtime_nanosleep(long nanoseconds);  // Sleep for specified nanoseconds (for select polling with timeout)

// Legion and scheduler operations
void runtime_scheduler_init(void);  // Initialize the infernal scheduler (call once at startup)
Legion* runtime_legion_spawn(void (*fn)(void*), void* arg, size_t stack_size);  // Spawn a new legion (from spawn keyword)
void runtime_legion_start(Legion* legion);  // Start a legion (add to scheduler)
void runtime_legion_yield(void);  // Yield control to scheduler (cooperative)
void* runtime_scheduler_run(void* arg);  // Run the infernal scheduler (called by OS threads)
void runtime_scheduler_shutdown(void);  // Shutdown scheduler (wait for all legions to complete)
Legion* runtime_get_current_legion(void);  // Get the currently running legion (NULL if not in legion context)
void runtime_legion_block(Legion* legion, Channel* channel);  // Block a legion on a channel
void runtime_legion_unblock(Legion* legion);  // Unblock a legion

#endif // RUNTIME_H

