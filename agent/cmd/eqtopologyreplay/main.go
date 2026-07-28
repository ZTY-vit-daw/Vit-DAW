package main

import (
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"vit-daw-agent/internal/workflows/plugingrabber"
)

type capturePage struct {
	Parameters     []json.RawMessage `json:"parameters"`
	TemplateRole   string            `json:"template_role"`
	PluginClass    string            `json:"plugin_class"`
	PluginIdentity map[string]any    `json:"plugin_identity"`
}

type replayCase struct {
	CaseID               string           `json:"case_id"`
	ParameterCount       int              `json:"parameter_count"`
	Recognized           bool             `json:"recognized"`
	Executable           bool             `json:"executable"`
	PublicClassification string           `json:"public_classification,omitempty"`
	TopologyGeneration   string           `json:"topology_generation,omitempty"`
	SupportedShapes      []string         `json:"supported_shapes,omitempty"`
	RejectionCodes       []string         `json:"rejection_codes,omitempty"`
	ShapeCapabilities    []map[string]any `json:"shape_capabilities,omitempty"`
	Sections             []map[string]any `json:"sections,omitempty"`
	Error                string           `json:"error,omitempty"`
}

type replayReport struct {
	SchemaVersion             string               `json:"schema_version"`
	CaptureRoot               string               `json:"capture_root"`
	CaseCount                 int                  `json:"case_count"`
	RecognizedCount           int                  `json:"recognized_count"`
	ExecutableCount           int                  `json:"executable_count"`
	ExpectedCapabilityRows    int                  `json:"expected_capability_rows"`
	CapabilityMismatchCount   int                  `json:"capability_mismatch_count"`
	CapabilityRegressionCount int                  `json:"capability_regression_count"`
	CapabilityExpansionCount  int                  `json:"capability_expansion_count"`
	CapabilityMismatches      []capabilityMismatch `json:"capability_mismatches,omitempty"`
	Cases                     []replayCase         `json:"cases"`
}

type capabilityMismatch struct {
	CaseID          string `json:"case_id"`
	Shape           string `json:"shape"`
	Action          string `json:"action"`
	ExpectedStatus  string `json:"expected_status"`
	ActualSupported bool   `json:"actual_supported"`
}

func main() {
	root := flag.String("root", "", "phase-1 raw/pages directory")
	out := flag.String("out", "", "optional JSON report path")
	expect := flag.String("expect", "", "optional approved shape_action_matrix.csv")
	flag.Parse()
	if strings.TrimSpace(*root) == "" {
		fatalf("-root is required")
	}
	absRoot, err := filepath.Abs(*root)
	if err != nil {
		fatalf("resolve root: %v", err)
	}
	entries, err := os.ReadDir(absRoot)
	if err != nil {
		fatalf("read root: %v", err)
	}
	report := replayReport{SchemaVersion: "waves-static-eq-production-replay/v1", CaptureRoot: absRoot}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		item := replayOne(filepath.Join(absRoot, entry.Name()), entry.Name())
		report.Cases = append(report.Cases, item)
		if item.Recognized {
			report.RecognizedCount++
		}
		if item.Executable {
			report.ExecutableCount++
		}
	}
	sort.Slice(report.Cases, func(i, j int) bool { return report.Cases[i].CaseID < report.Cases[j].CaseID })
	report.CaseCount = len(report.Cases)
	expected := map[string]string{}
	if strings.TrimSpace(*expect) != "" {
		expected, err = compareExpectedCapabilities(&report, *expect)
		if err != nil {
			fatalf("compare expected capabilities: %v", err)
		}
	}
	encoded, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		fatalf("encode report: %v", err)
	}
	encoded = append(encoded, '\n')
	if strings.TrimSpace(*out) == "" {
		_, _ = os.Stdout.Write(encoded)
		return
	}
	if err := os.MkdirAll(filepath.Dir(*out), 0o755); err != nil {
		fatalf("create report directory: %v", err)
	}
	if err := os.WriteFile(*out, encoded, 0o644); err != nil {
		fatalf("write report: %v", err)
	}
	artifacts, err := writeAcceptanceArtifacts(*out, report, expected)
	if err != nil {
		fatalf("write acceptance artifacts: %v", err)
	}
	fmt.Printf("production replay: cases=%d recognized=%d executable=%d report=%s artifacts=%s\n",
		report.CaseCount, report.RecognizedCount, report.ExecutableCount, *out, strings.Join(artifacts, ","))
	if report.CapabilityRegressionCount > 0 {
		os.Exit(2)
	}
}

func compareExpectedCapabilities(report *replayReport, path string) (map[string]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	rowsCSV, err := csv.NewReader(file).ReadAll()
	if err != nil {
		return nil, err
	}
	if len(rowsCSV) < 2 {
		return nil, fmt.Errorf("expectation CSV is empty")
	}
	header := map[string]int{}
	for index, value := range rowsCSV[0] {
		value = strings.TrimPrefix(value, "\ufeff")
		header[value] = index
	}
	for _, required := range []string{"case_id", "shape", "action", "status"} {
		if _, ok := header[required]; !ok {
			return nil, fmt.Errorf("expectation CSV is missing %s", required)
		}
	}
	actual := map[string]bool{}
	for _, item := range report.Cases {
		for _, capability := range item.ShapeCapabilities {
			shape := text(capability["shape"])
			actions, _ := capability["actions"].(map[string]any)
			for action, value := range actions {
				supported, _ := value.(bool)
				actual[item.CaseID+"\x00"+shape+"\x00"+action] = supported
			}
		}
	}
	expected := map[string]string{}
	for _, row := range rowsCSV[1:] {
		if len(row) < len(rowsCSV[0]) {
			continue
		}
		caseID, shape, action, status := row[header["case_id"]], row[header["shape"]], row[header["action"]], row[header["status"]]
		report.ExpectedCapabilityRows++
		key := caseID + "\x00" + shape + "\x00" + action
		expected[key] = status
		got := actual[key]
		want := status != "rejected"
		if got != want {
			if want {
				report.CapabilityRegressionCount++
			} else {
				report.CapabilityExpansionCount++
			}
			report.CapabilityMismatches = append(report.CapabilityMismatches, capabilityMismatch{
				CaseID: caseID, Shape: shape, Action: action, ExpectedStatus: status, ActualSupported: got,
			})
		}
	}
	report.CapabilityMismatchCount = len(report.CapabilityMismatches)
	return expected, nil
}

func replayOne(directory, caseID string) replayCase {
	item := replayCase{CaseID: caseID}
	files, err := filepath.Glob(filepath.Join(directory, "page_*.json"))
	if err != nil || len(files) == 0 {
		item.Error = firstError(err, "no capture pages")
		return item
	}
	sort.Strings(files)
	params := []plugingrabber.ParameterInfo{}
	templateRole := ""
	pluginClass := ""
	pluginIdentity := map[string]any(nil)
	for _, file := range files {
		payload, err := os.ReadFile(file)
		if err != nil {
			item.Error = err.Error()
			return item
		}
		var page capturePage
		if err := json.Unmarshal(payload, &page); err != nil {
			item.Error = err.Error()
			return item
		}
		if templateRole == "" {
			templateRole = strings.TrimSpace(page.TemplateRole)
		}
		if pluginClass == "" {
			pluginClass = strings.TrimSpace(page.PluginClass)
		}
		if pluginIdentity == nil && len(page.PluginIdentity) > 0 {
			pluginIdentity = page.PluginIdentity
		}
		for _, raw := range page.Parameters {
			var row map[string]any
			if err := json.Unmarshal(raw, &row); err != nil {
				item.Error = err.Error()
				return item
			}
			if _, exists := row["id"]; !exists {
				row["id"] = row["param_id"]
			}
			normalized, _ := json.Marshal(row)
			var param plugingrabber.ParameterInfo
			if err := json.Unmarshal(normalized, &param); err != nil {
				item.Error = err.Error()
				return item
			}
			params = append(params, param)
		}
	}
	item.ParameterCount = len(params)
	summary := plugingrabber.BuildEQBandSummary(plugingrabber.ParameterDigest{
		Parameters: params, ParameterCount: len(params), TemplateRole: templateRole,
		PluginClass: pluginClass, PluginIdentity: pluginIdentity,
	})
	if summary == nil {
		item.RejectionCodes = []string{"not_static_eq"}
		return item
	}
	item.Recognized = true
	item.PublicClassification = text(summary["eq_model"])
	item.SupportedShapes = stringsValue(summary["supported_filter_kinds"])
	topology, _ := summary["control_topology"].(map[string]any)
	item.TopologyGeneration = text(topology["generation"])
	item.ShapeCapabilities = rows(topology["shape_capabilities"])
	item.Executable = caseHasExecutableShape(item.ShapeCapabilities)
	item.RejectionCodes = caseRejectionCodes(summary, item.ShapeCapabilities)
	for _, section := range rows(summary["sections"]) {
		item.Sections = append(item.Sections, map[string]any{
			"section": section["section"], "complete": section["complete"], "addressing": section["addressing"],
			"reachable_kinds": section["reachable_kinds"], "exclusion_codes": section["exclusion_codes"],
			"shape_capabilities": section["shape_capabilities"],
		})
	}
	return item
}

func caseHasExecutableShape(capabilities []map[string]any) bool {
	for _, capability := range capabilities {
		if supported, _ := mapValue(capability["actions"])["upsert"].(bool); supported {
			return true
		}
	}
	return false
}

func caseRejectionCodes(summary map[string]any, capabilities []map[string]any) []string {
	seen := map[string]bool{}
	for _, section := range rows(summary["sections"]) {
		for _, code := range stringsValue(section["exclusion_codes"]) {
			if code != "" {
				seen[code] = true
			}
		}
	}
	for _, capability := range capabilities {
		for _, value := range mapValue(capability["rejection_codes"]) {
			for _, code := range stringsValue(value) {
				if code != "" {
					seen[code] = true
				}
			}
		}
	}
	out := make([]string, 0, len(seen))
	for code := range seen {
		out = append(out, code)
	}
	sort.Strings(out)
	return out
}

func rows(value any) []map[string]any {
	if typed, ok := value.([]map[string]any); ok {
		return typed
	}
	if typed, ok := value.([]any); ok {
		out := make([]map[string]any, 0, len(typed))
		for _, value := range typed {
			if row, ok := value.(map[string]any); ok {
				out = append(out, row)
			}
		}
		return out
	}
	return nil
}

func stringsValue(value any) []string {
	switch typed := value.(type) {
	case []string:
		return typed
	case []any:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			out = append(out, text(item))
		}
		return out
	default:
		return nil
	}
}

func mapValue(value any) map[string]any {
	typed, _ := value.(map[string]any)
	return typed
}

func writeAcceptanceArtifacts(jsonPath string, report replayReport, expected map[string]string) ([]string, error) {
	directory := filepath.Dir(jsonPath)
	casePath := filepath.Join(directory, "production_replay_cases.csv")
	capabilityPath := filepath.Join(directory, "production_replay_capabilities.csv")
	summaryPath := filepath.Join(directory, "production_replay_summary.md")
	if err := writeCaseCSV(casePath, report); err != nil {
		return nil, err
	}
	if err := writeCapabilityCSV(capabilityPath, report, expected); err != nil {
		return nil, err
	}
	if err := writeReplaySummary(summaryPath, report); err != nil {
		return nil, err
	}
	return []string{casePath, capabilityPath, summaryPath}, nil
}

func writeCaseCSV(path string, report replayReport) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	writer := csv.NewWriter(file)
	writeErr := writer.Write([]string{"case_id", "parameter_count", "recognized", "executable",
		"public_classification", "supported_shapes", "rejection_codes", "topology_generation", "error"})
	for _, item := range report.Cases {
		if writeErr != nil {
			break
		}
		writeErr = writer.Write([]string{item.CaseID, fmt.Sprint(item.ParameterCount), fmt.Sprint(item.Recognized),
			fmt.Sprint(item.Executable), item.PublicClassification, strings.Join(item.SupportedShapes, "|"),
			strings.Join(item.RejectionCodes, "|"), item.TopologyGeneration, item.Error})
	}
	writer.Flush()
	if writeErr == nil {
		writeErr = writer.Error()
	}
	if closeErr := file.Close(); writeErr == nil {
		writeErr = closeErr
	}
	return writeErr
}

type replayActualCapability struct {
	Supported      bool
	RejectionCodes []string
}

func writeCapabilityCSV(path string, report replayReport, expected map[string]string) error {
	actual := map[string]replayActualCapability{}
	caseByID := map[string]replayCase{}
	for _, item := range report.Cases {
		caseByID[item.CaseID] = item
		for _, capability := range item.ShapeCapabilities {
			shape := text(capability["shape"])
			actions := mapValue(capability["actions"])
			rejections := mapValue(capability["rejection_codes"])
			for action, value := range actions {
				supported, _ := value.(bool)
				actual[item.CaseID+"\x00"+shape+"\x00"+action] = replayActualCapability{
					Supported: supported, RejectionCodes: stringsValue(rejections[action]),
				}
			}
		}
	}
	keys := make([]string, 0, len(expected))
	for key := range expected {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	writer := csv.NewWriter(file)
	writeErr := writer.Write([]string{"case_id", "recognized", "executable", "public_classification",
		"shape", "action", "expected_status", "actual_status", "comparison", "rejection_codes"})
	for _, key := range keys {
		if writeErr != nil {
			break
		}
		parts := strings.Split(key, "\x00")
		if len(parts) != 3 {
			continue
		}
		item := caseByID[parts[0]]
		capability := actual[key]
		expectedStatus := expected[key]
		actualStatus := "rejected"
		if capability.Supported {
			actualStatus = "supported"
		}
		comparison := "matched"
		expectedSupported := expectedStatus != "rejected"
		if capability.Supported && !expectedSupported {
			comparison = "expansion"
		} else if !capability.Supported && expectedSupported {
			comparison = "regression"
		}
		writeErr = writer.Write([]string{parts[0], fmt.Sprint(item.Recognized), fmt.Sprint(item.Executable),
			item.PublicClassification, parts[1], parts[2], expectedStatus, actualStatus, comparison,
			strings.Join(capability.RejectionCodes, "|")})
	}
	writer.Flush()
	if writeErr == nil {
		writeErr = writer.Error()
	}
	if closeErr := file.Close(); writeErr == nil {
		writeErr = closeErr
	}
	return writeErr
}

func writeReplaySummary(path string, report replayReport) error {
	classifications := map[string]int{}
	for _, item := range report.Cases {
		classification := item.PublicClassification
		if classification == "" {
			classification = "unrecognized"
		}
		classifications[classification]++
	}
	classificationNames := make([]string, 0, len(classifications))
	for name := range classifications {
		classificationNames = append(classificationNames, name)
	}
	sort.Strings(classificationNames)
	var out strings.Builder
	fmt.Fprintln(&out, "# Waves static EQ production replay acceptance")
	fmt.Fprintln(&out)
	fmt.Fprintf(&out, "- Schema: `%s`\n", report.SchemaVersion)
	fmt.Fprintf(&out, "- Capture cases: %d\n", report.CaseCount)
	fmt.Fprintf(&out, "- Structural models: %d\n", report.RecognizedCount)
	fmt.Fprintf(&out, "- Generically executable models: %d\n", report.ExecutableCount)
	fmt.Fprintf(&out, "- Shape/action rows: %d\n", report.ExpectedCapabilityRows)
	fmt.Fprintf(&out, "- Capability regressions: %d\n", report.CapabilityRegressionCount)
	fmt.Fprintf(&out, "- Conservative-model capability expansions: %d\n", report.CapabilityExpansionCount)
	fmt.Fprintln(&out)
	fmt.Fprintln(&out, "## Public compatibility projection")
	fmt.Fprintln(&out)
	for _, name := range classificationNames {
		fmt.Fprintf(&out, "- `%s`: %d\n", name, classifications[name])
	}
	if len(report.CapabilityMismatches) > 0 {
		fmt.Fprintln(&out)
		fmt.Fprintln(&out, "## Capability differences from phase 2")
		fmt.Fprintln(&out)
		fmt.Fprintln(&out, "| Case | Shape | Action | Phase 2 | Production |")
		fmt.Fprintln(&out, "|---|---|---|---|---|")
		for _, mismatch := range report.CapabilityMismatches {
			actual := "rejected"
			if mismatch.ActualSupported {
				actual = "supported"
			}
			fmt.Fprintf(&out, "| %s | %s | %s | %s | %s |\n", mismatch.CaseID, mismatch.Shape,
				mismatch.Action, mismatch.ExpectedStatus, actual)
		}
	}
	fmt.Fprintln(&out)
	fmt.Fprintln(&out, "## Per-model result")
	fmt.Fprintln(&out)
	fmt.Fprintln(&out, "| Case | Params | Model | Executable | Shapes | Rejection evidence |")
	fmt.Fprintln(&out, "|---|---:|---|---|---|---|")
	for _, item := range report.Cases {
		classification := item.PublicClassification
		if classification == "" {
			classification = "unrecognized"
		}
		fmt.Fprintf(&out, "| %s | %d | %s | %t | %s | %s |\n", item.CaseID, item.ParameterCount,
			classification, item.Executable, strings.Join(item.SupportedShapes, ", "), strings.Join(item.RejectionCodes, ", "))
	}
	return os.WriteFile(path, []byte(out.String()), 0o644)
}

func text(value any) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

func firstError(err error, fallback string) string {
	if err != nil {
		return err.Error()
	}
	return fallback
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
