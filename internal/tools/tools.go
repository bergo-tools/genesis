package tools

import "github.com/zp/genesis/internal/agent"

// RegisterBuiltins adds the builtin tools. Genesis keeps this set deliberately
// small: character beats (with inner thought), the narrator's descriptions, and
// the turn-ending choice prompt. Persistent state tracking is parked for now.
func RegisterBuiltins(r *agent.Registry) {
	r.Register(messageTool())
	r.Register(narratorTool())
	r.Register(choicesTool())
}
