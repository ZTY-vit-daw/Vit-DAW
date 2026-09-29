package queryengine

// bridge.go — agentprotocol.MaterializedStore（MAT-A 契约下沉，materialize.Store
// 实现）→ queryengine.MaterializedStore（引擎侧冻结接口）的只读类型桥。
//
// 两个类型世界的分工（接口冻结，IMPL-A 已定签名不动）：
//   - agentprotocol：物化层契约的协议归属（Subscribe 消费面不止引擎——台账/
//     审计/未来 memory 检索都可直接消费协议面）；
//   - queryengine：引擎消费面（谓词/索引/游标与引擎类型同族）。
//
// 本桥是 v1 接线点：引擎吃 agentprotocol 实现（materialize.Store 即插）时经
// NewProtocolStore 适配。类型一一对应转换，零语义改写；错误原样透传（%w 不
// 包装语义，只加前缀定位）。契约测试（store_contract_test.go）经本桥在引擎
// 消费面锁定 materialize 已验语义（upsert 代际/句柄保留/marked_stale 不删行
// ——MAT-E 行为为准绳，不重定义）。

import (
	"context"
	"fmt"
	"sync"

	"vit-daw-agent/internal/agentprotocol"
)

// ProtocolStore 把 agentprotocol.MaterializedStore 适配为引擎侧
// MaterializedStore。零值不可用；NewProtocolStore 构造。
type ProtocolStore struct {
	inner agentprotocol.MaterializedStore
}

// NewProtocolStore 构造桥。inner 为 nil 视为编程错误（引擎无 store 不可用，
// 与 NewEngine 的可 nil payload 不同——store 是引擎的硬依赖）。
func NewProtocolStore(inner agentprotocol.MaterializedStore) *ProtocolStore {
	if inner == nil {
		panic("queryengine: NewProtocolStore(nil)——引擎的物化读口是硬依赖")
	}
	return &ProtocolStore{inner: inner}
}

// SnapshotView latest/exact 语义归物化层（§5.1.2 引擎零推断），selector 逐
// 字段转换透传。
func (p *ProtocolStore) SnapshotView(ctx context.Context, sel SnapshotSelector) ([]MaterializedRow, error) {
	rows, err := p.inner.SnapshotView(ctx, agentprotocol.SnapshotSelector{
		Mode:          agentprotocol.SnapshotMode(sel.Mode),
		Revision:      sel.Revision,
		ObservationID: sel.ObservationID,
	})
	if err != nil {
		return nil, fmt.Errorf("queryengine: protocol store snapshot view: %w", err)
	}
	out := make([]MaterializedRow, 0, len(rows))
	for _, r := range rows {
		out = append(out, MaterializedRow{Ref: r.Ref, Freshness: r.Freshness, Payload: r.Payload})
	}
	return out, nil
}

// Resolve 句柄/状态/读取函数逐字段透传（Ref 两面同型，共享引用安全——
// 物化层契约行返回时已做克隆）。
func (p *ProtocolStore) Resolve(ctx context.Context, ref agentprotocol.Ref) (ResolvedEvidence, error) {
	res, err := p.inner.Resolve(ctx, ref)
	if err != nil {
		return ResolvedEvidence{}, fmt.Errorf("queryengine: protocol store resolve: %w", err)
	}
	return ResolvedEvidence{
		Handle:    res.Handle,
		Bytes:     res.Bytes,
		Freshness: res.Freshness,
		ReadAll:   res.ReadAll,
	}, nil
}

// Subscribe 变更流事件逐字段转换（Op 字符串值两面同词表——MAT-A 下沉时常量
// 同源，此处类型转换不改语义）。转发 goroutine 随取消（unsubscribe 或上游
// 关闭）退出——消费侧停读不得遗留阻塞发送。
func (p *ProtocolStore) Subscribe(ctx context.Context) (<-chan MaterializedChange, func(), error) {
	innerCh, unsubscribe, err := p.inner.Subscribe(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("queryengine: protocol store subscribe: %w", err)
	}
	if innerCh == nil {
		return nil, unsubscribe, nil
	}
	ch := make(chan MaterializedChange, cap(innerCh))
	done := make(chan struct{})
	var closeOnce sync.Once
	cancel := func() {
		closeOnce.Do(func() {
			unsubscribe()
			close(done)
		})
	}
	go func() {
		defer close(ch)
		for {
			select {
			case <-done:
				return
			case change, ok := <-innerCh:
				if !ok {
					return
				}
				converted := MaterializedChange{
					Op:  MaterializedOp(change.Op),
					Row: MaterializedRow{Ref: change.Row.Ref, Freshness: change.Row.Freshness, Payload: change.Row.Payload},
				}
				select {
				case ch <- converted:
				case <-done:
					return
				}
			}
		}
	}()
	return ch, cancel, nil
}

// 编译期断言：ProtocolStore 实现 queryengine.MaterializedStore。
var _ MaterializedStore = (*ProtocolStore)(nil)
