package materialize

// table_b.go — 表 B：观察域→kind 数据表（MAT-B，设计 §4.2）。
//
// 脏传播的两张表串联（§4.2）：事件/收据 ──(表 A：shadow addChangeScopes，既有
// 不动)──> 观察域 scopes ──(表 B：本文件)──> kind 集合 ──(行坐标匹配)──> ref 行集。
//
// 初值来源（§4.2 主表+arrive 域段，对照 RECON §1.1 触发链矩阵与 QUERY_ENGINE
// §3.3 逐 kind 推演）：
//   - precomputable kind（事件可重算）：dom/mom/tim/tom/epm/acp；
//   - registered 登记型（F9：fxm/com 随观察落盘登记，rlm 半物化 §4.5 只登记
//     不重算）——两类在 MAT-B 的标脏传播里同构（都进 dirty 集合，§3.2），
//     重算与否的区分归 MAT-C 的 Recompute 适配。
//
// 锁定：TestTableBInitialValuesLocked（表驱动逐条目锁定+kind ⊆ refschema 注册表+
// 表 A 词表全覆盖）。project.state（表 A 默认兜底）不是条目——"全部 kind 保守
// 标脏"由传播算法的查表落空兜底实现（宁多勿漏，条目化反而会冻结全 kind 列表）。

// TableBEntry 是表 B 的一行：观察域 → 受影响 kind 集合。
type TableBEntry struct {
	Domain string   // 观察域（表 A 词表或 arrive 域词表）
	Kinds  []string // 受影响 kind（含登记型；语义=该域变更须把这些 kind 的行标脏）
}

// tableB 是生产初值（设计 §4.2）。修改任何条目必须同步改表驱动锁定测试。
var tableB = []TableBEntry{
	// invalidate 域（表 A 词表，§4.2 主表）
	{"track.level", []string{"dom", "mom", "tim", "rlm"}}, // rlm=登记（半物化 §4.5）
	{"track.stereo_space", []string{"dom", "mom"}},
	{"track.basic_energy", []string{"dom", "tim"}},
	{"track.time_dynamics", []string{"dom", "tim"}},
	{"track.timbre_frequency", []string{"dom", "tim"}},
	{"mix.multitrack_relationship", []string{"mom"}},
	{"mix.frequency_relationship", []string{"mom"}},
	{"mix.masking_relationship", []string{"mom"}},
	{"project.headroom", []string{"dom", "rlm"}},
	{"project.structure", []string{"tom", "tim", "epm"}},
	{"processor.identity_and_controls", []string{"com"}},
	{"processor.behavior", []string{"com"}},
	{"processor.change_delta", []string{"com"}},
	{"comparison.before_after", []string{"fxm"}},
	// arrive 域（§4.2 末段：#4/#5 特征瓦片→feature.*；#6-#8 L3 族→acoustic.l3.<family>）
	{"feature.waveform", []string{"dom", "tim"}},
	{"feature.spectral", []string{"dom", "tim"}},
	{"acoustic.l3.band_energy", []string{"mom", "acp", "dom", "tim"}},
	{"acoustic.l3.stereo_relation", []string{"mom", "acp", "dom", "tim"}},
	{"acoustic.l3.loudness", []string{"mom", "acp", "dom", "tim"}},
	// #9 收编（§2.2 批 4/RECON §5.5 分叉点闭合）：L2 probe 证据到达——masking 帧
	// →mom；轨道级测量→dom/tim（宁多勿漏档位，不收窄到 mom 单 kind）。
	{"feature.l2_render_probe", []string{"mom", "dom", "tim"}},
}

// TableBEntries 返回生产表 B 的拷贝（表驱动测试锁定与审计用）。
func TableBEntries() []TableBEntry {
	out := make([]TableBEntry, len(tableB))
	for i, entry := range tableB {
		out[i] = TableBEntry{Domain: entry.Domain, Kinds: append([]string(nil), entry.Kinds...)}
	}
	return out
}

// tableBIndex 是生产表的域→kind 索引（启动期一次构建，只读）。
var tableBIndex = buildTableBIndex(tableB)

func buildTableBIndex(entries []TableBEntry) map[string][]string {
	index := make(map[string][]string, len(entries))
	for _, entry := range entries {
		index[entry.Domain] = append([]string(nil), entry.Kinds...)
	}
	return index
}
