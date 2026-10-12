package harness

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-zeromq/zmq4"
)

// M2-TIMEFX-2 timefx dual-tap probe collection: send the kernel probe
// command, collect the terminal probe event over the feature pub socket, and
// locate the kernel-written evidence artifact. Mirrors the COM paired
// collection path (com_observation.go) for the reverb/delay families.

const (
	timefxProbeFeatureType     = "timefx_dual_tap_probe"
	timefxProbeReadyCommand    = "timefx_dual_tap_probe_ready"
	timefxProbeFailureCommand  = "timefx_dual_tap_probe_failed"
	timefxEvidenceArtifactRoot = "timefx_evidence"
)

// CollectTimefxDualTapProbe sends one timefx_dual_tap_probe kernel command
// and blocks until the matching terminal event (ready/failed) arrives or the
// context times out. It returns the terminal event plus the synchronous
// kernel reply. A nil receipt without error means the wait window elapsed
// before any terminal event matched.
func (h *Harness) CollectTimefxDualTapProbe(ctx context.Context, kernelCmd map[string]any, requestID, trackID, pluginID string) (map[string]any, map[string]any, error) {
	if h == nil || h.kernel == nil {
		return nil, nil, fmt.Errorf("kernel_client_unavailable")
	}
	timeout := mixboardL2RenderProbeCollectWait()
	if timeout < 60*time.Second {
		timeout = 60 * time.Second
	}
	var reply map[string]any
	send := func() error {
		sendCtx, cancel := context.WithTimeout(context.Background(), mixboardFeatureBackgroundSendWait())
		defer cancel()
		var sendErr error
		reply, _, sendErr = h.kernel.SendCommand(sendCtx, kernelCmd)
		if sendErr != nil {
			return sendErr
		}
		if !kernelReplySucceeded(reply) {
			return fmt.Errorf("%s", firstNonEmpty(firstString(reply, "message", "error"), "timefx_dual_tap_probe request failed"))
		}
		return nil
	}
	receipt, err := collectFeatureProbeEvent(ctx, mixboardFeatureSubURL, timefxProbeFeatureType,
		[]string{timefxProbeReadyCommand, timefxProbeFailureCommand}, requestID, trackID, pluginID, timeout, send)
	return receipt, reply, err
}

// TimefxEvidenceArtifactPath resolves the kernel artifact location for one
// probe pair under the dev-root workspace (mirrors comEvidenceArtifactPath).
func TimefxEvidenceArtifactPath(pairID string) string {
	for _, key := range []string{"VIT_DAW_DEV_ROOT", "VIT_DEV_ROOT", "VIT_ROOT"} {
		if root := strings.TrimSpace(os.Getenv(key)); root != "" {
			return filepath.Join(root, "VitApp", "Workspace", "Artifacts", timefxEvidenceArtifactRoot, pairID, pairID+".json")
		}
	}
	return ""
}

// collectFeatureProbeEvent is the generalized COM-probe event collector: it
// subscribes the feature pub socket, invokes afterSubscribe (the command
// send), then waits for the first event matching featureType plus one of the
// terminal commands, filtered by the identity triple when non-empty.
func collectFeatureProbeEvent(ctx context.Context, subURL, featureType string, terminalCommands []string,
	requestID, trackID, pluginID string, timeout time.Duration, afterSubscribe func() error) (map[string]any, error) {
	opCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	sub := zmq4.NewSub(opCtx, zmq4.WithTimeout(120*time.Millisecond), zmq4.WithAutomaticReconnect(true))
	defer sub.Close()
	if err := sub.SetOption(zmq4.OptionSubscribe, ""); err != nil {
		return nil, err
	}
	if err := sub.Dial(firstNonEmpty(subURL, mixboardFeatureSubURL)); err != nil {
		return nil, err
	}
	warmMixboardFeatureSubscriber(opCtx)
	if afterSubscribe != nil {
		if err := afterSubscribe(); err != nil {
			return nil, err
		}
	}
	terminal := map[string]bool{}
	for _, command := range terminalCommands {
		terminal[command] = true
	}
	for {
		select {
		case <-opCtx.Done():
			return nil, nil
		default:
		}
		msg, err := sub.Recv()
		if err != nil {
			if errors.Is(err, context.DeadlineExceeded) || strings.Contains(strings.ToLower(err.Error()), "timeout") {
				continue
			}
			if errors.Is(err, context.Canceled) {
				return nil, nil
			}
			return nil, err
		}
		event := map[string]any{}
		if json.Unmarshal([]byte(zmqMsgPayload(msg)), &event) != nil || firstString(event, "feature_type") != featureType {
			continue
		}
		if requestID != "" && firstString(event, "request_id") != requestID {
			continue
		}
		if trackID != "" && firstString(event, "track_id") != trackID {
			continue
		}
		if pluginID != "" && firstString(event, "plugin_instance_id") != pluginID {
			continue
		}
		if terminal[firstString(event, "command")] {
			return event, nil
		}
	}
}
