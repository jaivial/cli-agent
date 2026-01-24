package app

import "strings"

type Mode string

const (
	ModeAsk        Mode = "ask"
	ModeArchitect  Mode = "architect"
	ModePlan       Mode = "plan"
	ModeDo         Mode = "do"
	ModeCode       Mode = "code"
	ModeDebug      Mode = "debug"
	ModeOrchestrate Mode = "orchestrate"
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
