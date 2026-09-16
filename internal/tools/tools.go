package tools

import "github.com/zp/genesis/internal/agent"

// RegisterBuiltins adds the builtin tools. Genesis keeps this set deliberately
// small: narration, inner monologue, persistent state, and the turn-ending
// choice prompt.
func RegisterBuiltins(r *agent.Registry) {
	r.Register(messageTool())
	r.Register(thinkTool())
	r.Register(updateStateTool())
	r.Register(choicesTool())
}
