package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"vit-daw-agent/internal/pluginsemantics"
	"vit-daw-agent/internal/processorattestation"
	"vit-daw-agent/internal/processorauthority"
)

type stringList []string

func (values *stringList) String() string { return strings.Join(*values, ",") }
func (values *stringList) Set(value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return fmt.Errorf("empty path")
	}
	*values = append(*values, value)
	return nil
}

func main() {
	if len(os.Args) < 2 {
		fail("usage: pcactl <import|certify-compressor|list|query|fingerprint|promote|stale|revoke>")
	}
	switch os.Args[1] {
	case "import":
		runImport(os.Args[2:])
	case "certify-compressor":
		runCertifyCompressor(os.Args[2:])
	case "list":
		runList(os.Args[2:])
	case "query":
		runQuery(os.Args[2:])
	case "fingerprint":
		runFingerprint(os.Args[2:])
	case "promote", "stale", "revoke":
		runTransition(os.Args[1], os.Args[2:])
	default:
		fail("unknown pcactl command " + os.Args[1])
	}
}

func runCertifyCompressor(args []string) {
	set := flag.NewFlagSet("certify-compressor", flag.ExitOnError)
	semanticsPath := set.String("semantics", "", "plugin semantics index; default is ~/.vit/plugin_semantics.json")
	storePath := set.String("store", "", "attestation store; default is ~/.vit/processor_control_attestations.v1.json")
	identifier := set.String("identifier", "", "exact installed plug-in identifier from the semantic index")
	outputDir := set.String("output-dir", "", "directory for the local certification receipt")
	agentHTTP := set.String("agent-http", "http://127.0.0.1:7878", "Agent HTTP base URL")
	timeout := set.Duration("timeout", 180*time.Second, "per-call certification timeout")
	_ = set.Parse(args)
	if strings.TrimSpace(*identifier) == "" || strings.TrimSpace(*outputDir) == "" {
		fail("certify-compressor requires -identifier and -output-dir")
	}
	index, err := pluginsemantics.Load(*semanticsPath)
	check(err)
	entry, err := exactEntry(index, *identifier)
	check(err)
	report, err := processorauthority.CertifyCompressor(processorauthority.LocalCertificationOptions{
		AgentHTTP: *agentHTTP, Entry: entry, OutputDir: *outputDir, Timeout: *timeout,
	})
	if err != nil {
		writeJSON(report)
		fail(err.Error())
	}
	store, err := processorattestation.NewStore(*storePath)
	check(err)
	importReport, err := processorauthority.ImportReceipts(store, index, []string{report.SummaryPath})
	check(err)
	writeJSON(struct {
		Certification processorauthority.LocalCertificationReport `json:"certification"`
		Import        processorauthority.ImportReport             `json:"import"`
	}{report, importReport})
}

func runImport(args []string) {
	set := flag.NewFlagSet("import", flag.ExitOnError)
	var receipts stringList
	set.Var(&receipts, "receipt", "strong receipt summary path; repeat for multiple receipts")
	semanticsPath := set.String("semantics", "", "plugin semantics index; default is ~/.vit/plugin_semantics.json")
	storePath := set.String("store", "", "attestation store; default is ~/.vit/processor_control_attestations.v1.json")
	_ = set.Parse(args)
	if len(receipts) == 0 {
		fail("import requires at least one -receipt")
	}
	index, err := pluginsemantics.Load(*semanticsPath)
	check(err)
	store, err := processorattestation.NewStore(*storePath)
	check(err)
	report, err := processorauthority.ImportReceipts(store, index, receipts)
	check(err)
	writeJSON(report)
}

func runList(args []string) {
	set := flag.NewFlagSet("list", flag.ExitOnError)
	storePath := set.String("store", "", "attestation store")
	_ = set.Parse(args)
	store, err := processorattestation.NewStore(*storePath)
	check(err)
	library, readReport, err := store.Read()
	check(err)
	writeJSON(struct {
		Library processorattestation.Library    `json:"library"`
		Read    processorattestation.ReadReport `json:"read"`
	}{library, readReport})
}

func runQuery(args []string) {
	set := flag.NewFlagSet("query", flag.ExitOnError)
	storePath := set.String("store", "", "attestation store")
	semanticsPath := set.String("semantics", "", "plugin semantics index")
	identifier := set.String("identifier", "", "exact installed plug-in identifier")
	subjectKey := set.String("subject-key", "", "stable PCA subject key")
	pluginPath := set.String("plugin-path", "", "current binary path when subject-key is used")
	fingerprint := set.String("fingerprint", "", "current sha256 fingerprint override")
	family := set.String("family", "", "static_eq or broadband_compressor")
	action := set.String("action", "", "current action")
	shape := set.String("shape", "", "static EQ shape")
	axis := set.String("axis", "", "compressor semantic axis")
	_ = set.Parse(args)
	key := strings.TrimSpace(*subjectKey)
	path := strings.TrimSpace(*pluginPath)
	if key == "" {
		if strings.TrimSpace(*identifier) == "" {
			fail("query requires -subject-key or exact -identifier")
		}
		index, err := pluginsemantics.Load(*semanticsPath)
		check(err)
		entry, err := exactEntry(index, *identifier)
		check(err)
		subject := processorattestation.Subject{Name: entry.Name, Manufacturer: entry.Manufacturer, Format: entry.Format,
			Identifier: entry.Identifier, InstalledPath: entry.PluginPath}
		key, err = processorattestation.BuildSubjectKey(subject)
		check(err)
		path = entry.PluginPath
	}
	currentFingerprint := strings.TrimSpace(*fingerprint)
	if currentFingerprint == "" {
		if path == "" {
			fail("query requires -plugin-path or -fingerprint with -subject-key")
		}
		var err error
		currentFingerprint, err = processorattestation.FingerprintPath(path)
		check(err)
	}
	store, err := processorattestation.NewStore(*storePath)
	check(err)
	result, err := store.Query(processorattestation.Query{SubjectKey: key, BinaryFingerprint: currentFingerprint,
		ProcessorFamily: *family, RequiredCoverage: []processorattestation.Coverage{{Action: *action, Shape: *shape, Axis: *axis}}})
	check(err)
	writeJSON(result)
}

func runFingerprint(args []string) {
	set := flag.NewFlagSet("fingerprint", flag.ExitOnError)
	path := set.String("path", "", "plug-in file or bundle path")
	_ = set.Parse(args)
	fingerprint, err := processorattestation.FingerprintPath(*path)
	check(err)
	writeJSON(map[string]string{"path": filepath.Clean(*path), "binary_fingerprint": fingerprint})
}

func runTransition(command string, args []string) {
	set := flag.NewFlagSet(command, flag.ExitOnError)
	storePath := set.String("store", "", "attestation store")
	id := set.String("id", "", "attestation ID")
	reason := set.String("reason", "", "required audit reason")
	_ = set.Parse(args)
	store, err := processorattestation.NewStore(*storePath)
	check(err)
	var attestation processorattestation.Attestation
	switch command {
	case "promote":
		attestation, err = store.Promote(*id, *reason)
	case "stale":
		attestation, err = store.MarkStale(*id, *reason)
	case "revoke":
		attestation, err = store.Revoke(*id, *reason)
	}
	check(err)
	writeJSON(attestation)
}

func exactEntry(index pluginsemantics.Index, identifier string) (pluginsemantics.Entry, error) {
	identifier = strings.TrimSpace(identifier)
	for _, entry := range index.Entries {
		if strings.EqualFold(entry.Identifier, identifier) {
			return entry, nil
		}
	}
	return pluginsemantics.Entry{}, fmt.Errorf("exact plug-in identifier %q not found", identifier)
}

func writeJSON(value any) {
	encoded, err := json.MarshalIndent(value, "", "  ")
	check(err)
	_, _ = os.Stdout.Write(append(encoded, '\n'))
}

func check(err error) {
	if err != nil {
		fail(err.Error())
	}
}

func fail(message string) {
	fmt.Fprintln(os.Stderr, message)
	os.Exit(1)
}
