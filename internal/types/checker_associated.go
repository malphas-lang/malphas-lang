package types

import (
	"fmt"

	"github.com/malphas-lang/malphas-lang/internal/ast"
	"github.com/malphas-lang/malphas-lang/internal/diag"
)

// checkTypeAssignments verifies that type assignments in an impl block
// match the associated types declared in the trait.
func (c *Checker) checkTypeAssignments(impl *ast.ImplDecl, trait *Trait) {
	// Create a map of type assignments for quick lookup
	assignments := make(map[string]*ast.TypeAssignment)
	for _, ta := range impl.TypeAssignments {
		assignments[ta.Name.Name] = ta
	}

	// Check that all trait associated types are specified
	for _, assocType := range trait.AssociatedTypes {
		assignment, found := assignments[assocType.Name]
		if !found {
			c.reportErrorWithCode(
				fmt.Sprintf("impl for trait %s is missing associated type %s", trait.Name, assocType.Name),
				impl.Span(),
				diag.CodeTypeMissingAssociatedType,
				fmt.Sprintf("add the missing associated type assignment:\n  type %s = <YourType>;", assocType.Name),
				nil,
			)
			continue
		}

		// Resolve the assigned type
		assignedType := c.resolveType(assignment.Type)

		// Check that the assigned type satisfies the bounds
		for _, bound := range assocType.Bounds {
			if err := Satisfies(assignedType, []Type{bound}, c.Env); err != nil {
				c.reportErrorWithCode(
					fmt.Sprintf("type %s does not satisfy trait bound %s for associated type %s",
						assignedType, bound, assocType.Name),
					assignment.Type.Span(),
					diag.CodeTypeConstraintViolation,
					fmt.Sprintf("the type %s must implement trait %s", assignedType, bound),
					nil,
				)
			}
		}

		// Remove from assignments map to track which ones we've checked
		delete(assignments, assocType.Name)
	}

	// Check for extra type assignments that aren't in the trait
	for name, ta := range assignments {
		c.reportErrorWithCode(
			fmt.Sprintf("trait %s has no associated type named %s", trait.Name, name),
			ta.Name.Span(),
			diag.CodeTypeUnknownAssociatedType,
			"remove this type assignment or check the trait definition",
			nil,
		)
	}
}
