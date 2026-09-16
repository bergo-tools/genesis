package tools

import "github.com/zp/genesis/internal/agent"

// RegisterBuiltins adds the builtin tools. Genesis keeps this set deliberately
// small: narrative beats (with inner thought), scene description, and the
// turn-ending choice prompt. Persistent state tracking is parked for now.
func RegisterBuiltins(r *agent.Registry) {
	r.Register(messageTool())
	r.Register(sceneTool())
	r.Register(choicesTool())
}
