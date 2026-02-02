package app

import "strings"

// Mode represents the operating mode of the agent.
type Mode string

const (
	ModeAsk         Mode = "ask"         // Ask mode for answering questions
	ModeArchitect   Mode = "architect"   // Architect mode for high-level design
	ModePlan        Mode = "plan"        // Plan mode for analyzing projects
	ModeDo          Mode = "do"          // Do mode for executing tasks
	ModeCode        Mode = "code"        // Code mode for making changes
	ModeDebug       Mode = "debug"       // Debug mode for troubleshooting
	ModeOrchestrate Mode = "orchestrate" // Orchestrate mode for coordinating multiple agents
)

func ParseMode(value string) (Mode, bool) {
	v := strings.ToLower(strings.TrimSpace(value))
	switch v {
	case string(ModeAsk):
		return ModeAsk, true
	case string(ModeArchitect):
		return ModeArchitect, true
	case string(ModePlan):
		return ModePlan, true
	case string(ModeDo):
		return ModeDo, true
	case string(ModeCode):
		return ModeCode, true
	case string(ModeDebug):
		return ModeDebug, true
	case string(ModeOrchestrate):
		return ModeOrchestrate, true
	default:
		return Mode(""), false
	}
}

func IsToolMode(mode Mode) bool {
	// Modes that allow tool execution
	return mode == ModeDo || mode == ModeCode || mode == ModeDebug || mode == ModeOrchestrate
}
