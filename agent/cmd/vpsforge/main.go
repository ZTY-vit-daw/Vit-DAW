package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"vit-daw-agent/internal/fxm"
	"vit-daw-agent/internal/pluginvps"
	"vit-daw-agent/internal/probeaudio"
	"vit-daw-agent/internal/vpsforge"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	var value any
	var err error
	switch os.Args[1] {
	case "draft":
		value, err = runPluginVPSDraft(os.Args[2:])
	case "verify":
		value, err = runPluginVPSVerify(os.Args[2:])
	case "init":
		value, err = runInit(os.Args[2:])
	case "status":
		value, err = runStatus(os.Args[2:])
	case "validate":
		value, err = runValidate(os.Args[2:])
	case "ingest-surface":
		value, err = runIngestSurface(os.Args[2:])
	case "measure":
		value, err = runMeasure(os.Args[2:])
	case "probe":
		value, err = runProbe(os.Args[2:])
	case "serve":
		err = runServe(os.Args[2:])
		if err == nil {
			return
		}
	case "host":
		err = runHost(os.Args[2:])
		if err == nil {
			return
		}
	case "test-audio":
		value, err = runTestAudio(os.Args[2:])
	default:
		usage()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "vpsforge:", err)
		os.Exit(1)
	}
	encoded, _ := json.MarshalIndent(value, "", "  ")
	fmt.Println(string(encoded))
}
func runPluginVPSDraft(args []string) (any, error) {
	fs := flag.NewFlagSet("draft", flag.ContinueOnError)
	surface := fs.String("surface", "", "surface_snapshot.json path")
	output := fs.String("output", "", "output <plugin>.vps.json path")
	if err := fs.Parse(args); err != nil {
		return nil, err
	}
	if fs.NArg() != 1 {
		return nil, fmt.Errorf("draft requires one plugin name, surface file, or workspace")
	}
	resolved, err := pluginvps.ResolveSurface(fs.Arg(0), *surface)
	if err != nil {
		return nil, err
	}
	doc, err := pluginvps.DraftFromSurfaceFile(resolved)
	if err != nil {
		return nil, err
	}
	path := strings.TrimSpace(*output)
	if path == "" {
		path = filepath.Join(pluginvps.DefaultDirectory(), pluginvps.Slug(doc.Plugin.Name)+".vps.json")
	}
	if err := pluginvps.Save(path, doc); err != nil {
		return nil, err
	}
	return map[string]any{"status": "draft", "path": path, "document": doc}, nil
}

func runPluginVPSVerify(args []string) (any, error) {
	fs := flag.NewFlagSet("verify", flag.ContinueOnError)
	worker := fs.String("worker", defaultVST3WorkerPath(), "native vpsforge_vst3_worker.exe path")
	if err := fs.Parse(args); err != nil {
		return nil, err
	}
	if fs.NArg() != 1 {
		return nil, fmt.Errorf("verify requires one <plugin>.vps.json file")
	}
	path := fs.Arg(0)
	doc, err := pluginvps.Load(path)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()
	result, err := pluginvps.Verify(ctx, doc, pluginvps.VerifyOptions{WorkerPath: *worker})
	if err != nil {
		return nil, err
	}
	if err := pluginvps.Save(path, result.Document); err != nil {
		return nil, err
	}
	return map[string]any{"status": "verified", "path": path, "checks": result.Checks, "parameters": result.Parameters, "verification": result.Document.Verification}, nil
}

func runInit(args []string) (any, error) {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	workspace := fs.String("workspace", "", "isolated authoring workspace directory")
	manufacturer := fs.String("manufacturer", "", "plugin manufacturer")
	name := fs.String("name", "", "plugin name")
	format := fs.String("format", "VST3", "VST3, CLAP, AAX or AU")
	version := fs.String("version", "", "plugin version")
	path := fs.String("install-path", "", "observed plugin installation path")
	capabilities := fs.String("capabilities", "unknown", "comma-separated candidate capability ids")
	host := fs.String("host-endpoint", "", "optional local host-adapter endpoint")
	if err := fs.Parse(args); err != nil {
		return nil, err
	}
	return vpsforge.Init(vpsforge.InitRequest{Root: *workspace, Identity: vpsforge.PluginIdentity{Manufacturer: *manufacturer, Name: *name, Format: *format, Version: *version, InstallPath: *path}, Capabilities: split(*capabilities), HostEndpoint: *host})
}

func runStatus(args []string) (any, error) {
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	root := fs.String("workspace", "", "workspace")
	if err := fs.Parse(args); err != nil {
		return nil, err
	}
	return vpsforge.Inspect(*root)
}
func runValidate(args []string) (any, error) {
	fs := flag.NewFlagSet("validate", flag.ContinueOnError)
	root := fs.String("workspace", "", "workspace")
	if err := fs.Parse(args); err != nil {
		return nil, err
	}
	return vpsforge.Validate(*root), nil
}

func runIngestSurface(args []string) (any, error) {
	fs := flag.NewFlagSet("ingest-surface", flag.ContinueOnError)
	root := fs.String("workspace", "", "workspace")
	input := fs.String("input", "", "surface snapshot JSON")
	source := fs.String("source", "manual-host-export", "evidence source")
	if err := fs.Parse(args); err != nil {
		return nil, err
	}
	var snapshot vpsforge.SurfaceSnapshot
	if err := readJSON(*input, &snapshot); err != nil {
		return nil, err
	}
	return vpsforge.IngestSurface(*root, snapshot, *source, snapshot.CapturedAt)
}

func runMeasure(args []string) (any, error) {
	fs := flag.NewFlagSet("measure", flag.ContinueOnError)
	root := fs.String("workspace", "", "workspace")
	input := fs.String("input", "", "FXM input JSON containing baseline and processed measurements")
	if err := fs.Parse(args); err != nil {
		return nil, err
	}
	var measurement fxm.Input
	if err := readJSON(*input, &measurement); err != nil {
		return nil, err
	}
	return vpsforge.RecordFXM(*root, measurement, timeFromString(measurement.CreatedAt))
}

func runProbe(args []string) (any, error) {
	fs := flag.NewFlagSet("probe", flag.ContinueOnError)
	root := fs.String("workspace", "", "workspace")
	host := fs.String("host", "", "local plugin-host adapter base URL")
	if err := fs.Parse(args); err != nil {
		return nil, err
	}
	return vpsforge.ProbeHost(context.Background(), *root, *host, nil)
}

func runServe(args []string) error {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	root := fs.String("workspace", "", "workspace")
	listen := fs.String("listen", "127.0.0.1:8899", "loopback listen address")
	if err := fs.Parse(args); err != nil {
		return err
	}
	host, _, err := net.SplitHostPort(*listen)
	if err != nil {
		return err
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return fmt.Errorf("vpsforge serve must bind to a loopback address")
	}
	if _, err := vpsforge.Inspect(*root); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "vpsforge serving %s on http://%s\n", *root, *listen)
	return http.ListenAndServe(*listen, vpsforge.Handler(*root))
}

// runHost starts the independent VST3 adapter. It is intentionally separate
// from `serve`: this process can host a plugin but does not own a Forge
// workspace, VPS Library, Credential, Catalog, or SPAL route.
func runHost(args []string) error {
	fs := flag.NewFlagSet("host", flag.ContinueOnError)
	listen := fs.String("listen", "127.0.0.1:8900", "loopback listen address")
	worker := fs.String("worker", defaultVST3WorkerPath(), "native vpsforge_vst3_worker.exe path")
	if err := fs.Parse(args); err != nil {
		return err
	}
	host, _, err := net.SplitHostPort(*listen)
	if err != nil {
		return err
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return fmt.Errorf("vpsforge host must bind to a loopback address")
	}
	adapter := vpsforge.NewVST3HostAdapter(*worker)
	defer adapter.Close()
	fmt.Fprintf(os.Stderr, "vpsforge VST3 host adapter listening on http://%s\n", *listen)
	return http.ListenAndServe(*listen, adapter.Handler())
}
func defaultVST3WorkerPath() string {
	executable, err := os.Executable()
	if err != nil {
		return "vpsforge_vst3_worker.exe"
	}
	root := filepath.Dir(executable)
	// This first candidate is the release packaging location. The second makes
	// local source-tree development reproducible without touching VitApp.
	for _, candidate := range []string{
		filepath.Join(root, "vpsforge_vst3_worker.exe"),
		filepath.Join(root, "native-host", "build", "vpsforge_vst3_worker_artefacts", "Release", "vpsforge_vst3_worker.exe"),
	} {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return filepath.Join(root, "vpsforge_vst3_worker.exe")
}
func runTestAudio(args []string) (any, error) {
	fs := flag.NewFlagSet("test-audio", flag.ContinueOnError)
	output := fs.String("output", "", "probe-audio output directory")
	if err := fs.Parse(args); err != nil {
		return nil, err
	}
	return probeaudio.Generate(*output, time.Now().UTC())
}
func readJSON(path string, output any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, output)
}

func split(value string) []string {
	out := []string{}
	for _, item := range strings.Split(value, ",") {
		if item = strings.TrimSpace(item); item != "" {
			out = append(out, item)
		}
	}
	return out
}

func timeFromString(value string) time.Time {
	parsed, _ := time.Parse(time.RFC3339Nano, strings.TrimSpace(value))
	return parsed
}
func usage() {
	fmt.Fprintln(os.Stderr, "usage: vpsforge <draft|verify|init|status|validate|ingest-surface|measure|probe|serve|host|test-audio> [flags]")
}
