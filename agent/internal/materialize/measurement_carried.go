package materialize

// measurement_carried.go — MAT-D2 登记型处置（方案②，零行为变化）。
//
// MAT-D 验收裁定②：S2 红=dom 行经 MixPackage.current_metrics 回退轴激活时与
// 物化侧不同源（DepInputs 不携带 MixPackage 测量——观察侧装配在 snapshot
// 目标键空时回退到 current_metrics.waveform/band_energy，物化侧合成包无此
// 输入，hash 必然不等）。本文件只做登记，不做行为修复：
//
//	回退轴激活形态（判定=mixboard.DOMMeasurementFallbackActive，与观察路径
//	装配同链）→ 该行标记 non_precomputable+measurement_carried 注记
//	（freshness/limitations 语义面）；ZeroDivergence 对账闸门排除该形态行
//	（recompute.go ReconcileShadowDetailed：对账跳过+计数单列
//	ShadowMeasurementCarriedExcluded——不静默）。
//
// 注记是登记状态不是投影内容：只进 Payload 标量（审计/查询可见），不改
// Ref.Hash（内容身份仍是投影自身）。回退序语义（③）不在本处置内（独立卡）。

import "vit-daw-agent/internal/mixboard"

// 注记键（freshness/limitations 语义面的 Payload 标量）。
const (
	// PayloadKeyNonPrecomputable 标记该行不可由物化侧从 DepInputs 重算复现
	// （F9 registered 语义的 dom 形态面：内容携带观察轮测量）。
	PayloadKeyNonPrecomputable = "non_precomputable"
	// PayloadKeyMeasurementCarried 标记该行内容经 MixPackage 测量回退轴携带
	// （观察轮 dom 输入的测量来源注记）。
	PayloadKeyMeasurementCarried = "measurement_carried"
)

// MarkMeasurementCarried 给行打 non_precomputable+measurement_carried 注记
// （幂等；克隆 Payload——不与调用方共享可变结构）。
func MarkMeasurementCarried(row Row) Row {
	payload := make(map[string]any, len(row.Payload)+2)
	for key, value := range row.Payload {
		payload[key] = value
	}
	payload[PayloadKeyNonPrecomputable] = true
	payload[PayloadKeyMeasurementCarried] = true
	row.Payload = payload
	return row
}

// IsMeasurementCarried 报告行是否携带 measurement_carried 注记（对账闸门的
// 排除形态判定）。
func IsMeasurementCarried(row Row) bool {
	carried, ok := row.Payload[PayloadKeyMeasurementCarried].(bool)
	return ok && carried
}

// DOMRowFromObservation 把观察轮 dom 投影行化（DOMRowFromProjection 同口径），
// 并按 MixPackage 测量回退轴激活形态打 measurement_carried 注记（MAT-D2 登记
// 型入口：harness 影子轮消费）。obs 只读；行化失败（空投影）不产行。
func DOMRowFromObservation(trackID string, obs *mixboard.ObservationPacket) (Row, bool) {
	if obs == nil || obs.DOMProjection == nil {
		return Row{}, false
	}
	row, ok := DOMRowFromProjection(trackID, *obs.DOMProjection)
	if !ok {
		return Row{}, false
	}
	if mixboard.DOMMeasurementFallbackActive(*obs) {
		row = MarkMeasurementCarried(row)
	}
	return row, true
}
