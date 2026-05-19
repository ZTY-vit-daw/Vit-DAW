package main

import (
	"context"
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
)

func main() {
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
	)
	flag.Parse()

	logger := logx.New(*verbose, *lastLogPath, *keepLogLines)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	reqTimeout := time.Duration(*timeoutMS) * time.Millisecond
	kernelClient := kernel.New(*zmqReqURL, reqTimeout)
	shadowProject := shadow.New(logger)
	bridgeService := bridge.New(bridge.Config{
		ZMQSubURL:     *zmqSubURL,
		GodotIP:       *godotIP,
		UDPToGodot:    *udpToGodot,
		UDPFromGodot:  *udpFromGodot,
		ReqMaxRetries: *retries,
		ReqTimeout:    reqTimeout,
	}, kernelClient, shadowProject, logger)
	chatServer := chat.New(kernelClient, shadowProject, logger)
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

	logger.Info("VitAgent started req=%s sub=%s udp_to=%d udp_from=%d", *zmqReqURL, *zmqSubURL, *udpToGodot, *udpFromGodot)
	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_ = httpServer.Shutdown(shutdownCtx)
	logger.Info("VitAgent shutdown complete")
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
