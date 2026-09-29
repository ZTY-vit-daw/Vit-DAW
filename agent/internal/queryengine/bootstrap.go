package queryengine

// bootstrap.go — v0 bootstrap 适配器（QUERY_ENGINE §5.2/§9 IMPL-B）。
//
// 职责：L1-2 物化层本体之外的起步腿——对四产物族落盘产物做只读扫描，以
// MaterializedStore 契约面供给引擎；增量腿用"文件指纹 + 定时轮询"模拟
// Subscribe（秒级粒度，仅够冒测；v1 换 L1-2 事件驱动变更流）。v0 不触发
// 任何 finalize/现算：扫描不到即如实少行（不伪造、不重算、不静默补零）。
//
// 四产物族（§5.2 表）与行化对接面（materialize 导出符号，只读消费）：
//   - 观察票 JSON     <AgentRoot>/<uuid>/observations/<obs_id>.json
//       dom ← materialize.DOMRowFromProjection（内容身份口径，snapshot=current）
//       fxm ← materialize.FXMRowsFromObservation（实例身份，snapshot=obs_id）
//       com ← materialize.COMRowsFromObservation（实例身份，snapshot=obs_id）
//   - 声学包          <AcousticStore>（acousticpackage.Store.Read 只读）
//       acp ← materialize.ACPAdapter().Build（M4 rev 指纹进 payload）
//   - COM paired 工件 <COMEvidenceDir>/<pairID>/<pairID>.json
//       dad.compressor_dual_tap 行（工件路径句柄，expand 天然指向）
//   - 特征快照行      <AgentRoot>/<uuid>/mixboard/mixboard_feature_snapshot.json
//       dad.l3（L3 特征族行按轨聚合）+ dad.l2_render_probe（probe 帧）
//
// 覆盖边界（如实申报，v0 不构成阻塞）：观察票中的 mom/tim 投影、capability
// preflight 的 rlm、导入回执的 tom/epm——materialize 未导出对应行化符号，
// 本适配器不在引擎侧重定义行化口径（接口冻结红线；扩展走 materialize 导出
// 面的后续卡）。
//
// freshness 语义：全部扫描行标注 material_reuse（存量行——bootstrap 无失效
// 传播知识，不冒称 current；§5.1 判定全在写侧，读侧零推断）。

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"
	"time"

	"vit-daw-agent/internal/acousticpackage"
	"vit-daw-agent/internal/agentprotocol"
	"vit-daw-agent/internal/com"
	"vit-daw-agent/internal/dom"
	"vit-daw-agent/internal/fxm"
	"vit-daw-agent/internal/materialize"
)

var (
	// ErrUnsupportedSelector at_or_before 未支持（OQ-2：v0 只保"盘面现状"，
	// 历史代回溯归物化层 v1+；不支持必须显式报错，不静默空集）。
	ErrUnsupportedSelector = errors.New("queryengine: bootstrap: snapshot selector unsupported (latest/exact only, OQ-2)")
	// ErrNoMaterializedRow Resolve 命中未知坐标（miss 如实上报，不静默空）。
	ErrNoMaterializedRow = errors.New("queryengine: bootstrap: no materialized row at ref coordinate")
	// ErrBootstrapScan 扫描面错误（产物文件存在但不可读/不可解析——显式上报，
	// 与"产物缺席=空行集"区分）。
	ErrBootstrapScan = errors.New("queryengine: bootstrap scan error")
)

const (
	// bootstrapSubscribeBuffer 每订阅者缓冲（慢消费者丢弃，写侧即轮询器不
	// 阻塞——与 materialize §6.2 同款纪律；可靠消费靠重拉 SnapshotView）。
	bootstrapSubscribeBuffer = 128
	// bootstrapDefaultPoll v0 轮询默认粒度（秒级，§5.2——仅够冒测）。
	bootstrapDefaultPoll = time.Second
	// bootstrapFeatureSnapshotName 桥快照文件名（kernel bridge 落盘）。
	bootstrapFeatureSnapshotName = "mixboard_feature_snapshot.json"
)

// BootstrapConfig 四产物族扫描配置。路径缺席（空串或目录不存在）= 该族空集
// （如实 empty+deferred，不报错）；文件存在但损坏 = 扫描错误（显式）。
type BootstrapConfig struct {
	AgentRoot      string        // 含 <uuid>/observations/ 与 <uuid>/mixboard/ 的根（.vit_agent）
	AcousticStore  string        // acoustic_package_status.json 路径
	COMEvidenceDir string        // com_evidence 工件目录（<pairID>/<pairID>.json）
	PollInterval   time.Duration // Subscribe 轮询间隔；<=0 取默认 1s
}

// BootstrapStore v0 bootstrap 适配器：queryengine.MaterializedStore 的只读
// 扫描实现。零值不可用；NewBootstrapStore 构造。
type BootstrapStore struct {
	cfg BootstrapConfig

	mu   sync.Mutex
	last *bootstrapScan // 最近一次成功扫描（指纹 memo：Resolve/轮询复用）
}

// NewBootstrapStore 构造 bootstrap 适配器（不触盘；首次 SnapshotView/Subscribe
// 时才扫描）。
func NewBootstrapStore(cfg BootstrapConfig) *BootstrapStore {
	return &BootstrapStore{cfg: cfg}
}

// bootstrapRow 契约行 + 工件路径句柄（句柄不进契约面——Resolve 现场给出）。
type bootstrapRow struct {
	row    MaterializedRow
	handle string
}

type bootstrapScan struct {
	rows        []bootstrapRow
	fingerprint string
}

// bootstrapFile 是指纹枚举的最小单元（mtime+size——v0 声明的秒级粒度）。
type bootstrapFile struct {
	path    string
	modNano int64
	size    int64
}

// ---- SnapshotView / Resolve / Subscribe（契约三函数）----

// SnapshotView 返回盘面现状全量（latest）或按行 snapshot 段过滤（exact）。
// at_or_before 显式拒绝（OQ-2）。行序=canonical ref 字典序（确定性迭代）。
func (b *BootstrapStore) SnapshotView(ctx context.Context, sel SnapshotSelector) ([]MaterializedRow, error) {
	mode := sel.Mode
	if mode == "" {
		mode = SnapshotLatest
	}
	switch mode {
	case SnapshotLatest:
	case SnapshotExact:
	default:
		return nil, fmt.Errorf("%w: %q", ErrUnsupportedSelector, mode)
	}
	scan, err := b.scanCached(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]MaterializedRow, 0, len(scan.rows))
	for _, r := range scan.rows {
		if mode == SnapshotExact && !bootstrapExactSnapshotMatch(sel, r.row.Ref.Snapshot) {
			continue
		}
		out = append(out, r.row)
	}
	return out, nil
}

func bootstrapExactSnapshotMatch(sel SnapshotSelector, snapshot string) bool {
	if sel.Revision != "" && sel.Revision != snapshot {
		return false
	}
	if sel.ObservationID != "" && sel.ObservationID != snapshot {
		return false
	}
	return true
}

// Resolve 按坐标四段解析（kind/scope/window/snapshot；hash 是值不是键——旧
// hash 引用解析到坐标行，与 materialize Resolve 同语义）。句柄=产物工件
// 路径（未 CAS 化产物的 §5.1.3 兜底形态），ReadAll 直读文件。
func (b *BootstrapStore) Resolve(ctx context.Context, ref agentprotocol.Ref) (ResolvedEvidence, error) {
	if err := ctx.Err(); err != nil {
		return ResolvedEvidence{}, err
	}
	scan, err := b.scanCached(ctx)
	if err != nil {
		return ResolvedEvidence{}, err
	}
	key := bootstrapCoordKey(ref)
	for _, r := range scan.rows {
		if bootstrapCoordKey(r.row.Ref) != key {
			continue
		}
		res := ResolvedEvidence{Handle: r.handle, Freshness: r.row.Freshness}
		if r.handle == "" {
			res.ReadAll = func() ([]byte, error) {
				return nil, errors.New("queryengine: bootstrap: row carries no content handle")
			}
			return res, nil
		}
		if info, err := os.Stat(r.handle); err == nil {
			res.Bytes = info.Size()
		}
		path := r.handle
		res.ReadAll = func() ([]byte, error) { return os.ReadFile(path) }
		return res, nil
	}
	return ResolvedEvidence{}, fmt.Errorf("%w: %s/%s", ErrNoMaterializedRow, ref.Kind, ref.ScopeValue)
}

// Subscribe 轮询模拟变更流（§5.2：文件 mtime+目录列举；秒级粒度）。基线=
// 订阅时盘面；此后每 PollInterval 重扫并 diff：新坐标→added、内容变化→
// replaced、产物消失→marked_stale（消失行"曾经存在"的可见性经事件保留；
// 下一次全量重建如实反映盘面——v0 边界，如实申报）。事件序=坐标字典序。
// 扫描失败轮次跳过（不伪造事件；持续失败由 SnapshotView 显式报错）。
func (b *BootstrapStore) Subscribe(ctx context.Context) (<-chan MaterializedChange, func(), error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	interval := b.cfg.PollInterval
	if interval <= 0 {
		interval = bootstrapDefaultPoll
	}
	baseline, err := b.scanCached(ctx)
	if err != nil {
		return nil, nil, err
	}
	ch := make(chan MaterializedChange, bootstrapSubscribeBuffer)
	done := make(chan struct{})
	var closeOnce sync.Once
	unsubscribe := func() {
		closeOnce.Do(func() { close(done) })
	}
	go func() {
		defer close(ch)
		prev := bootstrapIndexRows(baseline)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-done:
				return
			case <-ticker.C:
			}
			next, err := b.scanCached(ctx)
			if err != nil {
				continue
			}
			indexed := bootstrapIndexRows(next)
			for _, ev := range bootstrapDiffRows(prev, indexed) {
				select {
				case ch <- ev:
				default: // 慢消费者丢弃（§6.2 同款）；可靠消费走 SnapshotView
				}
			}
			prev = indexed
		}
	}()
	return ch, unsubscribe, nil
}

// ---- 扫描 ----

// bootstrapFamilies 指纹枚举的四族文件清单（指纹=全部文件的 path|mtime|size
// 摘要——盘面无变化则复用上次扫描结果）。
type bootstrapFamilies struct {
	obs      []bootstrapFile // 观察票
	features []bootstrapFile // 桥快照
	acoustic *bootstrapFile  // 声学包 store（nil=缺席）
	pairDirs []bootstrapFile // COM paired 工件（<pairID>.json）
}

func (b *BootstrapStore) enumerate() (bootstrapFamilies, string, error) {
	var fam bootstrapFamilies
	abs := func(p string) string { return filepath.Clean(p) }

	if root := strings.TrimSpace(b.cfg.AgentRoot); root != "" {
		obs, err := bootstrapGlobStat(filepath.Join(abs(root), "*", "observations", "*.json"))
		if err != nil {
			return fam, "", err
		}
		fam.obs = obs
		features, err := bootstrapGlobStat(filepath.Join(abs(root), "*", "mixboard", bootstrapFeatureSnapshotName))
		if err != nil {
			return fam, "", err
		}
		fam.features = features
	}
	if path := strings.TrimSpace(b.cfg.AcousticStore); path != "" {
		if f, ok, err := bootstrapStatOne(abs(path)); err != nil {
			return fam, "", err
		} else if ok {
			fam.acoustic = &f
		}
	}
	if dir := strings.TrimSpace(b.cfg.COMEvidenceDir); dir != "" {
		pairs, err := bootstrapGlobStat(filepath.Join(abs(dir), "*", "*.json"))
		if err != nil {
			return fam, "", err
		}
		fam.pairDirs = pairs
	}
	return fam, bootstrapFingerprint(fam), nil
}

func bootstrapGlobStat(pattern string) ([]bootstrapFile, error) {
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("%w: glob %s: %v", ErrBootstrapScan, pattern, err)
	}
	out := make([]bootstrapFile, 0, len(matches))
	for _, path := range matches {
		f, ok, err := bootstrapStatOne(path)
		if err != nil {
			return nil, err
		}
		if ok {
			out = append(out, f)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].path < out[j].path })
	return out, nil
}

func bootstrapStatOne(path string) (bootstrapFile, bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return bootstrapFile{}, false, nil
		}
		return bootstrapFile{}, false, fmt.Errorf("%w: stat %s: %v", ErrBootstrapScan, path, err)
	}
	if info.IsDir() {
		return bootstrapFile{}, false, nil
	}
	return bootstrapFile{path: filepath.Clean(path), modNano: info.ModTime().UnixNano(), size: info.Size()}, true, nil
}

func bootstrapFingerprint(fam bootstrapFamilies) string {
	h := sha256.New()
	for _, group := range [][]bootstrapFile{fam.obs, fam.features, fam.pairDirs} {
		for _, f := range group {
			fmt.Fprintf(h, "%s|%d|%d\n", f.path, f.modNano, f.size)
		}
	}
	if fam.acoustic != nil {
		fmt.Fprintf(h, "%s|%d|%d\n", fam.acoustic.path, fam.acoustic.modNano, fam.acoustic.size)
	}
	return hex.EncodeToString(h.Sum(nil))
}

// scanCached 指纹命中的 memo 扫描：盘面无变化直接复用上次结果（v0 秒级
// 粒度的等价物）；指纹变化则全量重扫。
func (b *BootstrapStore) scanCached(ctx context.Context) (*bootstrapScan, error) {
	fam, fingerprint, err := b.enumerate()
	if err != nil {
		return nil, err
	}
	b.mu.Lock()
	if b.last != nil && b.last.fingerprint == fingerprint {
		cached := b.last
		b.mu.Unlock()
		return cached, nil
	}
	b.mu.Unlock()

	scan, err := b.scanFamilies(ctx, fam, fingerprint)
	if err != nil {
		return nil, err
	}
	b.mu.Lock()
	b.last = scan
	b.mu.Unlock()
	return scan, nil
}

func (b *BootstrapStore) scanFamilies(ctx context.Context, fam bootstrapFamilies, fingerprint string) (*bootstrapScan, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	rows := make([]bootstrapRow, 0, len(fam.obs)*2+len(fam.pairDirs)+8)
	for _, f := range fam.obs {
		family, err := b.scanObservationTicket(f.path)
		if err != nil {
			return nil, err
		}
		rows = append(rows, family...)
	}
	if fam.acoustic != nil {
		family, err := b.scanAcousticStore(fam.acoustic.path)
		if err != nil {
			return nil, err
		}
		rows = append(rows, family...)
	}
	for _, f := range fam.pairDirs {
		family, err := b.scanComPairArtifact(f.path)
		if err != nil {
			return nil, err
		}
		rows = append(rows, family...)
	}
	for _, f := range fam.features {
		family, err := b.scanFeatureSnapshot(f.path)
		if err != nil {
			return nil, err
		}
		rows = append(rows, family...)
	}
	sort.Slice(rows, func(i, j int) bool {
		return canonOrEmpty(rows[i].row.Ref) < canonOrEmpty(rows[j].row.Ref)
	})
	return &bootstrapScan{rows: rows, fingerprint: fingerprint}, nil
}

func canonOrEmpty(ref agentprotocol.Ref) string {
	s, err := agentprotocol.FormatRef(ref)
	if err != nil {
		return "" // 不可规范化行排最前（引擎建索引时按 T4 跳过）
	}
	return s
}

// bootstrapMaterializeRow materialize.Row → 契约行（freshness 统一 material_reuse；
// 句柄回填为产物工件路径——materialize 行化函数不产句柄）。
func bootstrapMaterializeRow(row materialize.Row, handle string) bootstrapRow {
	return bootstrapRow{
		row:    MaterializedRow{Ref: row.Ref, Freshness: FreshnessMaterialReuse, Payload: row.Payload},
		handle: handle,
	}
}

// ---- 族 1：观察票 JSON ----

// bootstrapObservationTicket 只解析 bootstrap 所需字段（不 import mixboard
// 全量包——观察票 schema 权威在 mixboard，此处只读子集）。
type bootstrapObservationTicket struct {
	ObservationID string          `json:"observation_id"`
	TargetRef     bootstrapTarget `json:"target_ref"`
	DOMProjection *dom.Projection `json:"dom_projection"`
	FXMProjection *fxm.Projection `json:"fxm_projection"`
	COMProjection *com.Projection `json:"com_projection"`
}

type bootstrapTarget struct {
	Kind  string `json:"kind"`
	ID    string `json:"id"`
	Label string `json:"label"`
}

func (b *BootstrapStore) scanObservationTicket(path string) ([]bootstrapRow, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("%w: read %s: %v", ErrBootstrapScan, path, err)
	}
	var ticket bootstrapObservationTicket
	if err := json.Unmarshal(data, &ticket); err != nil {
		return nil, fmt.Errorf("%w: parse %s: %v", ErrBootstrapScan, path, err)
	}
	obsID := strings.TrimSpace(ticket.ObservationID)
	if obsID == "" {
		obsID = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	}
	trackID := strings.TrimSpace(ticket.TargetRef.ID)
	if trackID == "" {
		trackID = "unknown"
	}
	rows := []bootstrapRow{}
	// dom：内容身份行（snapshot=current，DOMRowFromProjection 坐标约定——
	// scope_kind 恒 track；非 track 目标的观察票按目标 ID 落 track 坐标）。
	if row, ok := materialize.DOMRowFromProjection(trackID, deref(ticket.DOMProjection)); ok {
		rows = append(rows, bootstrapMaterializeRow(row, path))
	}
	for _, row := range materialize.FXMRowsFromObservation(obsID, ticket.FXMProjection) {
		rows = append(rows, bootstrapMaterializeRow(row, path))
	}
	for _, row := range materialize.COMRowsFromObservation(obsID, ticket.COMProjection) {
		rows = append(rows, bootstrapMaterializeRow(row, path))
	}
	return rows, nil
}

func deref(p *dom.Projection) dom.Projection {
	if p == nil {
		return dom.Projection{}
	}
	return *p
}

// ---- 族 2：声学包 store ----

func (b *BootstrapStore) scanAcousticStore(path string) ([]bootstrapRow, error) {
	snap, err := acousticpackage.NewStore(path).Read()
	if err != nil {
		return nil, fmt.Errorf("%w: acoustic store %s: %v", ErrBootstrapScan, path, err)
	}
	built, err := materialize.ACPAdapter().Build(materialize.DepInputs{AcousticPackages: snap.Packages})
	if err != nil {
		return nil, fmt.Errorf("%w: acp adapter: %v", ErrBootstrapScan, err)
	}
	rows := make([]bootstrapRow, 0, len(built))
	for _, row := range built {
		rows = append(rows, bootstrapMaterializeRow(row, path))
	}
	return rows, nil
}

// ---- 族 3：COM paired 工件 ----

type bootstrapComPair struct {
	PairID         string `json:"pair_id"`
	SchemaVersion  string `json:"schema_version"`
	ProcessorScope struct {
		TrackID          string `json:"track_id"`
		PluginInstanceID string `json:"plugin_instance_id"`
		TopologyClass    string `json:"topology_class"`
		SupportClass     string `json:"support_class"`
	} `json:"processor_scope"`
}

func (b *BootstrapStore) scanComPairArtifact(path string) ([]bootstrapRow, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("%w: read %s: %v", ErrBootstrapScan, path, err)
	}
	var pair bootstrapComPair
	if err := json.Unmarshal(data, &pair); err != nil {
		return nil, fmt.Errorf("%w: parse %s: %v", ErrBootstrapScan, path, err)
	}
	pairID := strings.TrimSpace(pair.PairID)
	if pairID == "" {
		pairID = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	}
	trackID := strings.TrimSpace(pair.ProcessorScope.TrackID)
	if trackID == "" {
		trackID = "pair:" + pairID // 无轨坐标的配对工件：以 pair 身份作 scope（refschema 两段非空）
	}
	payload := map[string]any{
		"schema_version": pair.SchemaVersion,
		"topology_class": pair.ProcessorScope.TopologyClass,
		"support_class":  pair.ProcessorScope.SupportClass,
	}
	if id := strings.TrimSpace(pair.ProcessorScope.PluginInstanceID); id != "" {
		payload["plugin_instance_id"] = id
	}
	// hash=工件文件自身的内容身份（文件字节即 canonical 形态，§3.3 hash 语义）。
	sum := sha256.Sum256(data)
	row := materialize.Row{
		Ref: agentprotocol.Ref{
			Kind: "dad.compressor_dual_tap", ScopeKind: "track", ScopeValue: trackID,
			Window: &agentprotocol.TimeWindow{AllTime: true}, Snapshot: pairID,
			Hash: "sha256:" + hex.EncodeToString(sum[:])[:16],
		},
		Payload: payload,
	}
	return []bootstrapRow{bootstrapMaterializeRow(row, path)}, nil
}

// ---- 族 4：桥快照（特征快照行）----

type bootstrapFeatureSnapshot struct {
	SchemaVersion           string           `json:"schema_version"`
	TrackWaveformEnvelopes  []map[string]any `json:"track_waveform_envelopes"`
	BandEnergySummaries     []map[string]any `json:"band_energy_summaries"`
	StereoRelationSummaries []map[string]any `json:"stereo_relation_summaries"`
	LoudnessSummaries       []map[string]any `json:"loudness_summaries"`
	L2RenderProbes          []map[string]any `json:"l2_render_probes"`
}

// bootstrapL3Sections L3 特征族段 → dad.l3 行的 payload 标签（materialize
// featureTrackIDs 同一四段词表——行坐标口径对齐）。
var bootstrapL3Sections = []struct {
	jsonKey string
	label   string
}{
	{"track_waveform_envelopes", "waveform"},
	{"band_energy_summaries", "band_energy"},
	{"stereo_relation_summaries", "stereo"},
	{"loudness_summaries", "loudness"},
}

func (b *BootstrapStore) scanFeatureSnapshot(path string) ([]bootstrapRow, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("%w: read %s: %v", ErrBootstrapScan, path, err)
	}
	var snap bootstrapFeatureSnapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return nil, fmt.Errorf("%w: parse %s: %v", ErrBootstrapScan, path, err)
	}
	rows := []bootstrapRow{}

	// dad.l3：每轨一行（四段按轨聚合；段缺席即不注记）。
	type l3Track struct {
		sections map[string]map[string]any // label → row
	}
	tracks := map[string]*l3Track{}
	sectionRows := map[string][]map[string]any{
		"waveform":    snap.TrackWaveformEnvelopes,
		"band_energy": snap.BandEnergySummaries,
		"stereo":      snap.StereoRelationSummaries,
		"loudness":    snap.LoudnessSummaries,
	}
	for _, section := range bootstrapL3Sections {
		for _, row := range sectionRows[section.label] {
			trackID := bootstrapRowString(row, "track_id", "source_track_id")
			if trackID == "" {
				continue // 无轨坐标特征行（v0 不物化，同 materialize featureTrackIDs 口径）
			}
			t := tracks[trackID]
			if t == nil {
				t = &l3Track{sections: map[string]map[string]any{}}
				tracks[trackID] = t
			}
			t.sections[section.label] = row
		}
	}
	for _, trackID := range bootstrapSortedKeys(tracks) {
		t := tracks[trackID]
		payload := map[string]any{"feature_family": "l3", "sections_present": len(t.sections)}
		identity := map[string]any{}
		for _, section := range bootstrapL3Sections {
			row, ok := t.sections[section.label]
			if !ok {
				continue
			}
			payload[section.label+".status"] = bootstrapRowString(row, "status")
			identity[section.label] = row
		}
		rows = append(rows, bootstrapRow{
			row: MaterializedRow{
				Ref: agentprotocol.Ref{
					Kind: "dad.l3", ScopeKind: "track", ScopeValue: trackID,
					Window: &agentprotocol.TimeWindow{AllTime: true}, Snapshot: "current",
					Hash: bootstrapContentHash(identity),
				},
				Freshness: FreshnessMaterialReuse,
				Payload:   payload,
			},
			handle: path,
		})
	}

	// dad.l2_render_probe：每轨一行（最新一条 probe 帧；snapshot=render_revision
	// 可 exact 回溯，缺席落 current）。
	probes := map[string]map[string]any{}
	for _, row := range snap.L2RenderProbes {
		trackID := bootstrapRowString(row, "track_id", "source_track_id")
		if trackID == "" {
			continue
		}
		probes[trackID] = row // 后者覆盖前者=列表序最新
	}
	for _, trackID := range bootstrapSortedKeys(probes) {
		probe := probes[trackID]
		payload := map[string]any{
			"command":       bootstrapRowString(probe, "command"),
			"status":        bootstrapRowString(probe, "status"),
			"balance_state": bootstrapRowString(probe, "balance_state"),
		}
		if bands, ok := probe["bands"].([]any); ok {
			payload["bands_count"] = len(bands)
		}
		snapshot := bootstrapRowString(probe, "render_revision")
		if snapshot == "" {
			snapshot = "current"
		}
		rows = append(rows, bootstrapRow{
			row: MaterializedRow{
				Ref: agentprotocol.Ref{
					Kind: "dad.l2_render_probe", ScopeKind: "track", ScopeValue: trackID,
					Window: &agentprotocol.TimeWindow{AllTime: true}, Snapshot: snapshot,
					Hash: bootstrapContentHash(probe),
				},
				Freshness: FreshnessMaterialReuse,
				Payload:   payload,
			},
			handle: path,
		})
	}
	return rows, nil
}

// bootstrapContentHash 通用内容身份：JSON marshal（键序 canonical）取
// sha256 前 16hex——与 materialize hashHex 同形态（"sha256:"+16hex），供
// bootstrap 自行行化的行（dad 系）使用；materialize 行化产物不经过本函数。
func bootstrapContentHash(value any) string {
	data, err := json.Marshal(value)
	if err != nil {
		data = []byte(fmt.Sprintf("marshal-error:%v", err))
	}
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])[:16]
}

func bootstrapRowString(row map[string]any, keys ...string) string {
	for _, key := range keys {
		if v, ok := row[key]; ok && v != nil {
			if s := strings.TrimSpace(fmt.Sprint(v)); s != "" && s != "<nil>" {
				return s
			}
		}
	}
	return ""
}

func bootstrapSortedKeys[T any](m map[string]T) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// ---- 轮询 diff ----

// bootstrapCoordKey 行坐标键（主键四段，hash 是值不进键——与 materialize
// rowKey 同语义）。
func bootstrapCoordKey(ref agentprotocol.Ref) string {
	window := "all"
	if ref.Window != nil && !ref.Window.AllTime {
		window = fmt.Sprintf("%d..%d", ref.Window.SampleStart, ref.Window.SampleEnd)
	}
	return ref.Kind + "\x1f" + ref.ScopeKind + "\x1f" + ref.ScopeValue + "\x1f" + window + "\x1f" + ref.Snapshot
}

func bootstrapIndexRows(scan *bootstrapScan) map[string]bootstrapRow {
	out := make(map[string]bootstrapRow, len(scan.rows))
	for _, r := range scan.rows {
		out[bootstrapCoordKey(r.row.Ref)] = r
	}
	return out
}

// bootstrapDiffRows 两代扫描 diff → 契约事件（坐标字典序确定性）。
func bootstrapDiffRows(prev, next map[string]bootstrapRow) []MaterializedChange {
	keys := make([]string, 0, len(prev)+len(next))
	seen := map[string]bool{}
	for k := range prev {
		keys = append(keys, k)
		seen[k] = true
	}
	for k := range next {
		if !seen[k] {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	events := make([]MaterializedChange, 0, len(keys))
	for _, key := range keys {
		before, inPrev := prev[key]
		after, inNext := next[key]
		switch {
		case inPrev && !inNext:
			// 产物消失：marked_stale（只改状态不删行的事件面；下次全量重建
			// 如实反映盘面——v0 轮询边界，回执申报）。
			row := before.row
			row.Freshness = FreshnessStale
			events = append(events, MaterializedChange{Op: MaterializedMarkedStale, Row: row})
		case !inPrev && inNext:
			events = append(events, MaterializedChange{Op: MaterializedAdded, Row: after.row})
		case inPrev && inNext:
			if bootstrapMaterialChanged(before, after) {
				events = append(events, MaterializedChange{Op: MaterializedReplaced, Row: after.row})
			}
		}
	}
	return events
}

func bootstrapMaterialChanged(a, b bootstrapRow) bool {
	return a.row.Ref.Hash != b.row.Ref.Hash ||
		a.row.Freshness != b.row.Freshness ||
		a.handle != b.handle ||
		!reflect.DeepEqual(a.row.Payload, b.row.Payload)
}

// 编译期断言：BootstrapStore 实现 queryengine.MaterializedStore（§5 契约）。
var _ MaterializedStore = (*BootstrapStore)(nil)
