package carriers

// L4 工程账本载体（CONTEXT_LAYERING_V1_DESIGN §2.4）：<projectpkg>/ledger/
// project_ledger.v1.jsonl——一行一条、文件永不改写（append-only 字面执行）。
// 撤销/修正=追加带 Supersedes 的新条目；前缀渲染顺序串联全部条目（含被
// 撤销的），"当前视图"由读侧归并。链式 PrevHash 是 append-only 证明：
// 每条 PrevHash=前条 canonical JSON sha256，中段篡改被校验器检出（T-C3）。
//
// 写入权限（§2.4 要点 4）：v1=会话侧结构化写入器；不经 execution_memory
// 继承链（22 键 allow-list 不扩，§8.3）。Kind 封闭枚举七值 fail-closed：
// 未知 Kind 拒载该条目+WARN，不静默丢弃（§8.3）。
//
// genesis 头部段（§2.4 要点 2）：工程打开时从投影层只读渲染一次（TOM
// 概览/总线拓扑/交付目标），以 Kind=topology_delta 条目入账。投影摘要
// 的采集归接线卡（IMPL-D，消费 LLMContext/摘要行不展开 raw package，
// §8.2）；本包定义 GenesisFacts 输入契约与入账机制。

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"strings"
	"time"
)

// LedgerRelPath 是 L4 载体相对工程包的路径。
const LedgerRelPath = "ledger/project_ledger.v1.jsonl"

// Kind 封闭枚举七值（§2.4 设计态 + 卡面"Kind 封闭枚举七值"）。
const (
	LedgerKindDecision              = "decision"
	LedgerKindObservationConclusion = "observation_conclusion"
	LedgerKindIdentityConfirmation  = "identity_confirmation"
	LedgerKindGoal                  = "goal"
	LedgerKindConstraint            = "constraint"
	LedgerKindTopologyDelta         = "topology_delta"
	LedgerKindUserOverride          = "user_override"
)

// LedgerKindValid 报告 Kind 是否在封闭枚举内（fail-closed 判据）。
func LedgerKindValid(kind string) bool {
	switch kind {
	case LedgerKindDecision, LedgerKindObservationConclusion, LedgerKindIdentityConfirmation,
		LedgerKindGoal, LedgerKindConstraint, LedgerKindTopologyDelta, LedgerKindUserOverride:
		return true
	default:
		return false
	}
}

// LedgerEntry 是一条账本条目（九字段，§2.4 设计态签名）。
type LedgerEntry struct {
	EntryID      int64     `json:"entry_id"`                // 单调递增（genesis 起 1）
	PrevHash     string    `json:"prev_hash"`               // 前条 canonical sha256；首条为空
	Kind         string    `json:"kind"`                    // 封闭枚举七值
	Phase        string    `json:"phase,omitempty"`         // 决策相位注记（披露语义，不复活相位门）
	Statement    string    `json:"statement"`               // 结论级陈述
	EvidenceRefs []string  `json:"evidence_refs,omitempty"` // vit:// 文法；外部工件 opaque 透传
	Supersedes   int64     `json:"supersedes,omitempty"`    // 0=无；撤销也是追加
	CreatedAt    time.Time `json:"created_at"`
}

// LedgerError 带条目定位的校验失败（区分失败类型：链断裂/未知 Kind/
// ID 非单调/解析失败——红测断言面）。
type LedgerError struct {
	EntryIndex int // 1-based 条目序
	EntryID    int64
	Kind       string // parse_error | unknown_kind | chain_mismatch | non_monotonic_id | empty_statement
	Detail     string
}

func (e *LedgerError) Error() string {
	return fmt.Sprintf("ledger entry #%d (id=%d) %s: %s", e.EntryIndex, e.EntryID, e.Kind, e.Detail)
}

// CanonicalEntryJSON 是条目的 canonical 序列化（struct 字段序+json 包
// 确定性；链式 hash 与写入行共用同一字节形态——读侧重算即比对）。
func CanonicalEntryJSON(entry LedgerEntry) ([]byte, error) {
	return json.Marshal(entry)
}

// EntryHash 返回条目 canonical sha256（链式证明的"前条哈希"值）。
func EntryHash(entry LedgerEntry) string {
	raw, err := CanonicalEntryJSON(entry)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func ledgerPath(projectDir string) string {
	return filepath.Join(projectDir, filepath.FromSlash(LedgerRelPath))
}

// ResolveProjectDir 把工程路径解析成账本宿主目录：真栈工程 project_path 是
// .vit 文件（sidecar 目录 .vit_agent/.vit_history 与其同级，对齐
// projectstore.Resolve 的 ProjectDir 语义=文件父目录），账本按
// <projectDir>/ledger/ 约定落同一父目录；project_path 本身是目录（草稿/
// 测试面）时原样使用。裸 .vit 路径直喂 ledgerPath 会让读侧永 absent、写侧
// MkdirAll 落在文件之下必然失败。空路径原样返回（无工程面=无账本宿主，
// 不落 cwd 相对面）。读侧探测（os.Stat），纯函数无副作用。
func ResolveProjectDir(projectPath string) string {
	if projectPath == "" {
		return ""
	}
	if info, err := os.Stat(projectPath); err == nil && info.IsDir() {
		return projectPath
	}
	return filepath.Dir(projectPath)
}

// AppendLedgerEntry 追加一条条目：EntryID=链尾+1，PrevHash=链尾哈希，
// 写入行=canonical JSON+"\n"（只追加，永不改写前缀）。Kind 越界=
// fail-closed 拒写。
func AppendLedgerEntry(projectDir string, kind, phase, statement string, evidenceRefs []string, supersedes int64, now time.Time) (LedgerEntry, error) {
	if !LedgerKindValid(kind) {
		return LedgerEntry{}, fmt.Errorf("ledger kind %q outside closed enum (7 values); entry refused", kind)
	}
	if strings.TrimSpace(statement) == "" {
		return LedgerEntry{}, fmt.Errorf("ledger entry statement must not be empty")
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	entries, err := ReadLedger(projectDir)
	if err != nil {
		return LedgerEntry{}, err
	}
	entry := LedgerEntry{
		EntryID:      int64(len(entries)) + 1,
		PrevHash:     "",
		Kind:         kind,
		Phase:        phase,
		Statement:    statement,
		EvidenceRefs: evidenceRefs,
		Supersedes:   supersedes,
		CreatedAt:    now,
	}
	if len(entries) > 0 {
		entry.PrevHash = EntryHash(entries[len(entries)-1])
	}
	line, err := CanonicalEntryJSON(entry)
	if err != nil {
		return LedgerEntry{}, err
	}
	path := ledgerPath(projectDir)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return LedgerEntry{}, err
	}
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return LedgerEntry{}, err
	}
	defer file.Close()
	if _, err := file.Write(append(line, '\n')); err != nil {
		return LedgerEntry{}, err
	}
	return entry, nil
}

// ReadLedger 读取并校验整本账本：缺文件=空层（nil, nil）；任何违规=
// *LedgerError（封闭枚举越界/链断裂/ID 非单调/解析失败），fail-closed
// 不静默跳过。校验器即读取器（篡改检测面 T-C3）。
func ReadLedger(projectDir string) ([]LedgerEntry, error) {
	path := ledgerPath(projectDir)
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer file.Close()
	var entries []LedgerEntry
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	index := 0
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		index++
		var entry LedgerEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			return nil, &LedgerError{EntryIndex: index, Kind: "parse_error", Detail: err.Error()}
		}
		if !LedgerKindValid(entry.Kind) {
			return nil, &LedgerError{
				EntryIndex: index, EntryID: entry.EntryID, Kind: "unknown_kind",
				Detail: fmt.Sprintf("kind %q outside closed enum; entry refused (fail-closed, no silent drop)", entry.Kind),
			}
		}
		if strings.TrimSpace(entry.Statement) == "" {
			return nil, &LedgerError{EntryIndex: index, EntryID: entry.EntryID, Kind: "empty_statement", Detail: "statement must not be empty"}
		}
		wantID := int64(len(entries)) + 1
		if entry.EntryID != wantID {
			return nil, &LedgerError{
				EntryIndex: index, EntryID: entry.EntryID, Kind: "non_monotonic_id",
				Detail: fmt.Sprintf("entry_id %d, want %d (monotonic from 1)", entry.EntryID, wantID),
			}
		}
		var wantPrev string
		if len(entries) > 0 {
			wantPrev = EntryHash(entries[len(entries)-1])
		}
		if entry.PrevHash != wantPrev {
			return nil, &LedgerError{
				EntryIndex: index, EntryID: entry.EntryID, Kind: "chain_mismatch",
				Detail: fmt.Sprintf("prev_hash %s, want %s (chain broken — prefix rewritten or tampered)", shortHash(entry.PrevHash), shortHash(wantPrev)),
			}
		}
		entries = append(entries, entry)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return entries, nil
}

func shortHash(hash string) string {
	if len(hash) > 12 {
		return hash[:12] + "…"
	}
	if hash == "" {
		return "(genesis)"
	}
	return hash
}

// GenesisFacts 是工程打开时从投影层只读渲染的头部事实（§2.4 要点 2；
// 消费 LLMContext/摘要行，不展开 raw package——§8.2）。采集归接线卡。
type GenesisFacts struct {
	TracksSummary   []string // TOM 概览：轨道清单摘要行
	BusTopology     string   // 总线拓扑：路由树文本化
	DeliveryTargets []string // 交付目标：RLM 目标/交付 profile 引用行
}

// AppendGenesis 在空账本上写入 genesis 头部段（每事实组一条
// topology_delta 条目，Phase=genesis）。账本非空=幂等跳过（genesis
// 只渲染一次；后续拓扑变化走增量 delta 条目，头部永不重写）。
func AppendGenesis(projectDir string, facts GenesisFacts, now time.Time) (int, error) {
	entries, err := ReadLedger(projectDir)
	if err != nil {
		return 0, err
	}
	if len(entries) > 0 {
		return 0, nil
	}
	appended := 0
	write := func(statement string) {
		if statement == "" {
			return
		}
		if _, err := AppendLedgerEntry(projectDir, LedgerKindTopologyDelta, "genesis", statement, nil, 0, now); err != nil {
			return
		}
		appended++
	}
	if len(facts.TracksSummary) > 0 {
		write("tom_overview: " + strings.Join(facts.TracksSummary, " | "))
	}
	write(prefixIfSet("bus_topology: ", facts.BusTopology))
	if len(facts.DeliveryTargets) > 0 {
		write("delivery_targets: " + strings.Join(facts.DeliveryTargets, " | "))
	}
	return appended, nil
}

func prefixIfSet(prefix, value string) string {
	if strings.TrimSpace(value) == "" {
		return ""
	}
	return prefix + value
}

// RenderLedger 渲染 L4 前缀段内容：全部条目（含被撤销的）按序串联，
// 撤销语义由最新条目承载（§2.4 要点 1）。确定性：条目序即 EntryID 序。
func RenderLedger(entries []LedgerEntry) string {
	if len(entries) == 0 {
		return ""
	}
	rows := make([]string, 0, len(entries))
	for _, entry := range entries {
		row := fmt.Sprintf("#%d [%s]", entry.EntryID, entry.Kind)
		if entry.Phase != "" {
			row += " (" + entry.Phase + ")"
		}
		row += " " + entry.Statement
		if entry.Supersedes > 0 {
			row += fmt.Sprintf(" [supersedes #%d]", entry.Supersedes)
		}
		if len(entry.EvidenceRefs) > 0 {
			row += " refs=" + strings.Join(entry.EvidenceRefs, ",")
		}
		rows = append(rows, row)
	}
	return "Project ledger (append-only):\n" + strings.Join(rows, "\n")
}

// LedgerCacheKey 是 L4 渲染输入身份（链尾 hash+条目数，§3.2 归因轨）。
func LedgerCacheKey(entries []LedgerEntry) string {
	if len(entries) == 0 {
		return ""
	}
	tail := entries[len(entries)-1]
	return fmt.Sprintf("ledger:%s:%d", EntryHash(tail), len(entries))
}
