package llm

import (
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

// M3-LLM-TIMEOUT-ADAPT-1: env override for the default per-attempt timeout
// used when a request sets neither Timeout nor NoTimeout. The historical
// hard-coded 60s default stays the fallback for every other path (env unset,
// parse failure, out of range), so existing callers (c1/c2 planner explicit
// timeouts, NoTimeout legs, TimeoutIsTotalBudget budgeting) keep their exact
// semantics. The override exists because reasoning-class engines physically
// need longer per-attempt windows (238 discrimination run: 12/28 attempts cut
// mid-inference by the 60s wall).
const llmTimeoutEnvVar = "VIT_AGENT_LLM_TIMEOUT_SEC"

const (
	defaultLLMAttemptTimeout = 60 * time.Second
	minLLMTimeoutSec         = 1
	maxLLMTimeoutSec         = 3600
)

// warnLLMTimeoutFallback is the single non-silent record point for a fallen
// back env override. It is a package-level variable so tests can inject a
// capture function (constructor injection, no process-global state).
var warnLLMTimeoutFallback = func(reason string) {
	fmt.Fprintf(os.Stderr, "llm timeout adapt: %s\n", reason)
}

// resolveDefaultAttemptTimeout maps the raw env value onto the fallback
// per-attempt timeout. Empty (after trim) keeps the historical 60s default.
// A parse failure or an out-of-range value (allowed 1-3600s) falls back to
// 60s and returns a non-empty warn reason so the caller records the
// misconfiguration instead of silently ignoring it.
func resolveDefaultAttemptTimeout(envValue string) (time.Duration, string) {
	trimmed := strings.TrimSpace(envValue)
	if trimmed == "" {
		return defaultLLMAttemptTimeout, ""
	}
	seconds, err := strconv.Atoi(trimmed)
	if err != nil {
		return defaultLLMAttemptTimeout, fmt.Sprintf("invalid %s=%q (not an integer); falling back to default %s", llmTimeoutEnvVar, trimmed, defaultLLMAttemptTimeout)
	}
	if seconds < minLLMTimeoutSec || seconds > maxLLMTimeoutSec {
		return defaultLLMAttemptTimeout, fmt.Sprintf("out-of-range %s=%d (allowed %d-%d); falling back to default %s", llmTimeoutEnvVar, seconds, minLLMTimeoutSec, maxLLMTimeoutSec, defaultLLMAttemptTimeout)
	}
	return time.Duration(seconds) * time.Second, ""
}

// attemptHTTPClient builds the per-request http.Client used when the caller
// did not inject one. Red lines (unchanged from the pre-adaptation shape):
// an explicit req.Timeout wins over the env override, NoTimeout keeps an
// uncapped client, and the env path only ever touches the no-Timeout default
// slot. A fallen-back env value is recorded once per adaptation decision,
// never silently.
func attemptHTTPClient(llmReq Request, envValue string) *http.Client {
	timeout := llmReq.Timeout
	if timeout <= 0 && !llmReq.NoTimeout {
		resolved, warnReason := resolveDefaultAttemptTimeout(envValue)
		timeout = resolved
		if warnReason != "" {
			warnLLMTimeoutFallback(warnReason)
		}
	}
	return &http.Client{Timeout: timeout}
}
