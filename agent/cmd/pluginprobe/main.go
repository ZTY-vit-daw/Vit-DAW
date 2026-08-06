package main

import (
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"vit-daw-agent/internal/pluginprobe"
)

func main() {
	worker := flag.String("worker", defaultWorkerPath(), "native pluginprobe_vst3_worker.exe path")
	listen := flag.String("listen", "127.0.0.1:9318", "loopback HTTP listen address")
	flag.Parse()

	if !loopbackAddress(*listen) {
		fmt.Fprintln(os.Stderr, "pluginprobe: listen address must be loopback")
		os.Exit(2)
	}
	adapter := pluginprobe.NewVST3HostAdapter(*worker)
	defer adapter.Close()
	fmt.Fprintf(os.Stderr, "pluginprobe observation host listening on http://%s\n", *listen)
	if err := http.ListenAndServe(*listen, adapter.Handler()); err != nil {
		fmt.Fprintln(os.Stderr, "pluginprobe:", err)
		os.Exit(1)
	}
}

func loopbackAddress(address string) bool {
	host, _, err := net.SplitHostPort(strings.TrimSpace(address))
	if err != nil {
		return false
	}
	ip := net.ParseIP(strings.Trim(host, "[]"))
	return ip != nil && ip.IsLoopback()
}

func defaultWorkerPath() string {
	if configured := strings.TrimSpace(os.Getenv("VIT_PLUGINPROBE_WORKER")); configured != "" {
		return configured
	}
	name := "pluginprobe_vst3_worker"
	if strings.EqualFold(filepath.Ext(os.Args[0]), ".exe") {
		name += ".exe"
	}
	candidates := []string{name}
	if executable, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(executable), name))
	}
	if cwd, err := os.Getwd(); err == nil {
		candidates = append(candidates,
			filepath.Join(cwd, "PluginProbe", "native-host", "build", "pluginprobe_vst3_worker_artefacts", "Release", name),
			filepath.Join(filepath.Dir(cwd), "PluginProbe", "native-host", "build", "pluginprobe_vst3_worker_artefacts", "Release", name),
		)
	}
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate
		}
	}
	return name
}
