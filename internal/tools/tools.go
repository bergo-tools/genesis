package tools

import "github.com/zp/genesis/internal/agent"

// RegisterBuiltins adds every builtin tool to the registry.
func RegisterBuiltins(r *agent.Registry) {
	r.Register(sendMessageTool())
	r.Register(thinkTool())
	r.Register(setSceneTool())
	r.Register(updateCharacterTool())
	r.Register(updateStateTool())
	r.Register(getStateTool())
	r.Register(rememberTool())
	r.Register(recallTool())
	r.Register(rollTool())
	r.Register(offerChoicesTool())
	r.Register(awaitPlayerTool())
	r.Register(endTurnTool())
}
