package analysis

import (
	"fmt"
	"sort"
	"strings"

	"github.com/malphas-lang/malphas-lang/internal/haruspex/liveir"
)

// VarID uniquely identifies a variable in the analysis.
type VarID string

// SymState represents the symbolic state at a given point in the program.
type SymState struct {
	Vars           map[VarID]liveir.LiveValue
	Temps          map[int]*liveir.SymExpr // Map ValueID to symbolic expression
	PathConditions []liveir.Constraint
	Unsatisfiable  bool // True if this state is unreachable (e.g. dead code)
}

// NewSymState creates a new empty symbolic state.
func NewSymState() *SymState {
	return &SymState{
		Vars:           make(map[VarID]liveir.LiveValue),
		Temps:          make(map[int]*liveir.SymExpr),
		PathConditions: make([]liveir.Constraint, 0),
		Unsatisfiable:  false,
	}
}

// Clone creates a deep copy of the symbolic state.
func (s *SymState) Clone() *SymState {
	newVars := make(map[VarID]liveir.LiveValue, len(s.Vars))
	for k, v := range s.Vars {
		newVars[k] = v
	}

	newTemps := make(map[int]*liveir.SymExpr, len(s.Temps))
	for k, v := range s.Temps {
		newTemps[k] = v
	}

	newConditions := make([]liveir.Constraint, len(s.PathConditions))
	copy(newConditions, s.PathConditions)

	return &SymState{
		Vars:           newVars,
		Temps:          newTemps,
		PathConditions: newConditions,
		Unsatisfiable:  s.Unsatisfiable,
	}
}

// GetVar retrieves the value of a variable.
func (s *SymState) GetVar(id VarID) (liveir.LiveValue, bool) {
	val, ok := s.Vars[id]
	return val, ok
}

// SetVar sets the value of a variable.
func (s *SymState) SetVar(id VarID, val liveir.LiveValue) {
	s.Vars[id] = val
}

// AddConstraint adds a path condition to the state.
func (s *SymState) AddConstraint(c liveir.Constraint) {
	s.PathConditions = append(s.PathConditions, c)
}

// Merge combines another state into this one (control flow join).
func (s *SymState) Merge(other *SymState) {
	// If other is unsatisfiable, ignore it (it's dead code)
	if other.Unsatisfiable {
		return
	}
	// If self is unsatisfiable, take other entirely
	if s.Unsatisfiable {
		*s = *other.Clone()
		return
	}

	// Merge variables
	for id, val := range other.Vars {
		if existing, ok := s.Vars[id]; ok {
			if !existing.Equals(val) {
				// Conflict: set to Unknown
				s.Vars[id] = liveir.LiveValue{Kind: liveir.ValueKindUnknown}
			}
		} else {
			// Variable only in other branch: set to Unknown (or handle as optional?)
			s.Vars[id] = liveir.LiveValue{Kind: liveir.ValueKindUnknown}
		}
	}
	for id := range s.Vars {
		if _, ok := other.Vars[id]; !ok {
			// Variable only in this branch: set to Unknown
			s.Vars[id] = liveir.LiveValue{Kind: liveir.ValueKindUnknown}
		}
	}

	// Merge constraints (intersection)
	// Keep only constraints present in both states
	var commonConditions []liveir.Constraint
	for _, c1 := range s.PathConditions {
		found := false
		for _, c2 := range other.PathConditions {
			if c1.Equals(c2) {
				found = true
				break
			}
		}
		if found {
			commonConditions = append(commonConditions, c1)
		}
	}
	s.PathConditions = commonConditions
}

func (s *SymState) String() string {
	var sb strings.Builder

	if s.Unsatisfiable {
		sb.WriteString("  (UNREACHABLE)\n")
	}

	if len(s.Vars) > 0 {
		sb.WriteString("  Vars:\n")
		var vars []string
		for id, val := range s.Vars {
			vars = append(vars, fmt.Sprintf("    %s: %s", id, val))
		}
		sort.Strings(vars)
		sb.WriteString(strings.Join(vars, "\n"))
		sb.WriteString("\n")
	}

	if len(s.PathConditions) > 0 {
		sb.WriteString("  Conds:\n")
		var conds []string
		for _, c := range s.PathConditions {
			conds = append(conds, fmt.Sprintf("    %s", c.String()))
		}
		sb.WriteString(strings.Join(conds, "\n"))
		sb.WriteString("\n")
	}

	if sb.Len() == 0 {
		return "  {}"
	}
	return strings.TrimRight(sb.String(), "\n")
}
