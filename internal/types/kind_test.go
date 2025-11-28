package types

import "testing"

func TestKindEquality(t *testing.T) {
	tests := []struct {
		name     string
		k1       Kind
		k2       Kind
		expected bool
	}{
		{"Star equals Star", KindStar, &Star{}, true},
		{"Star not equals Arrow", KindStar, KindUnary, false},
		{"Unary equals Unary", KindUnary, &Arrow{From: KindStar, To: KindStar}, true},
		{"Binary equals Binary", KindBinary, KindBinary, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.k1.Equals(tt.k2)
			if result != tt.expected {
				t.Errorf("Equals(%v, %v) = %v, want %v", tt.k1, tt.k2, result, tt.expected)
			}
		})
	}
}

func TestKindUnification(t *testing.T) {
	tests := []struct {
		name      string
		k1        Kind
		k2        Kind
		shouldErr bool
	}{
		{"Unify Star with Star", KindStar, KindStar, false},
		{"Unify Unary with Unary", KindUnary, KindUnary, false},
		{"Unify Star with Unary fails", KindStar, KindUnary, true},
		{"Unify KindVar with Star", &KindVar{ID: 1}, KindStar, false},
		{"Unify KindVar with Unary", &KindVar{ID: 2}, KindUnary, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := UnifyKinds(tt.k1, tt.k2)
			if (err != nil) != tt.shouldErr {
				t.Errorf("UnifyKinds(%v, %v) error = %v, shouldErr = %v", tt.k1, tt.k2, err, tt.shouldErr)
			}
		})
	}
}

func TestKindSubstitution(t *testing.T) {
	kv1 := &KindVar{ID: 1}
	kv2 := &KindVar{ID: 2}

	subst := KindSubstitution{
		1: KindStar,
		2: KindUnary,
	}

	t.Run("Substitute KindVar with Star", func(t *testing.T) {
		result := subst.Apply(kv1)
		if !result.Equals(KindStar) {
			t.Errorf("Apply(%v) = %v, want *", kv1, result)
		}
	})

	t.Run("Substitute KindVar with Unary", func(t *testing.T) {
		result := subst.Apply(kv2)
		if !result.Equals(KindUnary) {
			t.Errorf("Apply(%v) = %v, want * -> *", kv2, result)
		}
	})

	t.Run("Substitute in Arrow", func(t *testing.T) {
		arrow := &Arrow{From: kv1, To: kv2}
		result := subst.Apply(arrow)
		expected := &Arrow{From: KindStar, To: KindUnary}
		if !result.Equals(expected) {
			t.Errorf("Apply(%v) = %v, want %v", arrow, result, expected)
		}
	})
}

func TestKindString(t *testing.T) {
	tests := []struct {
		name     string
		kind     Kind
		expected string
	}{
		{"Star", KindStar, "*"},
		{"Unary", KindUnary, "* -> *"},
		{"Binary", KindBinary, "* -> * -> *"},
		{"Nested Arrow", &Arrow{From: KindUnary, To: KindStar}, "(* -> *) -> *"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.kind.String()
			if result != tt.expected {
				t.Errorf("String() = %v, want %v", result, tt.expected)
			}
		})
	}
}
