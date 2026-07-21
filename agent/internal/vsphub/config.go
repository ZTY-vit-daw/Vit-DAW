package vsphub

import (
	"strings"
	"time"
)

const (
	VSPVersion              = "1.0"
	DefaultHTTPAddr         = "127.0.0.1:8787"
	DefaultKernelReqURL     = "tcp://127.0.0.1:5555"
	DefaultKernelSubURL     = "tcp://127.0.0.1:5556"
	DefaultRequestTimeout   = 120 * time.Second
	DefaultReadHeaderTimout = 5 * time.Second
	MaxHTTPBodyBytes        = 16 * 1024 * 1024

	TransportHTTP      = "vsp.hub.http"
	TransportWebSocket = "vsp.hub.websocket"
)

type Config struct {
	HTTPAddr          string
	KernelReqURL      string
	KernelSubURL      string
	RequestTimeout    time.Duration
	ReadHeaderTimeout time.Duration
	EnableTelemetry   bool
	HubName           string
	HubVersion        string
}

func (c Config) WithDefaults() Config {
	if strings.TrimSpace(c.HTTPAddr) == "" {
		c.HTTPAddr = DefaultHTTPAddr
	}
	if strings.TrimSpace(c.KernelReqURL) == "" {
		c.KernelReqURL = DefaultKernelReqURL
	}
	if strings.TrimSpace(c.KernelSubURL) == "" {
		c.KernelSubURL = DefaultKernelSubURL
	}
	if c.RequestTimeout <= 0 {
		c.RequestTimeout = DefaultRequestTimeout
	}
	if c.ReadHeaderTimeout <= 0 {
		c.ReadHeaderTimeout = DefaultReadHeaderTimout
	}
	if strings.TrimSpace(c.HubName) == "" {
		c.HubName = "Vit VSP Hub"
	}
	if strings.TrimSpace(c.HubVersion) == "" {
		c.HubVersion = "vsp-hub-v1"
	}
	return c
}
