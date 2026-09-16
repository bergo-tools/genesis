package tools

import "github.com/zp/genesis/internal/agent"

// RegisterBuiltins adds the builtin tools. Genesis keeps this set deliberately
// small: narrative beats (with inner thought), persistent tracking, scene
// description, and the turn-ending choice prompt.
func RegisterBuiltins(r *agent.Registry) {
	r.Register(messageTool())
	r.Register(updateStateTool())
	r.Register(sceneTool())
	r.Register(choicesTool())
}
