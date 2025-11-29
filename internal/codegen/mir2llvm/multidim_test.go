package mir2llvm

import (
	"strings"
	"testing"

	"github.com/malphas-lang/malphas-lang/internal/mir"
	"github.com/malphas-lang/malphas-lang/internal/types"
)

// TestMultiDimensionalIndexing_2D tests 2D array indexing
func TestMultiDimensionalIndexing_2D(t *testing.T) {
	gen := NewGenerator()

	// Create a 2D slice type: [][]int
	elemType := &types.Slice{Elem: &types.Primitive{Kind: types.Int}}
	arrType := &types.Slice{Elem: elemType}

	// Create locals
	arr := mir.Local{ID: 0, Name: "arr", Type: arrType}
	result := mir.Local{ID: 1, Name: "result", Type: &types.Primitive{Kind: types.Int}}

	// Test LoadIndex with 2 indices
	loadIndex := &mir.LoadIndex{
		Result: result,
		Target: &mir.LocalRef{Local: arr},
		Indices: []mir.Operand{
			&mir.Literal{Type: &types.Primitive{Kind: types.Int}, Value: int64(0)},
			&mir.Literal{Type: &types.Primitive{Kind: types.Int}, Value: int64(1)},
		},
	}

	gen.localRegs[arr.ID] = "%reg0"

	err := gen.generateLoadIndex(loadIndex)
	if err != nil {
		t.Fatalf("generateLoadIndex() error = %v", err)
	}

	output := gen.builder.String()

	// Verify it calls runtime_slice_get twice
	sliceGetCount := strings.Count(output, "@runtime_slice_get")
	if sliceGetCount != 2 {
		t.Errorf("Expected 2 calls to runtime_slice_get for 2D indexing, got %d. Output:\n%s", sliceGetCount, output)
	}

	// Verify first call uses index 0
	if !strings.Contains(output, "runtime_slice_get(%Slice* %reg0, i64 0)") {
		t.Errorf("Expected first call with index 0. Output:\n%s", output)
	}

	// Verify second call uses index 1
	if !strings.Contains(output, ", i64 1)") {
		t.Errorf("Expected second call with index 1. Output:\n%s", output)
	}

	// Verify there's a bitcast between the two calls for traversing the nested slice
	if !strings.Contains(output, "bitcast i8*") && !strings.Contains(output, "to %Slice*") {
		t.Errorf("Expected bitcast for nested slice traversal. Output:\n%s", output)
	}
}

// TestMultiDimensionalIndexing_3D tests 3D array indexing
func TestMultiDimensionalIndexing_3D(t *testing.T) {
	gen := NewGenerator()

	// Create a 3D slice type: [][][]int
	innerType := &types.Primitive{Kind: types.Int}
	level1 := &types.Slice{Elem: innerType}
	level2 := &types.Slice{Elem: level1}
	arrType := &types.Slice{Elem: level2}

	// Create locals
	arr := mir.Local{ID: 0, Name: "arr", Type: arrType}
	result := mir.Local{ID: 1, Name: "result", Type: &types.Primitive{Kind: types.Int}}

	// Test LoadIndex with 3 indices
	loadIndex := &mir.LoadIndex{
		Result: result,
		Target: &mir.LocalRef{Local: arr},
		Indices: []mir.Operand{
			&mir.Literal{Type: &types.Primitive{Kind: types.Int}, Value: int64(0)},
			&mir.Literal{Type: &types.Primitive{Kind: types.Int}, Value: int64(1)},
			&mir.Literal{Type: &types.Primitive{Kind: types.Int}, Value: int64(2)},
		},
	}

	gen.localRegs[arr.ID] = "%reg0"

	err := gen.generateLoadIndex(loadIndex)
	if err != nil {
		t.Fatalf("generateLoadIndex() error = %v", err)
	}

	output := gen.builder.String()

	// Verify it calls runtime_slice_get three times
	sliceGetCount := strings.Count(output, "@runtime_slice_get")
	if sliceGetCount != 3 {
		t.Errorf("Expected 3 calls to runtime_slice_get for 3D indexing, got %d. Output:\n%s", sliceGetCount, output)
	}

	// Verify all three indices are used
	if !strings.Contains(output, ", i64 0)") {
		t.Errorf("Expected index 0. Output:\n%s", output)
	}
	if !strings.Contains(output, ", i64 1)") {
		t.Errorf("Expected index 1. Output:\n%s", output)
	}
	if !strings.Contains(output, ", i64 2)") {
		t.Errorf("Expected index 2. Output:\n%s", output)
	}

	// Verify there are bitcasts for nested slice traversal
	bitcastCount := strings.Count(output, "bitcast i8*")
	if bitcastCount < 2 {
		t.Errorf("Expected at least 2 bitcasts for 3D traversal, got %d. Output:\n%s", bitcastCount, output)
	}
}

// TestMultiDimensionalIndexing_Store2D tests storing to a 2D array
func TestMultiDimensionalIndexing_Store2D(t *testing.T) {
	gen := NewGenerator()

	// Create a 2D slice type: [][]int
	elemType := &types.Slice{Elem: &types.Primitive{Kind: types.Int}}
	arrType := &types.Slice{Elem: elemType}

	// Create locals
	arr := mir.Local{ID: 0, Name: "arr", Type: arrType}
	value := mir.Local{ID: 1, Name: "value", Type: &types.Primitive{Kind: types.Int}}

	// Test StoreIndex with 2 indices
	storeIndex := &mir.StoreIndex{
		Target: &mir.LocalRef{Local: arr},
		Indices: []mir.Operand{
			&mir.Literal{Type: &types.Primitive{Kind: types.Int}, Value: int64(3)},
			&mir.Literal{Type: &types.Primitive{Kind: types.Int}, Value: int64(4)},
		},
		Value: &mir.LocalRef{Local: value},
	}

	gen.localRegs[arr.ID] = "%reg0"
	gen.localRegs[value.ID] = "%reg1"

	err := gen.generateStoreIndex(storeIndex)
	if err != nil {
		t.Fatalf("generateStoreIndex() error = %v", err)
	}

	output := gen.builder.String()

	// Verify it calls runtime_slice_get once (for traversal) and runtime_slice_set once (for the store)
	sliceGetCount := strings.Count(output, "@runtime_slice_get")
	sliceSetCount := strings.Count(output, "@runtime_slice_set")

	if sliceGetCount != 1 {
		t.Errorf("Expected 1 call to runtime_slice_get for 2D store traversal, got %d. Output:\n%s", sliceGetCount, output)
	}
	if sliceSetCount != 1 {
		t.Errorf("Expected 1 call to runtime_slice_set for 2D store, got %d. Output:\n%s", sliceSetCount, output)
	}

	// Verify indices are used correctly
	if !strings.Contains(output, ", i64 3") {
		t.Errorf("Expected first index 3. Output:\n%s", output)
	}
	if !strings.Contains(output, ", i64 4") {
		t.Errorf("Expected second index 4. Output:\n%s", output)
	}
}

// TestMultiDimensionalIndexing_DynamicIndices tests with variable indices instead of literals
func TestMultiDimensionalIndexing_DynamicIndices(t *testing.T) {
	gen := NewGenerator()

	// Create a 2D slice type: [][]int
	elemType := &types.Slice{Elem: &types.Primitive{Kind: types.Int}}
	arrType := &types.Slice{Elem: elemType}

	// Create locals
	arr := mir.Local{ID: 0, Name: "arr", Type: arrType}
	idx1 := mir.Local{ID: 1, Name: "i", Type: &types.Primitive{Kind: types.Int}}
	idx2 := mir.Local{ID: 2, Name: "j", Type: &types.Primitive{Kind: types.Int}}
	result := mir.Local{ID: 3, Name: "result", Type: &types.Primitive{Kind: types.Int}}

	// Test LoadIndex with variable indices
	loadIndex := &mir.LoadIndex{
		Result: result,
		Target: &mir.LocalRef{Local: arr},
		Indices: []mir.Operand{
			&mir.LocalRef{Local: idx1},
			&mir.LocalRef{Local: idx2},
		},
	}

	gen.localRegs[arr.ID] = "%reg0"
	gen.localRegs[idx1.ID] = "%reg1"
	gen.localRegs[idx2.ID] = "%reg2"

	err := gen.generateLoadIndex(loadIndex)
	if err != nil {
		t.Fatalf("generateLoadIndex() error = %v", err)
	}

	output := gen.builder.String()

	// Verify it calls runtime_slice_get twice
	sliceGetCount := strings.Count(output, "@runtime_slice_get")
	if sliceGetCount != 2 {
		t.Errorf("Expected 2 calls to runtime_slice_get, got %d. Output:\n%s", sliceGetCount, output)
	}

	// Verify variable indices are used (registers %reg1 and %reg2)
	if !strings.Contains(output, "%reg1") {
		t.Errorf("Expected first index register %%reg1. Output:\n%s", output)
	}
	if !strings.Contains(output, "%reg2") {
		t.Errorf("Expected second index register %%reg2. Output:\n%s", output)
	}
}
