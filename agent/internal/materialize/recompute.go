package materialize

// recompute.go — kind 适配器注册与重算编排（MAT-C，设计 §4.1/§2.2/§5.3/§7.1）。
//
// §4.1 两段式的实现半边：kind 级依赖与 Build 在适配器静态注册（启动期一次），
// 行级实例匹配归 MAT-B 传播。重算三入口（§2.2）中 v1 落地两支：
//
//	RecomputeLazy(deps)  lazy 语义——只重算"无行或含 stale 行"的 precomputable
//	                    kind（observe 读端回填语义的前置形态；不为干净 kind 付费）；
//	FullRebuild(deps)    oracle——无视 dirty 集合强制全部 precomputable kind 重算
//	                    （G2-B 对拍的另一轨；与增量轨共享同一 Build 与输入读取
//	                    路径，对拍检测的只是传播漏标，§5.3 可信度前提）。
//
// 影子对账（§7.1）：ReconcileShadow 把一轮 observe 的现算产物与物化行逐行
// hash 比，分歧行计数入 Metrics.ShadowDivergences（切换闸门的依据仪表）。
//
// 单写者纪律（§7.3）：重算提交与标脏同走 Store 互斥锁；适配器 Build 本身是
// 纯函数（deps 进、rows 出），不回调 Store。

import (
	"fmt"
	"sort"

	"vit-daw-agent/internal/acousticpackage"
	"vit-daw-agent/internal/agentprotocol"
)

// DepInputs 是适配器消费的输入束（§4.1 DepInputs 的 v1 形态）：三个先切 kind
// 的全部输入域——tom←shadow 工程快照（project.structure）、acp←acousticpackage
// store 快照行、dom←mixboard feature snapshot（source_only 档；paired/change
// 类请求携带测量不预计算，F9）。harness 侧装配（materialize_shadow.go）。
type DepInputs struct {
	ProjectState     map[string]any           // shadow Summary（tracks/project_revision）
	FeatureSnapshot  map[string]any           // feature snapshot 顶层键形态
	AcousticPackages []acousticpackage.Status // acp store 快照行
	ProjectRevision  string                   // dom Conditions.ProjectRevision 条件轴
}

// 适配器的 Compute 档位词表（§4.1）：v1 适配器注册表只收 precomputable——
// registered（fxm/com，F9）不经适配器，其写入只有登记路径（Upsert 直通）。
const (
	ComputePrecomputable = "precomputable"
	ComputeRegistered    = "registered"
)

// KindAdapter 是一个 precomputable kind 的物化适配器：Build 从 DepInputs 现算
// 该 kind 的行集（Upsert 提交点）。InputScopes 是文档性依赖域声明（表 B 词表
// 子集），供审计对照，不参与传播计算（传播查表 B）。
type KindAdapter struct {
	Kind        string
	Compute     string
	InputScopes []string
	Build       func(deps DepInputs) ([]Row, error)
}

// RegisterAdapters 注册适配器（启动期一次；kind 必须在 refschema 注册表内、
// Compute 必须为 precomputable、Build 非空；重复注册报错）。
func (s *Store) RegisterAdapters(adapters ...KindAdapter) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, adapter := range adapters {
		if adapter.Kind == "" {
			return fmt.Errorf("materialize: 适配器 kind 为空")
		}
		if adapter.Compute != ComputePrecomputable {
			return fmt.Errorf("materialize: 适配器 %s 的 Compute 必须为 %s（registered 走登记路径）", adapter.Kind, ComputePrecomputable)
		}
		if adapter.Build == nil {
			return fmt.Errorf("materialize: 适配器 %s 缺 Build", adapter.Kind)
		}
		if !kindRegistered(adapter.Kind) {
			return fmt.Errorf("materialize: 适配器 kind %q 不在 refschema 注册表", adapter.Kind)
		}
		if s.adapters == nil {
			s.adapters = map[string]KindAdapter{}
		}
		if _, dup := s.adapters[adapter.Kind]; dup {
			return fmt.Errorf("materialize: 适配器 %s 重复注册", adapter.Kind)
		}
		s.adapters[adapter.Kind] = adapter
		s.adapterOrder = append(s.adapterOrder, adapter.Kind)
		sort.Strings(s.adapterOrder)
	}
	return nil
}

func kindRegistered(kind string) bool {
	for _, candidate := range agentprotocol.RegisteredRefKinds() {
		if candidate == kind {
			return true
		}
	}
	return false
}

// AdapterKinds 返回已注册适配器的 kind 集（字典序拷贝）。
func (s *Store) AdapterKinds() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]string(nil), s.adapterOrder...)
}

// RecomputeKinds 显式重算指定 kind（deps→Build→Upsert）。未注册的 kind 报错
// （fail-loud：白名单 typo 不静默）。
func (s *Store) RecomputeKinds(deps DepInputs, kinds ...string) error {
	for _, kind := range kinds {
		s.mu.RLock()
		adapter, ok := s.adapters[kind]
		s.mu.RUnlock()
		if !ok {
			return fmt.Errorf("materialize: kind %q 无已注册适配器", kind)
		}
		if err := s.recomputeAdapter(deps, adapter); err != nil {
			return err
		}
	}
	return nil
}

// RecomputeLazy 重算"无行或含 stale 行"的 precomputable kind（§2.2 lazy：miss/
// stale 才付费；干净 kind 不重算）。kind 级判定（§4.4 脏的单位=投影）。
func (s *Store) RecomputeLazy(deps DepInputs) error {
	for _, kind := range s.lazyKinds() {
		s.mu.RLock()
		adapter := s.adapters[kind]
		s.mu.RUnlock()
		if err := s.recomputeAdapter(deps, adapter); err != nil {
			return err
		}
	}
	return nil
}

// FullRebuild 强制重算全部已注册 precomputable kind（G2-B oracle，§5.3：无视
// dirty 集合）。与增量轨共享同一 Build 与 deps——对拍检测传播漏标。
func (s *Store) FullRebuild(deps DepInputs) error {
	for _, kind := range s.AdapterKinds() {
		s.mu.RLock()
		adapter := s.adapters[kind]
		s.mu.RUnlock()
		if err := s.recomputeAdapter(deps, adapter); err != nil {
			return err
		}
	}
	return nil
}

// recomputeAdapter 跑一个适配器并提交（Build 纯函数→Upsert 行级 diff 提交点）。
// 提交成功（含空行集 no-op）清除该 kind 的事件级脏标记——新 scope 行在重算
// 后已被行集覆盖，标记的使命完成。
func (s *Store) recomputeAdapter(deps DepInputs, adapter KindAdapter) error {
	rows, err := adapter.Build(deps)
	if err != nil {
		return fmt.Errorf("materialize: %s 适配器 Build: %w", adapter.Kind, err)
	}
	if len(rows) > 0 {
		if err := s.Upsert(rows); err != nil {
			return err
		}
	}
	s.clearKindDirty(adapter.Kind)
	return nil
}

// lazyKinds 返回需要 lazy 重算的 kind 集：事件级脏标记命中（§4.4 脏的单位=
// 投影，新 scope 行只能由此产生）、无行（miss）或任一行 stale。
// 已消失 scope 的 stale 行（重算后不再产出的旧坐标行，§2.3 stale 不删行）会让
// 所在 kind 每轮都被判脏——正确性无损（重算是 no-op 提交），浪费留 v2 收口。
func (s *Store) lazyKinds() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	kindHasRows := map[string]bool{}
	kindHasStale := map[string]bool{}
	for _, row := range s.current.rows {
		kindHasRows[row.Ref.Kind] = true
		if row.Freshness == agentprotocol.FreshnessStale {
			kindHasStale[row.Ref.Kind] = true
		}
	}
	var out []string
	for _, kind := range s.adapterOrder {
		if s.dirtyKinds[kind] || !kindHasRows[kind] || kindHasStale[kind] {
			out = append(out, kind)
		}
	}
	return out
}

// ReconcileShadow 影子对账（§7.1 shadow）：把一轮 observe 的现算产物（行化后，
// 与物化行同坐标同 hash 口径）与物化行逐行比对——同坐标 hash/payload 不等或
// 物化行缺失即分歧；分歧行数返回并累计入 Metrics.ShadowDivergences。现算侧
// 多出的内容不判分歧（观察是目标域子集，物化面更宽是常态）。
func (s *Store) ReconcileShadow(fresh []Row) int {
	divergences, _ := s.ReconcileShadowDetailed(fresh)
	return divergences
}

// ShadowDivergenceDetail 是单行分歧的取证明细（MAT-D：真栈分歧的根因级证据——
// 坐标+两侧 hash+两侧 payload，G2 上交材料的粒度）。
type ShadowDivergenceDetail struct {
	Ref           string                        // canonical ref（分歧行坐标）
	FreshHash     string                        // 现算侧内容身份
	StoredHash    string                        // 物化侧内容身份（行缺失时空）
	StoredMissing bool                          // 物化侧无该坐标行
	FreshPayload  map[string]any                // 现算侧标量（对照用）
	StoredPayload map[string]any                // 物化侧标量（行缺失时 nil）
	InvalidatedBy string                        // 物化侧最近失效记账（脏残留线索）
	Freshness     string                        // 物化侧 freshness
	Row           agentprotocol.MaterializedRow // 物化侧行（日志/调试消费）
}

// ReconcileShadowDetailed 同 ReconcileShadow，另返回分歧明细（MAT-D 取证面：
// 真栈 ShadowDivergences!=0 时 harness 侧逐行打 WARN——切换闸门红必须可解释）。
func (s *Store) ReconcileShadowDetailed(fresh []Row) (int, []ShadowDivergenceDetail) {
	if len(fresh) == 0 {
		return 0, nil
	}
	s.mu.RLock()
	stored := make(map[string]materialRow, len(s.current.rows))
	for key, row := range s.current.rows {
		stored[key] = row
	}
	s.mu.RUnlock()

	divergences := 0
	var details []ShadowDivergenceDetail
	for _, row := range fresh {
		if err := row.Ref.Validate(); err != nil {
			// 现算侧行化产出非法坐标=对账输入缺陷，计分歧（fail-visible）。
			divergences++
			continue
		}
		have, ok := stored[rowKey(row.Ref)]
		if !ok {
			divergences++
			refStr, _ := agentprotocol.FormatRef(row.Ref)
			details = append(details, ShadowDivergenceDetail{
				Ref: refStr, FreshHash: row.Ref.Hash, StoredMissing: true, FreshPayload: row.Payload,
			})
			continue
		}
		if have.Ref.Hash != row.Ref.Hash || !payloadEqual(have.Payload, row.Payload) {
			divergences++
			refStr, _ := agentprotocol.FormatRef(row.Ref)
			details = append(details, ShadowDivergenceDetail{
				Ref:           refStr,
				FreshHash:     row.Ref.Hash,
				StoredHash:    have.Ref.Hash,
				FreshPayload:  row.Payload,
				StoredPayload: clonePayload(have.Payload),
				InvalidatedBy: have.InvalidatedBy,
				Freshness:     have.Freshness,
				Row:           contractRow(have),
			})
		}
	}
	// 比对行数与分歧同源累计（MAT-D：零分歧的非空洞证据）。
	s.metrics.addReconcileRows(int64(len(fresh)))
	if divergences > 0 {
		s.metrics.addShadowDivergence(int64(divergences))
	}
	return divergences, details
}
