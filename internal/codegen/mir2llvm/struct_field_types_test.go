package mir2llvm

import (
	"strings"
	"testing"

	"github.com/malphas-lang/malphas-lang/internal/mir"
	"github.com/malphas-lang/malphas-lang/internal/types"
)

// TestStructFieldTypes tests that struct fields use correct types instead of hardcoded i64
func TestStructFieldTypes_MixedTypes(t *testing.T) {
	gen := newTestGenerator()

	// Define a struct type with mixed field types
	// struct Point { x: float, y: float, active: bool }
	pointStruct := &types.Struct{
		Name: "Point",
		Fields: []types.Field{
			{Name: "x", Type: types.TypeFloat},
			{Name: "y", Type: types.TypeFloat},
			{Name: "active", Type: types.TypeBool},
		},
	}

	// Create result local with the struct type
	result := mir.Local{ID: 1, Name: "p", Type: pointStruct}

	// Create field values
	xVal := &mir.Literal{Type: types.TypeFloat, Value: float64(1.5)}
	yVal := &mir.Literal{Type: types.TypeFloat, Value: float64(2.5)}
	activeVal := &mir.Literal{Type: types.TypeBool, Value: true}

	cons := &mir.ConstructStruct{
		Result: result,
		Type:   "Point",
		Fields: map[string]mir.Operand{
			"x":      xVal,
			"y":      yVal,
			"active": activeVal,
		},
	}

	err := gen.generateConstructStruct(cons)
	if err != nil {
		t.Fatalf("generateConstructStruct() error = %v", err)
	}

	output := gen.builder.String()

	// Check that float fields use 'double' type, not i64
	if !strings.Contains(output, "store double") {
		t.Errorf("Expected 'store double' for float fields, got:\n%s", output)
	}

	// Check that bool field uses 'i1' type, not i64
	if !strings.Contains(output, "store i1") {
		t.Errorf("Expected 'store i1' for bool field, got:\n%s", output)
	}

	// Should NOT contain hardcoded i64 stores for these fields
	// (Note: there might be i64 for indices in getelementptr, which is OK)
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "store i64") && !strings.Contains(line, "getelementptr") {
			t.Errorf("Found hardcoded 'store i64' which should not be there:\n%s", line)
		}
	}
}

func TestStructFieldTypes_StoreField(t *testing.T) {
	gen := newTestGenerator()

	// Define a struct type: struct Data { count: int, ratio: float, valid: bool }
	dataStruct := &types.Struct{
		Name: "Data",
		Fields: []types.Field{
			{Name: "count", Type: types.TypeInt},
			{Name: "ratio", Type: types.TypeFloat},
			{Name: "valid", Type: types.TypeBool},
		},
	}

	// Create target local
	targetLocal := mir.Local{ID: 1, Name: "d", Type: dataStruct}
	targetRef := &mir.LocalRef{Local: targetLocal}

	// Allocate the local first
	gen.localRegs[1] = "%reg0"
	gen.emit("  %reg0 = alloca %struct.Data*")

	// Test storing a float field
	floatVal := &mir.Literal{Type: types.TypeFloat, Value: float64(3.14)}
	storeRatio := &mir.StoreField{
		Target: targetRef,
		Field:  "ratio",
		Value:  floatVal,
	}

	err := gen.generateStoreField(storeRatio)
	if err != nil {
		t.Fatalf("generateStoreField() error = %v", err)
	}

	output := gen.builder.String()

	// Should use double for the float field
	if !strings.Contains(output, "store double") {
		t.Errorf("Expected 'store double' for float field, got:\n%s", output)
	}

	// Reset for next test
	gen.builder.Reset()
	gen.emit("  %reg0 = alloca %struct.Data*")

	// Test storing a bool field
	boolVal := &mir.Literal{Type: types.TypeBool, Value: true}
	storeValid := &mir.StoreField{
		Target: targetRef,
		Field:  "valid",
		Value:  boolVal,
	}

	err = gen.generateStoreField(storeValid)
	if err != nil {
		t.Fatalf("generateStoreField() error = %v", err)
	}

	output = gen.builder.String()

	// Should use i1 for the bool field
	if !strings.Contains(output, "store i1") {
		t.Errorf("Expected 'store i1' for bool field, got:\n%s", output)
	}
}

func TestStructFieldTypes_StringField(t *testing.T) {
	gen := newTestGenerator()

	// struct Person { name: string, age: int }
	personStruct := &types.Struct{
		Name: "Person",
		Fields: []types.Field{
			{Name: "name", Type: types.TypeString},
			{Name: "age", Type: types.TypeInt},
		},
	}

	result := mir.Local{ID: 1, Name: "person", Type: personStruct}

	nameVal := &mir.Literal{Type: types.TypeString, Value: "Alice"}
	ageVal := &mir.Literal{Type: types.TypeInt, Value: int64(30)}

	cons := &mir.ConstructStruct{
		Result: result,
		Type:   "Person",
		Fields: map[string]mir.Operand{
			"name": nameVal,
			"age":  ageVal,
		},
	}

	err := gen.generateConstructStruct(cons)
	if err != nil {
		t.Fatalf("generateConstructStruct() error = %v", err)
	}

	output := gen.builder.String()

	// Should use %String* for string field
	if !strings.Contains(output, "store %String*") {
		t.Errorf("Expected 'store %%String*' for string field, got:\n%s", output)
	}

	// Should use i64 for int field
	if !strings.Contains(output, "store i64") {
		t.Errorf("Expected 'store i64' for int field, got:\n%s", output)
	}
}
