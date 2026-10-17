package fxm

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
)

// AGENT-SENDROUTE-SEMANTICS-1（设计件 §3 FXM 行）：路由指纹纳入 send 拓扑
// hash——send 边变化破坏「同路由」前提，折叠进 RoutingHash 后随 ProjectionID
// 种子旋转，复用现有 fingerprint 失效机制，不新增投影。

// SendTopologyHash folds the send/return topology read-back (track.send.list
// shape: sends + dangling rows) into a stable fingerprint. Edge identity
// (source->fx), send amount, and bus pair participate; row order does not.
// An unreadable or absent topology yields "" (the caller keeps the base
// routing hash unchanged rather than fabricating a fingerprint).
func SendTopologyHash(topology any) string {
	row, ok := topology.(map[string]any)
	if !ok || len(row) == 0 {
		return ""
	}
	edges := make([]string, 0, 8)
	collect := func(value any, danglingTag string) {
		switch rows := value.(type) {
		case []any:
			for _, raw := range rows {
				if edge, ok := raw.(map[string]any); ok {
					edges = append(edges, canonicalSendEdge(edge, danglingTag))
				}
			}
		case []map[string]any:
			for _, edge := range rows {
				edges = append(edges, canonicalSendEdge(edge, danglingTag))
			}
		}
	}
	collect(row["sends"], "live")
	collect(row["dangling"], "dangling")
	if len(edges) == 0 {
		// A topology read that returned a shape with no edge lists at all is
		// not a fingerprint fact; only an explicit empty topology hashes.
		if row["sends"] == nil && row["dangling"] == nil {
			return ""
		}
	}
	sort.Strings(edges)
	sum := sha256.Sum256([]byte(strings.Join(edges, "|")))
	return "sendtopo:" + hex.EncodeToString(sum[:])[:16]
}

func canonicalSendEdge(edge map[string]any, tag string) string {
	source, _ := edge["source_track_id"].(string)
	fx, _ := edge["fx_track_id"].(string)
	return fmt.Sprintf("%s>%s:%s:%v:%v:%v", trimSpaceDefault(source, "?"), trimSpaceDefault(fx, "?"), tag,
		edge["amount_db"], edge["send_bus"], edge["return_bus"])
}

func trimSpaceDefault(value, fallback string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fallback
	}
	return trimmed
}

// RoutingHashWithSendTopology combines a base routing hash with the send
// fingerprint: either side rotating rotates the combined identity, which
// feeds the FXM projection seed (same-routing premise + cache staleness).
func RoutingHashWithSendTopology(base, sendHash string) string {
	sendHash = strings.TrimSpace(sendHash)
	if sendHash == "" {
		return base
	}
	base = strings.TrimSpace(base)
	if base == "" {
		return sendHash
	}
	sum := sha256.Sum256([]byte(base + "+" + sendHash))
	return "route:" + hex.EncodeToString(sum[:])[:16]
}
