package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"vit-daw-agent/internal/bridge"
	"vit-daw-agent/internal/chat"
	"vit-daw-agent/internal/kernel"
	"vit-daw-agent/internal/logx"
	"vit-daw-agent/internal/shadow"
	"vit-daw-agent/internal/vspclient"
)

const (
	vpsForgeStagingVPSPathEnv        = "VIT_VPS_FORGE_STAGING_VPS_PATH"
	vpsForgeStagingActivationPathEnv = "VIT_VPS_FORGE_STAGING_ACTIVATION_PATH"
	vpsForgeStagingActivationSchema  = "vit.vpsforge.staging_activation.v1"
)

type vpsForgeStagingActivation struct {
	SchemaVersion  string `json:"schema_version"`
	Enabled        bool   `json:"enabled"`
	StagingVPSPath string `json:"staging_vps_path"`
}

func main() {
	stagingActivationSource, stagingActivationErr := configureVPSForgeStagingActivation()
	configureCapabilityRuntimeV1Defaults()
	var (
		zmqReqURL    = flag.String("zmq-req-url", envString("VIT_AGENT_ZMQ_REQ_URL", envString("VIT_BRIDGE_ZMQ_REQ_URL", "tcp://127.0.0.1:5555")), "kernel ZMQ REQ endpoint")
		zmqSubURL    = flag.String("zmq-sub-url", envString("VIT_AGENT_ZMQ_SUB_URL", envString("VIT_BRIDGE_ZMQ_SUB_URL", "tcp://127.0.0.1:5556")), "kernel ZMQ SUB endpoint")
		godotIP      = flag.String("godot-ip", envString("VIT_AGENT_GODOT_IP", envString("VIT_BRIDGE_GODOT_IP", "127.0.0.1")), "Godot UDP host")
		udpToGodot   = flag.Int("udp-to-godot", envInt("VIT_AGENT_UDP_TO_GODOT", envInt("VIT_BRIDGE_UDP_TO_GODOT", 4444)), "telemetry UDP destination port")
		udpFromGodot = flag.Int("udp-from-godot", envInt("VIT_AGENT_UDP_FROM_GODOT", envInt("VIT_BRIDGE_UDP_FROM_GODOT", 4445)), "command UDP listen port")
		httpAddr     = flag.String("http", envString("VIT_AGENT_HTTP_ADDR", "127.0.0.1:7878"), "agent HTTP listen address")
		timeoutMS    = flag.Int("req-timeout-ms", envInt("VIT_AGENT_REQ_TIMEOUT_MS", envInt("VIT_BRIDGE_REQ_TIMEOUT_MS", 120000)), "kernel request timeout in milliseconds")
		retries      = flag.Int("req-max-retries", envInt("VIT_AGENT_REQ_MAX_RETRIES", envInt("VIT_BRIDGE_REQ_MAX_RETRIES", 1)), "kernel request retry count")
		verbose      = flag.Bool("verbose", envBool("VIT_AGENT_VERBOSE", envBool("VIT_BRIDGE_VERBOSE", false)), "verbose logging")
		lastLogPath  = flag.String("last-log-path", envString("VIT_AGENT_LAST_LOG_PATH", envString("VIT_BRIDGE_LAST_LOG_PATH", defaultLogPath())), "rolling last-log path")
		keepLogLines = flag.Int("keep-last-log-lines", envInt("VIT_AGENT_KEEP_LAST_LOG_LINES", envInt("VIT_BRIDGE_KEEP_LAST_LOG_LINES", 500)), "rolling log line count")
		fileReplyDir = flag.String("file-reply-dir", envString("VIT_AGENT_FILE_REPLY_DIR", envString("VIT_BRIDGE_FILE_REPLY_DIR", "")), "directory for oversized command reply JSON files")
		vspHubURL    = flag.String("vsp-hub-url", envString("VIT_AGENT_VSP_HUB_URL", "http://127.0.0.1:8787/vsp"), "VSP Hub endpoint for registering VitAgent as role=agent; empty disables registration")
		vspHubReq    = flag.Bool("vsp-hub-required", envBool("VIT_AGENT_VSP_HUB_REQUIRED", false), "stop VitAgent if VSP Hub registration fails")
	)
	flag.Parse()
	if strings.TrimSpace(*fileReplyDir) == "" && strings.TrimSpace(*lastLogPath) != "" {
		*fileReplyDir = filepath.Join(filepath.Dir(*lastLogPath), "BridgeReplies")
	}

	logger := logx.New(*verbose, *lastLogPath, *keepLogLines)
	if stagingActivationErr != nil {
		logger.Warn("[vpsforge.staging] activation ignored: %v", stagingActivationErr)
	} else if stagingActivationSource != "" {
		logger.Info("[vpsforge.staging] activated isolated staging VPS from %s", stagingActivationSource)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	reqTimeout := time.Duration(*timeoutMS) * time.Millisecond
	kernelClient := kernel.New(*zmqReqURL, reqTimeout)
	shadowProject := shadow.New(logger)
	chatServer := chat.New(kernelClient, shadowProject, logger)
	bridgeService := bridge.New(bridge.Config{
		ZMQSubURL:     *zmqSubURL,
		GodotIP:       *godotIP,
		UDPToGodot:    *udpToGodot,
		UDPFromGodot:  *udpFromGodot,
		ReqMaxRetries: *retries,
		ReqTimeout:    reqTimeout,
		FileReplyDir:  *fileReplyDir,
		TelemetryHook: chatServer.HandleKernelTelemetry,
		VSPHubURL:     *vspHubURL,
	}, kernelClient, shadowProject, logger)
	httpServer := &http.Server{
		Addr:              *httpAddr,
		Handler:           chatServer.Routes(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		logger.Info("HTTP agent API listening on http://%s", *httpAddr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("HTTP server failed: %v", err)
			stop()
		}
	}()

	go func() {
		if err := bridgeService.Run(ctx); err != nil {
			logger.Error("bridge stopped: %v", err)
			stop()
		}
	}()

	startVSPHubRegistration(ctx, logger, *vspHubURL, *vspHubReq, reqTimeout, stop)

	logger.Info("VitAgent started req=%s sub=%s udp_to=%d udp_from=%d", *zmqReqURL, *zmqSubURL, *udpToGodot, *udpFromGodot)
	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_ = httpServer.Shutdown(shutdownCtx)
	logger.Info("VitAgent shutdown complete")
}

// configureVPSForgeStagingActivation is intentionally opt-in.  A normal
// Agent keeps its canonical VPS Library and Catalog; this function merely
// points the process at an explicitly activated staging artifact, which the
// chat bridge opens through its own disposable library.
func configureVPSForgeStagingActivation() (string, error) {
	if strings.TrimSpace(os.Getenv(vpsForgeStagingVPSPathEnv)) != "" {
		return "environment", nil
	}
	for _, candidate := range vpsForgeStagingActivationCandidates() {
		data, err := os.ReadFile(candidate)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return "", fmt.Errorf("read staging activation %s: %w", candidate, err)
		}
		var activation vpsForgeStagingActivation
		if err := json.Unmarshal(data, &activation); err != nil {
			return "", fmt.Errorf("decode staging activation %s: %w", candidate, err)
		}
		if activation.SchemaVersion != vpsForgeStagingActivationSchema {
			return "", fmt.Errorf("staging activation %s has unsupported schema %q", candidate, activation.SchemaVersion)
		}
		if !activation.Enabled {
			return candidate, nil
		}
		stagingPath := strings.TrimSpace(activation.StagingVPSPath)
		if stagingPath == "" {
			return "", fmt.Errorf("staging activation %s has no staging_vps_path", candidate)
		}
		if !filepath.IsAbs(stagingPath) {
			stagingPath = filepath.Join(filepath.Dir(candidate), stagingPath)
		}
		stagingPath, err = filepath.Abs(stagingPath)
		if err != nil {
			return "", fmt.Errorf("resolve staging artifact from %s: %w", candidate, err)
		}
		if info, statErr := os.Stat(stagingPath); statErr != nil || info.IsDir() {
			if statErr != nil {
				return "", fmt.Errorf("staging artifact from %s is unavailable: %w", candidate, statErr)
			}
			return "", fmt.Errorf("staging artifact from %s is not a file", candidate)
		}
		if err := os.Setenv(vpsForgeStagingVPSPathEnv, stagingPath); err != nil {
			return "", fmt.Errorf("activate staging artifact from %s: %w", candidate, err)
		}
		return candidate, nil
	}
	return "", nil
}

func vpsForgeStagingActivationCandidates() []string {
	values := []string{}
	appendPath := func(path string) {
		path = strings.TrimSpace(path)
		if path == "" {
			return
		}
		for _, existing := range values {
			if strings.EqualFold(existing, path) {
				return
			}
		}
		values = append(values, path)
	}
	appendPath(os.Getenv(vpsForgeStagingActivationPathEnv))
	for _, root := range []string{os.Getenv("VIT_DAW_DEV_ROOT"), os.Getenv("VIT_DAW_ROOT")} {
		if strings.TrimSpace(root) != "" {
			appendPath(filepath.Join(root, "VPSForge", "staging", "active_vpsforge_staging_runtime.json"))
		}
	}
	if executable, err := os.Executable(); err == nil {
		binRoot := filepath.Dir(executable)
		if strings.EqualFold(filepath.Base(binRoot), "bin") && strings.EqualFold(filepath.Base(filepath.Dir(binRoot)), "agent") {
			appendPath(filepath.Join(filepath.Dir(filepath.Dir(binRoot)), "VPSForge", "staging", "active_vpsforge_staging_runtime.json"))
		}
	}
	if configRoot, err := os.UserConfigDir(); err == nil && strings.TrimSpace(configRoot) != "" {
		appendPath(filepath.Join(configRoot, "Vit", "Agent", "vpsforge_staging_runtime.json"))
	}
	return values
}

// This release passed isolated Release-kernel CAS/idempotency and full B2/B3
// History -> execution -> structural/acoustic verification smokes. B2/B3 are
// permanently cut over at the Chat abstraction seam; these defaults remain
// for deployment telemetry compatibility, but explicit "off" values cannot
// resurrect the physically retired legacy authority. Binary rollback is the
// only rollback after this Strangler cutover stage.
func configureCapabilityRuntimeV1Defaults() {
	setEnvDefault("VIT_CAPABILITY_RUNTIME_V1_ROLLOUT", "all")
	setEnvDefault("VIT_CAPABILITY_RUNTIME_V1_LIVE_VERIFIED", "true")
	setEnvDefault("VIT_CAPABILITY_RUNTIME_V1_DISABLE_LEGACY_CREATION", "true")
}

func setEnvDefault(name, value string) {
	if _, exists := os.LookupEnv(name); !exists {
		_ = os.Setenv(name, value)
	}
}

func startVSPHubRegistration(ctx context.Context, logger *logx.Logger, hubURL string, required bool, timeout time.Duration, stop context.CancelFunc) {
	hubURL = strings.TrimSpace(hubURL)
	if hubURL == "" {
		if logger != nil {
			logger.Info("VSP Hub registration disabled")
		}
		return
	}
	go func() {
		attempt := 0
		for {
			attempt++
			reqCtx, cancel := context.WithTimeout(ctx, minDuration(timeout, 8*time.Second))
			client := vspclient.New(hubURL, "vit.agent.official", "agent", "VitAgent", "vsp-hub-client-v1", minDuration(timeout, 8*time.Second))
			reply, err := client.Hello(reqCtx,
				[]string{
					"command.request",
					"state.snapshot",
					"state.delta",
					"state.resync",
					"event.subscribe",
					"event.poll",
					"asset.request",
					"realtime.publish",
				},
				[]string{"vsp.hub.http"},
			)
			cancel()
			if err == nil {
				if logger != nil {
					logger.Info("VitAgent registered with VSP Hub url=%s session_id=%s type=%s", hubURL, client.SessionID, fmt.Sprint(reply["type"]))
				}
				return
			}
			if required {
				if logger != nil {
					logger.Error("required VSP Hub registration failed url=%s error=%v", hubURL, err)
				}
				stop()
				return
			}
			if logger != nil && (attempt == 1 || attempt%10 == 0) {
				logger.Warn("VSP Hub registration pending url=%s attempt=%d error=%v", hubURL, attempt, err)
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(2 * time.Second):
			}
		}
	}()
}

func minDuration(a, b time.Duration) time.Duration {
	if a <= 0 {
		return b
	}
	if a < b {
		return a
	}
	return b
}

func envString(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		switch strings.ToLower(v) {
		case "1", "true", "on", "yes":
			return true
		case "0", "false", "off", "no":
			return false
		}
	}
	return fallback
}

func defaultLogPath() string {
	wd, err := os.Getwd()
	if err != nil {
		return ""
	}
	if filepath.Base(wd) == "agent" {
		return filepath.Join(filepath.Dir(wd), "VitApp", "Workspace", "Logs", "agent_last.log")
	}
	return filepath.Join(wd, "VitApp", "Workspace", "Logs", "agent_last.log")
}

func init() {
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "VitAgent bridges Godot UDP, VitApp ZMQ, and Ask Vit chat.\n\n")
		flag.PrintDefaults()
	}
}
