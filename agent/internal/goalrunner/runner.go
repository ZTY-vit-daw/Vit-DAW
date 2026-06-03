package goalrunner

import "vit-daw-agent/internal/agentloop"

const (
	StopReasonDone               = agentloop.StopReasonDone
	StopReasonNeedsClarification = agentloop.StopReasonNeedsClarification
	StopReasonNeedsConfirmation  = agentloop.StopReasonNeedsConfirmation
	StopReasonLimitReached       = agentloop.StopReasonLimitReached
	StopReasonInterjection       = agentloop.StopReasonInterjection
	StopReasonCancelled          = agentloop.StopReasonCancelled
	StopReasonFailed             = agentloop.StopReasonFailed

	LimitTypeTurns     = agentloop.LimitTypeTurns
	LimitTypeToolCalls = agentloop.LimitTypeToolCalls
	LimitTypeTimeout   = agentloop.LimitTypeTimeout
)

type Budget = agentloop.Budget
type Input = agentloop.Input
type Continuation = agentloop.Continuation
type Result = agentloop.Result
type Planner = agentloop.Planner
type ToolExecutor = agentloop.ToolExecutor
type Runner = agentloop.Runner
