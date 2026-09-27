package materialize

// manifest.go — 物化清单落盘与崩溃恢复（MATERIALIZATION §2.1 第三层）。
//
// manifest.json：generation、行坐标→hash→CAS 句柄、载荷标量、失效记账；
// tmp+rename 原子写（harness 原子写模式同款）。崩溃恢复语义（§2.1 原文）：
// 内存索引可从 manifest+CAS 句柄全量重建；不持久化 dirty 集合——恢复后
// 所有存量行降级 material_reuse，等首个权威快照事件（#14）重建基准
// （宁多标不漏标）。manifest 不记 freshness 字段：恢复即统一降级。

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
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

// loadManifest 按 §2.1 恢复语义重建内存索引；manifest 不存在=空库起步，
// 存在但损坏/含非法 ref/坐标重复=fail-loud（返回错误，不静默丢行）。
func (s *Store) loadManifest() error {
	path := filepath.Join(s.dir, manifestFileName)
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	var mf manifestFile
	if err := json.Unmarshal(data, &mf); err != nil {
		return fmt.Errorf("materialize: manifest 损坏（%s）: %w", path, err)
	}
	rows := make(map[string]materialRow, len(mf.Rows))
	for _, mr := range mf.Rows {
		parsed, err := agentprotocol.ParseRef(mr.Ref)
		if err != nil {
			return fmt.Errorf("materialize: manifest 行 ref 非法: %w", err)
		}
		if parsed.State != agentprotocol.RefStateParsed || parsed.Ref == nil {
			return fmt.Errorf("materialize: manifest 行 ref 非 canonical 解析态: %q", mr.Ref)
		}
		ref := cloneRef(*parsed.Ref)
		key := rowKey(ref)
		if _, dup := rows[key]; dup {
			return fmt.Errorf("materialize: manifest 重复行坐标: %q", mr.Ref)
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
	return nil
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
