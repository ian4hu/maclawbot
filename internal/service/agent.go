package service

import (
	"maclawbot/internal/proxy"
	"maclawbot/internal/router"
)

// HandleAgentChange ensures proxy servers match the configured agents.
// Called after /clawbot new or /clawbot del commands.
func HandleAgentChange(state *router.State, pm *proxy.ProxyManager) {
	agents := state.GetAgents()

	// Start servers for agents that don't have one running
	for name, agent := range agents {
		if pm.GetQueue(name) == nil && agent.Enabled {
			pm.OnAgentAdded(agent)
		}
	}

	// Stop servers for agents that were removed from state
	for _, name := range pm.GetActiveAgents() {
		if _, exists := agents[name]; !exists {
			pm.OnAgentRemoved(name)
		}
	}
}
