package materialize

// timing_carried.go — MAT-D4 登记型处置（与 measurement-carried 同款：对账
// 闸门排除+计数单列，不静默；零行为修复——不改观察路径，不改行内容）。
//
// MAT-D3 取证/验收裁定②：真栈 S2 剩余分歧=观察投影与物化输入的时序两态
// （观察侧投影是 finalize 时点用当时 snapshot 算的；物化尾挂取到的
// feature_snapshot 在 finalize 之后已前进/被穿透——三角重放 replay_eq_obs=false
// 实证）。裁定：时序两态不构成失效漏标，G2-D 闭环口径=收敛态（静默窗后零
// 分歧），瞬时轮分歧按 timing-carried 登记排除。
//
// 判定（对账时，recompute.go ReconcileShadowWithTiming 消费）：
//
//	对会判分歧的 dom 行，用当前 deps 同路径重放（BuildDOMRowForTrack——与
//	DOMAdapter.Build 严格同链）：
//	  重放==物化行 而 观察行异
//	    → 分歧可归因两侧输入世代差（物化行反映当前输入，观察投影停在 finalize
//	      世代）→ timing-carried：排除+单列计数 ShadowTimingCarriedExcluded；
//	  重放缺行 / 重放≠物化行
//	    → 判定不可靠（无法区分 timing 与真分歧）→ 宁计分歧不误排除
//	      （宁少勿滥——对账口径不是失效传播，与漏标检测的宁多勿漏不冲突）。
//
// 登记形态说明：与 measurement-carried 的行注记不同，timing 是对账两侧的
// 关系不是行的持久属性（物化行本身无错），故不注记行、只在对账时判定+计数。

// timingCarriedSignature 报告一个"会判分歧"的 fresh 行是否呈 timing-carried
// 签名：当前输入（deps）同路径重放与物化行全等（hash+payload——物化行反映
// 当前输入世代）而观察行异（观察投影停在旧世代）。deps 为空或非 dom/track
// 行直接 false（不参与判定）。
func timingCarriedSignature(fresh Row, stored materialRow, deps *DepInputs) bool {
	if deps == nil || fresh.Ref.Kind != "dom" || fresh.Ref.ScopeKind != "track" {
		return false
	}
	replay, ok := BuildDOMRowForTrack(*deps, fresh.Ref.ScopeValue)
	if !ok {
		return false // 当前输入重放不出该轨行=不可靠 → 宁计分歧
	}
	if replay.Ref.Hash != stored.Ref.Hash || !payloadEqual(replay.Payload, stored.Payload) {
		return false // 物化行不反映当前输入（陈旧/异源）=不可靠 → 宁计分歧
	}
	return replay.Ref.Hash != fresh.Ref.Hash || !payloadEqual(replay.Payload, fresh.Payload)
}
