package plugingrabber

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

const multibandTopologySchema = "multiband-dynamics-control-topology/v1"

// MultibandModel describes only a provable filterbank plus repeated dynamics
// cells. Names and plugin identity are never consulted by the recognizer.
type MultibandModel struct {
	Classification string
	Confidence     float64
	Crossovers     []CompressorBinding
	Bands          []MultibandBand
	SharedControls []CompressorBinding
	Auxiliary      []MultibandAuxiliaryStage
	ExclusionCodes []string
	Generation     string
}

type MultibandBand struct {
	Key            string
	Index          int
	OperatingPoint []CompressorBinding
	Transfer       []CompressorBinding
	Timing         []CompressorBinding
	GainAction     []CompressorBinding
	Mode           []CompressorBinding
}

type MultibandAuxiliaryStage struct {
	Kind     string
	ParamIDs []string
}

type multibandCandidate struct {
	Param          ParameterInfo
	Role           string
	Section        string
	BandKey        string
	BandIdx        int
	Evidence       int
	LocalCrossover bool
}

var multibandBandToken = regexp.MustCompile(`(?i)(?:^|[^a-z0-9])(?:band|b)\s*([0-9]+)(?:[^a-z0-9]|$)`)
var multibandNamedBandToken = regexp.MustCompile(`(?i)(?:^|[^a-z0-9])(low-mid|high-mid|low|mid|high)(?:[^a-z0-9]|$)`)
var multibandTrailingIndex = regexp.MustCompile(`(?i)(?:^|[^a-z0-9])([0-9]+)\s*$`)

func DetectMultibandModel(digest ParameterDigest) *MultibandModel {
	model, _ := DetectMultibandModelWithBoundary(digest)
	return model
}

func DetectMultibandModelWithBoundary(digest ParameterDigest) (*MultibandModel, string) {
	candidates := make([]multibandCandidate, 0, len(digest.Parameters))
	aux := map[string][]string{}
	for _, param := range digest.Parameters {
		if kind := multibandAuxiliaryKind(param); kind != "" {
			aux[kind] = append(aux[kind], param.ID)
			continue
		}
		c := classifyMultibandParameter(param)
		if c.Role == "" || !param.HostControllable {
			continue
		}
		candidates = append(candidates, c)
	}
	crossovers := []CompressorBinding{}
	localCrossovers := []multibandCandidate{}
	bandRows := map[string][]multibandCandidate{}
	shared := []multibandCandidate{}
	for _, c := range candidates {
		if c.Section == "frequency" {
			if c.LocalCrossover {
				localCrossovers = append(localCrossovers, c)
				continue
			}
			crossovers = append(crossovers, multibandBinding(c, "shared"))
			continue
		}
		if c.BandKey == "" {
			shared = append(shared, c)
			continue
		}
		bandRows[c.BandKey] = append(bandRows[c.BandKey], c)
	}
	if len(localCrossovers) > 0 {
		return nil, "unresolved_band_local_crossovers"
	}
	sort.Slice(crossovers, func(i, j int) bool {
		left, right := multibandCrossoverOrdinal(crossovers[i].Name), multibandCrossoverOrdinal(crossovers[j].Name)
		if left != right {
			return left < right
		}
		return crossovers[i].ParamID < crossovers[j].ParamID
	})
	if len(crossovers) == 0 {
		if len(aux["dynamic_eq"]) > 0 {
			return nil, "unsupported_dynamic_eq"
		}
		if len(aux["de_esser"]) > 0 {
			return nil, "unsupported_de_esser"
		}
		if len(aux["spectral_dynamics"]) > 0 {
			return nil, "unsupported_spectral_dynamics"
		}
		if len(aux["clipper"]) > 0 {
			return nil, "unsupported_clipper"
		}
		return nil, "not_multiband_dynamics"
	}
	if !multibandCrossoversOrdered(crossovers) {
		return nil, "unresolved_crossover_order"
	}
	if len(aux["dynamic_eq"]) > 0 {
		return nil, "unsupported_dynamic_eq"
	}
	if len(aux["de_esser"]) > 0 {
		return nil, "unsupported_de_esser"
	}
	if len(aux["spectral_dynamics"]) > 0 {
		return nil, "unsupported_spectral_dynamics"
	}
	if len(aux["clipper"]) > 0 {
		return nil, "unsupported_clipper"
	}
	if len(aux["maximizer"]) > 0 || len(aux["limiter"]) > 0 {
		return nil, "unsupported_multiband_maximizer"
	}
	bands := buildMultibandBands(bandRows)
	if len(bands) < 2 {
		return nil, "not_multiband_dynamics"
	}
	complete := 0
	for _, band := range bands {
		if multibandBandComplete(band) {
			complete++
		}
	}
	if complete < 2 {
		return nil, "unresolved_multiband_cells"
	}
	if len(shared) == 0 {
		return nil, "unresolved_shared_modifiers"
	}
	model := &MultibandModel{Classification: "filterbank_repeated_dynamics", Confidence: 0.96, Crossovers: crossovers, Bands: bands}
	for kind, ids := range aux {
		sort.Strings(ids)
		model.Auxiliary = append(model.Auxiliary, MultibandAuxiliaryStage{Kind: kind, ParamIDs: ids})
	}
	for _, c := range shared {
		model.SharedControls = append(model.SharedControls, multibandBinding(c, "shared"))
	}
	sort.Slice(model.Auxiliary, func(i, j int) bool { return model.Auxiliary[i].Kind < model.Auxiliary[j].Kind })
	sort.Slice(model.SharedControls, func(i, j int) bool { return model.SharedControls[i].ParamID < model.SharedControls[j].ParamID })
	model.Generation = multibandTopologyGeneration(model)
	return model, ""
}

func BuildMultibandSummary(digest ParameterDigest) map[string]any {
	summary, _ := BuildMultibandSummaryWithBoundary(digest)
	return summary
}

func BuildMultibandSummaryWithBoundary(digest ParameterDigest) (map[string]any, string) {
	model, boundary := DetectMultibandModelWithBoundary(digest)
	if model == nil {
		return nil, boundary
	}
	bands := make([]map[string]any, 0, len(model.Bands))
	for _, band := range model.Bands {
		bands = append(bands, map[string]any{
			"band_key": band.Key, "band_index": band.Index,
			"operating_point": compressorBindingRows(band.OperatingPoint),
			"transfer":        compressorBindingRows(band.Transfer),
			"timing":          compressorBindingRows(band.Timing),
			"gain_action":     compressorBindingRows(band.GainAction),
			"mode":            compressorBindingRows(band.Mode),
		})
	}
	aux := make([]map[string]any, 0, len(model.Auxiliary))
	for _, stage := range model.Auxiliary {
		aux = append(aux, map[string]any{"kind": stage.Kind, "param_ids": stage.ParamIDs})
	}
	return map[string]any{
		"schema_version":   multibandTopologySchema,
		"classification":   model.Classification,
		"mapping_source":   "generic_structural",
		"confidence":       model.Confidence,
		"control_topology": map[string]any{"schema_version": multibandTopologySchema, "generation": model.Generation},
		"filterbank":       map[string]any{"crossovers": compressorBindingRows(model.Crossovers), "crossover_count": len(model.Crossovers), "ordered": true},
		"band_cells":       bands,
		"shared_controls":  compressorBindingRows(model.SharedControls),
		"auxiliary_stages": aux,
	}, ""
}

func classifyMultibandParameter(param ParameterInfo) multibandCandidate {
	text := strings.ToLower(strings.TrimSpace(parameterDisplayName(param) + " " + param.RawName + " " + param.Alias + " " + param.NormalizedRole))
	c := multibandCandidate{Param: param}
	if multibandContains(text, "sidechain", "side chain", "sc eq", "sc filter", "detector") {
		return c
	}
	bandKey, bandIndex := multibandBandKey(text)
	if (multibandContains(text, "crossover", "cross over", "x-over", "xover") || multibandSharedSplitFrequency(text)) && !multibandHasToken(text, "q") {
		c.Role, c.Section, c.Evidence = "crossover", "frequency", 3
		c.BandKey, c.BandIdx = bandKey, bandIndex
		c.LocalCrossover = multibandBandToken.MatchString(text)
		return c
	}
	c.BandKey, c.BandIdx = bandKey, bandIndex
	switch {
	case multibandHasToken(text, "threshold") || multibandHasToken(text, "thresh") || multibandContains(text, "peak reduction", "compression amount", "range"):
		c.Role, c.Section, c.Evidence = multibandOperatingRole(text), "operating_point", 3
	case multibandHasToken(text, "ratio") || multibandHasToken(text, "knee"):
		c.Role, c.Section, c.Evidence = "ratio", "transfer", 3
	case multibandHasToken(text, "attack"):
		c.Role, c.Section, c.Evidence = "attack", "timing", 2
	case multibandHasToken(text, "release") || multibandHasToken(text, "recovery"):
		c.Role, c.Section, c.Evidence = "release", "timing", 2
	case multibandHasToken(text, "hold") || multibandHasToken(text, "lookahead"):
		c.Role, c.Section, c.Evidence = multibandTimingRole(text), "timing", 1
	case multibandHasToken(text, "gain") || multibandHasToken(text, "makeup") || multibandHasToken(text, "range"):
		c.Role, c.Section, c.Evidence = multibandGainRole(text), "gain_action", 2
	case multibandHasToken(text, "bypass") || multibandHasToken(text, "enable") || multibandHasToken(text, "solo"):
		c.Role, c.Section, c.Evidence = "band_mode", "mode", 1
	}
	return c
}

func multibandSharedSplitFrequency(text string) bool {
	if !multibandHasToken(text, "freq") && !multibandHasToken(text, "frequency") {
		return false
	}
	if multibandBandToken.MatchString(text) {
		return false
	}
	compact := strings.NewReplacer(" ", "", "-", "", "_", "", "/", "").Replace(text)
	return strings.Contains(compact, "lowmidfreq") || strings.Contains(compact, "midlowfreq") ||
		strings.Contains(compact, "midhighfreq") || strings.Contains(compact, "highmidfreq")
}

func multibandOperatingRole(text string) string {
	if multibandContains(text, "peak reduction", "compression amount") {
		return "reduction_amount"
	}
	if multibandHasToken(text, "range") {
		return "range"
	}
	return "threshold"
}
func multibandGainRole(text string) string {
	if multibandHasToken(text, "range") {
		return "range"
	}
	if multibandContains(text, "makeup") {
		return "makeup_gain"
	}
	return "gain"
}
func multibandTimingRole(text string) string {
	if multibandHasToken(text, "lookahead") {
		return "lookahead"
	}
	return "hold"
}
func multibandBandKey(text string) (string, int) {
	if strings.Contains(text, "sidechain") || strings.Contains(text, "sc eq") {
		return "", 0
	}
	if m := multibandBandToken.FindStringSubmatch(text); len(m) == 2 {
		var index int
		fmt.Sscanf(m[1], "%d", &index)
		return fmt.Sprintf("band_%d", index), index
	}
	if m := multibandNamedBandToken.FindStringSubmatch(text); len(m) == 2 {
		key := strings.ReplaceAll(m[1], "-", "_")
		return key, map[string]int{"low": 1, "low_mid": 2, "mid": 3, "high_mid": 4, "high": 5}[key]
	}
	if m := multibandTrailingIndex.FindStringSubmatch(text); len(m) == 2 {
		var index int
		fmt.Sscanf(m[1], "%d", &index)
		if index > 0 {
			return fmt.Sprintf("band_%d", index), index
		}
	}
	return "", 0
}
func buildMultibandBands(rows map[string][]multibandCandidate) []MultibandBand {
	out := make([]MultibandBand, 0, len(rows))
	for key, candidates := range rows {
		band := MultibandBand{Key: key, Index: candidates[0].BandIdx}
		for _, c := range candidates {
			binding := multibandBinding(c, key)
			switch c.Section {
			case "operating_point":
				band.OperatingPoint = append(band.OperatingPoint, binding)
			case "transfer":
				band.Transfer = append(band.Transfer, binding)
			case "timing":
				band.Timing = append(band.Timing, binding)
			case "gain_action":
				band.GainAction = append(band.GainAction, binding)
			case "mode":
				band.Mode = append(band.Mode, binding)
			}
		}
		for _, bindings := range []*[]CompressorBinding{&band.OperatingPoint, &band.Transfer, &band.Timing, &band.GainAction, &band.Mode} {
			sortCompressorBindings(bindings)
		}
		out = append(out, band)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Index != out[j].Index {
			return out[i].Index < out[j].Index
		}
		return out[i].Key < out[j].Key
	})
	return out
}
func multibandBandComplete(band MultibandBand) bool {
	operating := len(band.OperatingPoint) > 0
	curveOrGain := len(band.Transfer) > 0 || len(band.GainAction) > 0
	timing := len(band.Timing) > 0
	return operating && curveOrGain && timing
}
func multibandCrossoversOrdered(crossovers []CompressorBinding) bool {
	previous := -1e300
	for _, c := range crossovers {
		v, ok := parseCompressorPhysical("frequency", c.CurrentText)
		if !ok || v <= previous {
			return false
		}
		previous = v
	}
	return true
}

func multibandCrossoverOrdinal(text string) int {
	text = strings.ToLower(text)
	if m := multibandTrailingIndex.FindStringSubmatch(text); len(m) == 2 {
		var index int
		fmt.Sscanf(m[1], "%d", &index)
		if index > 0 {
			return index
		}
	}
	compact := strings.NewReplacer(" ", "", "-", "", "_", "", "/", "").Replace(text)
	for _, row := range []struct {
		token string
		index int
	}{{"lowmid", 20}, {"lomid", 20}, {"midlow", 20}, {"midhigh", 40}, {"himid", 40}, {"highmid", 40}} {
		if strings.Contains(compact, row.token) {
			return row.index
		}
	}
	for _, row := range []struct {
		token string
		index int
	}{{"low", 10}, {"mid", 30}, {"high", 50}} {
		if multibandHasToken(text, row.token) {
			return row.index
		}
	}
	return 1000000
}
func multibandBinding(c multibandCandidate, stage string) CompressorBinding {
	param := c.Param
	binding := CompressorBinding{ParamID: param.ID, Name: parameterDisplayName(param), Role: c.Role, Channel: "shared", CompressionStage: stage,
		CurrentText: param.ValueText, CurrentNormalized: anyFloat(param.NormalizedValue), Domain: param.DisplayDomainCandidate,
		Curve: compressorCurveFromProbe(param, c.Role), Reachable: compressorReachableValues(param, c.Role), PhysicalUnit: compressorBindingPhysicalUnit(param, c.Role, param.DisplayDomainCandidate)}
	if physical, ok := parseCompressorPhysical(c.Role, param.ValueText); ok {
		binding.CurrentPhysical = &physical
	}
	return binding
}
func multibandAuxiliaryKind(param ParameterInfo) string {
	text := strings.ToLower(strings.TrimSpace(parameterDisplayName(param) + " " + param.RawName + " " + param.Alias + " " + param.NormalizedRole))
	switch {
	case multibandContains(text, "dynamic eq", "dynamic equalizer", "eq dyn", "eq threshold") && !multibandContains(text, "band threshold"):
		return "dynamic_eq"
	case multibandContains(text, "de-esser", "de esser", "deesser", "deess", "s-reduction", "s reduction"):
		return "de_esser"
	case multibandContains(text, "maximizer", "maximiser"):
		return "maximizer"
	case multibandContains(text, "limiter", "limiting", "ceiling", "true peak"):
		return "limiter"
	case multibandContains(text, "spectral dynamics", "spectral compression", "spectral limiter"):
		return "spectral_dynamics"
	case multibandContains(text, "clipper", "soft clip", "clipping"):
		return "clipper"
	}
	return ""
}
func multibandContains(text string, values ...string) bool {
	for _, value := range values {
		if strings.Contains(text, value) {
			return true
		}
	}
	return false
}
func multibandHasToken(text, token string) bool {
	return regexp.MustCompile(`(?i)(?:^|[^a-z0-9])` + regexp.QuoteMeta(token) + `(?:[^a-z0-9]|$)`).MatchString(text)
}
func multibandTopologyGeneration(model *MultibandModel) string {
	type row struct{ ID, Role, Section, Stage string }
	rows := []row{}
	for _, c := range model.Crossovers {
		rows = append(rows, row{c.ParamID, c.Role, "frequency", c.CompressionStage})
	}
	for _, b := range model.Bands {
		for section, bindings := range map[string][]CompressorBinding{"operating_point": b.OperatingPoint, "transfer": b.Transfer, "timing": b.Timing, "gain_action": b.GainAction, "mode": b.Mode} {
			for _, c := range bindings {
				rows = append(rows, row{c.ParamID, c.Role, section, b.Key})
			}
		}
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Stage != rows[j].Stage {
			return rows[i].Stage < rows[j].Stage
		}
		if rows[i].Section != rows[j].Section {
			return rows[i].Section < rows[j].Section
		}
		return rows[i].ID < rows[j].ID
	})
	b, _ := json.Marshal(map[string]any{"schema": multibandTopologySchema, "classification": model.Classification, "rows": rows, "crossover_count": len(model.Crossovers), "band_count": len(model.Bands)})
	sum := sha256.Sum256(b)
	return "mbt1_" + hex.EncodeToString(sum[:12])
}
