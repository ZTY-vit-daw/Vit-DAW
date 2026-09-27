package materialize

// manifest.go — 物化清单落盘与崩溃恢复（MATERIALIZATION §2.1 第三层）。
//
// manifest.json：generation、行坐标→hash→CAS 句柄、载荷标量、失效记账；
// tmp+rename 原子写（harness 原子写模式同款）。崩溃恢复语义（§2.1 原文）：
// 内存索引可从 manifest+CAS 句柄全量重建；不持久化 dirty 集合——恢复后
// 所有存量行降级 material_reuse，等首个权威快照事件（#14）重建基准
// （宁多标不漏标）。manifest 不记 freshness 字段：恢复即统一降级。

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"vit-daw-agent/internal/agentprotocol"
)

const (
	manifestFileName = "manifest.json"
	manifestSchema   = 1
)

// manifestRow 是单行清单项：ref 以 canonical L0 字符串落盘（FormatRef 产物，
// 读回走 ParseRef——经 refschema 语法与注册表门复核），hash 段即内容身份。
type manifestRow struct {
	Ref           string         `json:"ref"`
	Handle        string         `json:"handle,omitempty"`
	Recomputed    int64          `json:"recomputed,omitempty"`
	InvalidatedBy string         `json:"invalidated_by,omitempty"`
	Payload       map[string]any `json:"payload,omitempty"`
}

type manifestFile struct {
	Schema     int64         `json:"schema"`
	Generation int64         `json:"generation"`
	SavedAt    time.Time     `json:"saved_at"`
	Rows       []manifestRow `json:"rows"`
}

// SaveManifest 把当前代整表写入 dir/manifest.json（tmp+rename 原子替换）。
// 行序=rowKey 字典序（确定性清单，供 diff/审计）。
func (s *Store) SaveManifest() error {
	if s.dir == "" {
		return ErrNoManifestDir
	}
	s.mu.RLock()
	gen := s.current
	rows := make([]manifestRow, 0, len(gen.rows))
	keys := make([]string, 0, len(gen.rows))
	for key := range gen.rows {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		row := gen.rows[key]
		refStr, err := agentprotocol.FormatRef(row.Ref)
		if err != nil {
			s.mu.RUnlock()
			return fmt.Errorf("materialize: manifest 行 ref 序列化: %w", err)
		}
		rows = append(rows, manifestRow{
			Ref:           refStr,
			Handle:        row.Handle,
			Recomputed:    row.Recomputed,
			InvalidatedBy: row.InvalidatedBy,
			Payload:       row.Payload,
		})
	}
	s.mu.RUnlock()

	data, err := json.MarshalIndent(&manifestFile{
		Schema:     manifestSchema,
		Generation: gen.id,
		SavedAt:    time.Now().UTC(),
		Rows:       rows,
	}, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return err
	}
	return atomicWriteFile(filepath.Join(s.dir, manifestFileName), data)
}

// loadManifest 按 §2.1 恢复语义重建内存索引。MAT-D 裁定（卡面采信，MAT-A
// fail-loud 的细化）：manifest 是可重建的缓存索引，损坏走"抢救+跳过+记账"
// 而非砖死启动——
//   - manifest 不存在=空库起步（返回 nil）；
//   - 整文件不可解析（半写截断/外部损坏）=流式抢救截断点前已完整落盘的行，
//     RecoverySalvaged 记账（行丢失=lazy 重算回补，损失可见即诚实）；
//   - 个别行损坏（非法 ref/坐标重复）=跳过该行+RecoverySkippedRows 记账
//     （非静默丢行，非整库拒载）；
//   - 空目录=显式 ErrNoManifestDir（不得静默退化为 CWD 相对路径读清单）。
func (s *Store) loadManifest() error {
	if strings.TrimSpace(s.dir) == "" {
		return ErrNoManifestDir
	}
	path := filepath.Join(s.dir, manifestFileName)
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	var mf manifestFile
	salvaged := false
	if err := json.Unmarshal(data, &mf); err != nil {
		mf, salvaged = salvageManifest(data)
	}
	rows := make(map[string]materialRow, len(mf.Rows))
	var skipped int64
	for _, mr := range mf.Rows {
		parsed, err := agentprotocol.ParseRef(mr.Ref)
		if err != nil || parsed.State != agentprotocol.RefStateParsed || parsed.Ref == nil {
			skipped++ // 损坏行：跳过+记账（可见，非静默）
			continue
		}
		ref := cloneRef(*parsed.Ref)
		key := rowKey(ref)
		if _, dup := rows[key]; dup {
			skipped++ // 坐标重复：保留先见者，后到者跳过+记账
			continue
		}
		rows[key] = materialRow{
			Ref:           ref,
			Freshness:     agentprotocol.FreshnessMaterialReuse, // §2.1：恢复后所有存量行统一降级
			Payload:       clonePayload(mr.Payload),
			Handle:        mr.Handle,
			Recomputed:    mr.Recomputed,
			InvalidatedBy: mr.InvalidatedBy,
		}
	}
	s.current = &generation{id: mf.Generation, rows: rows}
	if skipped > 0 || salvaged {
		s.metrics.setRecovery(skipped, salvaged)
	}
	return nil
}

// salvageManifest 从半写/损坏的 manifest 字节流里抢救可完整解码的内容
// （§5.5 崩溃恢复）。SaveManifest 的 tmp+rename 使最终文件要么旧要么新，
// 但磁盘级截断仍可能发生；行对象独立自描述（MarshalIndent 行序在文件尾），
// 截断点之前的完整行可以安全复用（行丢失=lazy 重算回补）。
//
// 返回（抢救到的清单, 是否检测到截断）。流式 token 解码：整文件走完且闭合
// =结构完整（整体 Unmarshal 失败源于别处，行仍可信）→截断=false；任一处
// token/解码断裂=截断点，停止抢救→true。头部字段（schema/generation）在
// 行数组之前落盘，截断在 rows 段时仍可抢救（键序无关地按 token 流取值）。
func salvageManifest(data []byte) (mf manifestFile, truncated bool) {
	dec := json.NewDecoder(bytes.NewReader(data))
	tok, err := dec.Token()
	if err != nil {
		return mf, true
	}
	if d, ok := tok.(json.Delim); !ok || d != '{' {
		return mf, true
	}
	for dec.More() {
		keyTok, err := dec.Token()
		if err != nil {
			return mf, true
		}
		key, ok := keyTok.(string)
		if !ok {
			return mf, true
		}
		if key == "rows" {
			arrTok, err := dec.Token()
			if err != nil {
				return mf, true
			}
			if d, ok := arrTok.(json.Delim); !ok || d != '[' {
				return mf, true
			}
			for dec.More() {
				var mr manifestRow
				if err := dec.Decode(&mr); err != nil {
					return mf, true // 截断点：半行丢弃，已抢救行保留
				}
				mf.Rows = append(mf.Rows, mr)
			}
			if _, err := dec.Token(); err != nil {
				return mf, true // 行数组未闭合
			}
			continue
		}
		var value any
		if err := dec.Decode(&value); err != nil {
			return mf, true
		}
		switch key {
		case "schema":
			if n, ok := value.(float64); ok {
				mf.Schema = int64(n)
			}
		case "generation":
			if n, ok := value.(float64); ok {
				mf.Generation = int64(n)
			}
		}
	}
	if _, err := dec.Token(); err != nil {
		return mf, true // 顶层对象未闭合
	}
	return mf, false
}

// atomicWriteFile 以 tmp+rename 原子替换写整文件（Windows 下 os.Rename 覆盖
// 已存目标；遗留 tmp 在失败路径显式清理）。
func atomicWriteFile(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".manifest-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		os.Remove(tmpName)
		return err
	}
	return nil
}
