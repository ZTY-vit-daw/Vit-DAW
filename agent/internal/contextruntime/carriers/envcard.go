package carriers

// L3 环境实例载体（CONTEXT_LAYERING_V1_DESIGN §2.3）：<workspace>/
// env_instance.v1.json，机器级、不跨设备同步（D14 明文）。InstanceID=
// 指纹哈希（分量明文 map 的 canonical 序 + pluginsemantics revision）；
// 指纹失配=旧卡 superseded 留档+新卡建立+env_changed 断裂，客观经验
// 蒸馏行随迁但降权（verified→downweighted；retired 留痕不复活）——
// 降权重建非静默沿用（T-C2）。
//
// 索引归并边界（§2.3 要点 1）：pluginsemantics 语义索引保持既有职责，
// 实例卡只持 revision 指纹做失配检测，不复制语义内容——两套真相禁止。
// 指纹分量的采集（OS/音频设备/块长采样率/插件清单）归接线卡（IMPL-D）；
// 本包只定义分量 map 与指纹算法。

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// EnvFilename 是 L3 载体文件名（工作区根下，.local 同构不跨设备）。
const EnvFilename = "env_instance.v1.json"

// DistilledNotes Status 封闭枚举（§2.3 要点 3：verified|downweighted|retired）。
const (
	EnvNoteVerified     = "verified"
	EnvNoteDownweighted = "downweighted"
	EnvNoteRetired      = "retired"
)

// EnvNote 是挂环境键的客观经验蒸馏行（D14 表第二行的前缀面；失配降权→
// 复验退役留痕的协议逻辑归 memory 收尾段，本卡只定 Status 语义）。
type EnvNote struct {
	NoteID       string    `json:"note_id"`
	Statement    string    `json:"statement"`
	EvidenceRefs []string  `json:"evidence_refs,omitempty"`
	VerifiedAt   time.Time `json:"verified_at"`
	Status       string    `json:"status"` // verified | downweighted | retired
}

// EnvInstanceCard 是机器+版本指纹实例卡（§2.3 设计态签名）。
type EnvInstanceCard struct {
	InstanceID        string            `json:"instance_id"`             // 指纹哈希
	SupersededBy      string            `json:"superseded_by,omitempty"` // 失配重建时旧卡留档链接
	CreatedAt         time.Time         `json:"created_at"`
	Components        map[string]string `json:"components"`                    // 指纹分量明文（可读性）
	PluginSemanticRev string            `json:"plugin_semantic_rev,omitempty"` // 语义索引 revision 指纹
	CalibrationRefs   []string          `json:"calibration_refs,omitempty"`    // PORT-C2 校准链 vit:// opaque 引用
	DistilledNotes    []EnvNote         `json:"distilled_notes,omitempty"`
}

// EnvDoc 是 env_instance.v1.json 的整档格式（卡片序列，append：旧卡不删）。
type EnvDoc struct {
	SchemaVersion string            `json:"schema_version"` // env_instance.v1
	Cards         []EnvInstanceCard `json:"cards"`
}

type envState struct {
	doc      *EnvDoc
	current  *EnvInstanceCard
	state    string // rendered | absent | corrupt
	warnings []string
	err      error
}

// ComputeEnvInstanceID 由指纹分量明文 map 与语义索引 revision 计算
// InstanceID：分量按 key 排序 canonical 化（"k=v" 行），与 revision 以
// \x00 相接后 sha256。确定性：同分量同 ID（T-C5 前提）。
func ComputeEnvInstanceID(components map[string]string, pluginSemanticRev string) string {
	keys := make([]string, 0, len(components))
	for key := range components {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	lines := make([]string, 0, len(keys))
	for _, key := range keys {
		lines = append(lines, key+"="+components[key])
	}
	payload := strings.Join(lines, "\n") + "\x00" + pluginSemanticRev
	sum := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(sum[:])
}

// LoadEnvCard 装载实例卡档：缺文件=absent；损坏=corrupt；当前卡=卡片
// 序列末尾未被 superseded 的卡。条目级（蒸馏行 Status）封闭枚举越界=
// 该行拒载+WARN（fail-closed）。
func LoadEnvCard(workspaceDir string) envState {
	path := filepath.Join(workspaceDir, EnvFilename)
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return envState{state: "absent"}
		}
		return envState{state: "corrupt", err: fmt.Errorf("env instance unreadable: %w", err)}
	}
	var doc EnvDoc
	if err := json.Unmarshal(raw, &doc); err != nil {
		return envState{state: "corrupt", err: fmt.Errorf("env instance unparseable: %w", err)}
	}
	if doc.SchemaVersion != "env_instance.v1" {
		return envState{state: "corrupt", err: fmt.Errorf("env instance schema_version %q != env_instance.v1", doc.SchemaVersion)}
	}
	state := envState{doc: &doc, state: "rendered"}
	for cardIndex := range doc.Cards {
		card := &doc.Cards[cardIndex]
		kept := make([]EnvNote, 0, len(card.DistilledNotes))
		for noteIndex, note := range card.DistilledNotes {
			switch note.Status {
			case EnvNoteVerified, EnvNoteDownweighted, EnvNoteRetired:
				kept = append(kept, note)
			default:
				state.warnings = append(state.warnings,
					fmt.Sprintf("env card %s note #%d (%s) refused: status %q outside closed enum {verified, downweighted, retired}",
						shortID(card.InstanceID), noteIndex+1, note.NoteID, note.Status))
			}
		}
		card.DistilledNotes = kept
	}
	// 当前卡=最后一张未被 superseded 的卡（append 序）。
	for index := len(doc.Cards) - 1; index >= 0; index-- {
		if doc.Cards[index].SupersededBy == "" {
			state.current = &doc.Cards[index]
			break
		}
	}
	return state
}

// shortID 取指纹前 12 字符用于日志/警示可读性。
func shortID(id string) string {
	if len(id) > 12 {
		return id[:12]
	}
	return id
}

// EnsureEnvCard 校验/构建当前实例卡（会话开始时调用，§2.3 要点 2）：
//   - 无卡：建立新卡（changed=false——建立不是失配）；
//   - 有卡且指纹匹配：返回当前卡（changed=false，会话内恒定=cache 价值）；
//   - 有卡且指纹失配：旧卡 superseded 留档+新卡建立，蒸馏行随迁降权
//     （verified→downweighted；downweighted/retired 原样），changed=true
//     +WARN（非静默重建，T-C2）。
func EnsureEnvCard(workspaceDir string, components map[string]string, pluginSemanticRev string, now time.Time) (EnvInstanceCard, bool, []string, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	liveID := ComputeEnvInstanceID(components, pluginSemanticRev)
	state := LoadEnvCard(workspaceDir)
	if state.state == "corrupt" {
		return EnvInstanceCard{}, false, state.warnings, state.err
	}
	if state.state == "absent" || state.current == nil {
		card := EnvInstanceCard{
			InstanceID:        liveID,
			CreatedAt:         now,
			Components:        components,
			PluginSemanticRev: pluginSemanticRev,
		}
		doc := state.doc
		if doc == nil {
			doc = &EnvDoc{SchemaVersion: "env_instance.v1"}
		}
		doc.Cards = append(doc.Cards, card)
		if err := SaveEnvCard(workspaceDir, doc); err != nil {
			return EnvInstanceCard{}, false, nil, err
		}
		return card, false, nil, nil
	}
	current := *state.current
	if current.InstanceID == liveID {
		return current, false, nil, nil
	}
	newCard := EnvInstanceCard{
		InstanceID:        liveID,
		CreatedAt:         now,
		Components:        components,
		PluginSemanticRev: pluginSemanticRev,
		CalibrationRefs:   current.CalibrationRefs,
		DistilledNotes:    carryNotesDownweighted(current.DistilledNotes),
	}
	state.current.SupersededBy = liveID
	state.doc.Cards = append(state.doc.Cards, newCard)
	if err := SaveEnvCard(workspaceDir, state.doc); err != nil {
		return EnvInstanceCard{}, false, nil, err
	}
	carried := len(newCard.DistilledNotes)
	warning := fmt.Sprintf("env fingerprint mismatch: card %s superseded by %s (rebuild; %d distilled note(s) carried over downweighted, not silently reused)",
		shortID(current.InstanceID), shortID(liveID), carried)
	return newCard, true, []string{warning}, nil
}

// carryNotesDownweighted 迁移旧卡蒸馏行：verified→downweighted（降权）；
// downweighted/retired 原样留痕（退役不复活）。
func carryNotesDownweighted(notes []EnvNote) []EnvNote {
	if len(notes) == 0 {
		return nil
	}
	out := make([]EnvNote, len(notes))
	copy(out, notes)
	for index := range out {
		if out[index].Status == EnvNoteVerified {
			out[index].Status = EnvNoteDownweighted
		}
	}
	return out
}

// RenderEnvCard 渲染 L3 前缀段内容（当前卡；确定性：分量按 key 排序、
// 蒸馏行按 NoteID 排序）。
func RenderEnvCard(card *EnvInstanceCard) string {
	if card == nil {
		return ""
	}
	keys := make([]string, 0, len(card.Components))
	for key := range card.Components {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	comps := make([]string, 0, len(keys))
	for _, key := range keys {
		comps = append(comps, key+"="+card.Components[key])
	}
	lines := []string{
		"Environment instance " + card.InstanceID,
		"components: " + strings.Join(comps, "; "),
	}
	if card.PluginSemanticRev != "" {
		lines = append(lines, "plugin_semantic_rev: "+card.PluginSemanticRev)
	}
	if len(card.CalibrationRefs) > 0 {
		lines = append(lines, "calibration_refs: "+strings.Join(card.CalibrationRefs, ", "))
	}
	notes := append([]EnvNote(nil), card.DistilledNotes...)
	sort.Slice(notes, func(i, j int) bool { return notes[i].NoteID < notes[j].NoteID })
	for _, note := range notes {
		lines = append(lines, fmt.Sprintf("%s [%s] %s", note.NoteID, note.Status, note.Statement))
	}
	return strings.Join(lines, "\n")
}

// EnvCacheKey 是 L3 渲染输入身份（实例指纹，§3.2 归因轨）。
func EnvCacheKey(card *EnvInstanceCard) string {
	if card == nil {
		return ""
	}
	return "env:" + card.InstanceID
}

// SaveEnvCard 写档（env_instance.v1.json；卡片序列 append 倾向——旧卡
// 不删，superseded 链在档内留痕）。
func SaveEnvCard(workspaceDir string, doc *EnvDoc) error {
	if doc.SchemaVersion == "" {
		doc.SchemaVersion = "env_instance.v1"
	}
	raw, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(workspaceDir, EnvFilename), raw, 0o644)
}
