package model

type Agent string

const (
	Codex      Agent = "codex"
	ClaudeCode Agent = "claude_code"
)

var Agents = []Agent{Codex, ClaudeCode}

// Events written before multi-assistant support came from Codex.
func (a Agent) Normalized() Agent {
	if a == "" {
		return Codex
	}
	return a
}

func (a Agent) Name() string {
	if a == ClaudeCode {
		return "Claude Code"
	}
	return "Codex"
}

func (e Event) AgentName() string {
	if e.Kind == Test {
		return "Codex / Claude Code"
	}
	return e.Agent.Name()
}

func (p Preferences) AgentEnabled(agent Agent) bool {
	if agent.Normalized() == ClaudeCode {
		return p.ClaudeAutoConnect
	}
	return p.AutoConnect
}
