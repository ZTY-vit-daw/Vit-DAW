package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"vit-daw-agent/internal/kernel"
	"vit-daw-agent/internal/logx"
	"vit-daw-agent/internal/vsphub"
)

func main() {
	var (
		listenAddr      = flag.String("listen", envString("VIT_VSP_HUB_ADDR", vsphub.DefaultHTTPAddr), "VSP Hub HTTP listen address")
		kernelReqURL    = flag.String("kernel-req", envString("VIT_VSP_HUB_KERNEL_REQ_URL", envString("VIT_AGENT_ZMQ_REQ_URL", vsphub.DefaultKernelReqURL)), "kernel ZMQ REQ endpoint")
		kernelSubURL    = flag.String("kernel-sub", envString("VIT_VSP_HUB_KERNEL_SUB_URL", envString("VIT_AGENT_ZMQ_SUB_URL", vsphub.DefaultKernelSubURL)), "kernel ZMQ SUB endpoint")
		timeoutMS       = flag.Int("req-timeout-ms", envInt("VIT_VSP_HUB_REQ_TIMEOUT_MS", 120000), "kernel request timeout in milliseconds")
		enableTelemetry = flag.Bool("enable-telemetry", envBool("VIT_VSP_HUB_ENABLE_TELEMETRY", true), "relay kernel telemetry to VSP websocket streams")
		verbose         = flag.Bool("verbose", envBool("VIT_VSP_HUB_VERBOSE", false), "verbose logging")
		lastLogPath     = flag.String("last-log-path", envString("VIT_VSP_HUB_LAST_LOG_PATH", defaultLogPath()), "rolling last-log path")
		keepLogLines    = flag.Int("keep-last-log-lines", envInt("VIT_VSP_HUB_KEEP_LAST_LOG_LINES", 800), "rolling log line count")
	)
	flag.Parse()

	logger := logx.New(*verbose, *lastLogPath, *keepLogLines)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	reqTimeout := time.Duration(*timeoutMS) * time.Millisecond
	kernelClient := kernel.New(*kernelReqURL, reqTimeout)
	hub := vsphub.New(vsphub.Config{
		HTTPAddr:        *listenAddr,
		KernelReqURL:    *kernelReqURL,
		KernelSubURL:    *kernelSubURL,
		RequestTimeout:  reqTimeout,
		EnableTelemetry: *enableTelemetry,
	}, kernelClient, logger)

	logger.Info("VspHub started listen=%s kernel_req=%s kernel_sub=%s telemetry=%v", *listenAddr, *kernelReqURL, *kernelSubURL, *enableTelemetry)
	if err := hub.Run(ctx); err != nil {
		logger.Error("VspHub stopped: %v", err)
		os.Exit(1)
	}
	logger.Info("VspHub shutdown complete")
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
		return filepath.Join(filepath.Dir(wd), "VitApp", "Workspace", "Logs", "vsp_hub_last.log")
	}
	return filepath.Join(wd, "VitApp", "Workspace", "Logs", "vsp_hub_last.log")
}

func init() {
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "VspHub is the Vit desktop VSP protocol hub.\n\n")
		flag.PrintDefaults()
	}
}
