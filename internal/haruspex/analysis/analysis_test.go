package analysis

import (
	"testing"

	"github.com/malphas-lang/malphas-lang/internal/haruspex/diagnostics"
	"github.com/malphas-lang/malphas-lang/internal/haruspex/liveir"
)

func TestAnalyzeEmptyFunction(t *testing.T) {
	engine := NewEngine()

	fn := &liveir.LiveFunction{
		Name: "test",
		Entry: &liveir.LiveBlock{
			ID:    0,
			Nodes: []liveir.LiveNode{},
			Next:  []*liveir.LiveBlock{},
		},
	}
	fn.Blocks = []*liveir.LiveBlock{fn.Entry}

	reporter := diagnostics.NewReporter()
	_, err := engine.Analyze(fn, reporter)
	if err != nil {
		t.Errorf("Analyze failed: %v", err)
	}
}

func TestAnalyzeBasicFlow(t *testing.T) {
	engine := NewEngine()

	// Entry -> Block1 -> Exit
	exit := &liveir.LiveBlock{ID: 2}
	block1 := &liveir.LiveBlock{ID: 1, Next: []*liveir.LiveBlock{exit}}
	entry := &liveir.LiveBlock{ID: 0, Next: []*liveir.LiveBlock{block1}}

	fn := &liveir.LiveFunction{
		Name:   "flow",
		Entry:  entry,
		Blocks: []*liveir.LiveBlock{entry, block1, exit},
	}

	reporter := diagnostics.NewReporter()
	_, err := engine.Analyze(fn, reporter)
	if err != nil {
		t.Errorf("Analyze failed: %v", err)
	}
}
