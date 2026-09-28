package queryengine

import "errors"

// errors.go — 查询引擎错误语义。"空结果"与"错误"是两种不同信号：
// 谓词不命中=空集；输入不可判定（未知 kind / 非法谓词 / 越界）=错误。

var (
	// ErrUnknownKind 查询引用了注册表之外的 kind（空集 ≠ kind 不存在，§2.1）。
	ErrUnknownKind = errors.New("queryengine: unknown ref kind")
	// ErrInvalidScope scope 谓词自相矛盾（如 ValueSet 但 Values 为空）。
	ErrInvalidScope = errors.New("queryengine: invalid scope predicate")
	// ErrInvalidWindow window 谓词非法（负采样点 / start > end）。
	ErrInvalidWindow = errors.New("queryengine: invalid window predicate")
	// ErrInvalidSnapshot snapshot 谓词非法（exact/at_or_before 缺 revision；latest 带 revision）。
	ErrInvalidSnapshot = errors.New("queryengine: invalid snapshot predicate")
	// ErrInvalidHash hash 谓词非法（非 16 位小写 hex；Any 与 SHA256 并设矛盾）。
	ErrInvalidHash = errors.New("queryengine: invalid hash predicate")
	// ErrInvalidPayloadCondition 载荷条件非法（空字段 / 未知算子）。
	ErrInvalidPayloadCondition = errors.New("queryengine: invalid payload condition")
	// ErrUnsupportedSort 排序字段未声明（R6：不静默乱序）。
	ErrUnsupportedSort = errors.New("queryengine: unsupported sort field")
	// ErrInvalidLimit limit 越界（负数 / 超硬上限 500）。
	ErrInvalidLimit = errors.New("queryengine: invalid limit")
	// ErrStaleCursor 游标 malformed 或绑定代与当前查询代不符（快照已换代）。
	ErrStaleCursor = errors.New("queryengine: stale cursor")
	// ErrInvalidExpandOptions expand 选项非法（MaxBytes 越界）。
	ErrInvalidExpandOptions = errors.New("queryengine: invalid expand options")
	// ErrInvalidRef ref 串不可解析为 L0 结构（malformed vit://）。
	ErrInvalidRef = errors.New("queryengine: invalid ref")
	// ErrInvalidDiffRef 差分侧集非法（空集 / legacy|opaque 缺 L0 坐标）。
	ErrInvalidDiffRef = errors.New("queryengine: invalid diff ref set")
	// ErrNotImplemented 骨架留位：该深度/路径在后续实现卡接线（IMPL-B/C/D）。
	ErrNotImplemented = errors.New("queryengine: not implemented in this card")
)
