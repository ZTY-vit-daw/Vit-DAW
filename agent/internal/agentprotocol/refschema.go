package agentprotocol

// RED-STUB for REFSCHEMA-L0-1: declarations only, bodies intentionally
// non-functional so the table-driven tests in refschema_test.go fail first.
// The real implementation replaces this file in the green commit.

const RefSchemePrefix = "vit://"

type TimeWindow struct {
	AllTime     bool
	SampleStart int64
	SampleEnd   int64
}

type Ref struct {
	Kind       string
	ScopeKind  string
	ScopeValue string
	Window     *TimeWindow
	Snapshot   string
	Hash       string
}

type RefState string

const (
	RefStateParsed RefState = "parsed"
	RefStateLegacy RefState = "legacy"
	RefStateOpaque RefState = "opaque"
)

const (
	RefFamilyProjectionContentID = "projection_content_id"
	RefFamilySnapshotRequestID   = "snapshot_request_id"
)

const (
	RefSlotHash     = "hash"
	RefSlotSnapshot = "snapshot"
)

type LegacyPrefixEntry struct {
	LegacyPrefix string
	Family       string
	TargetKind   string
	Slot         string
	Anchor       string
}

type LegacyTranslation struct {
	LegacyPrefix string
	Family       string
	TargetKind   string
	Slot         string
	Value        string
}

type ParsedRef struct {
	State  RefState
	Raw    string
	Ref    *Ref
	Legacy *LegacyTranslation
}

var RefSchemaWarnLogger func(line string)

func (r Ref) Validate() error { return nil }

func FormatRef(r Ref) (string, error) { return "", nil }

func ParseRef(raw string) (ParsedRef, error) { return ParsedRef{}, nil }

func LegacyPrefixRegistry() []LegacyPrefixEntry { return nil }

func RegisteredRefKinds() []string { return nil }

func OpaqueWarnCounts() map[string]int { return nil }

func ResetOpaqueWarnState() {}
