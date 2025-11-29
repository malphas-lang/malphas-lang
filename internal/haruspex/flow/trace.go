package flow

import (
	"github.com/malphas-lang/malphas-lang/internal/haruspex/liveir"
)

// TerminationKind describes how a path terminated.
type TerminationKind int

const (
	TerminationUnknown TerminationKind = iota
	TerminationReturn
	TerminationPanic
	TerminationUnreachable
)

// FlowStep represents a single step in a flow trace.
type FlowStep struct {
	Node liveir.LiveNode
	// TODO: Add state changes
}

// FlowTrace represents a single execution path through the code.
type FlowTrace struct {
	PathID        int
	Steps         []FlowStep
	PathCondition []liveir.Constraint
	Termination   TerminationKind
}
