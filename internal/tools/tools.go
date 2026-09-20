package tools

import "github.com/zp/genesis/internal/agent"

// RegisterBuiltins adds the builtin tools. Genesis keeps this set deliberately
// small: character beats (prose interleaved with inner thought, the Narrator's
// descriptions among them) and the turn-ending choice prompt. Persistent state
// tracking is parked for now.
func RegisterBuiltins(r *agent.Registry) {
	r.Register(writingBlockTool())
	r.Register(choicesTool())
}
