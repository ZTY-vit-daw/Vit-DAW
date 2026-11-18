package legamedesign

// 编译产物清单与缓存失效（契约 §6.5）：
//   缓存键四元组 = (设计源字节 SHA-256, 编译器版本, 装配输入清单哈希, 目标卡 schema 版本)
//   装配输入清单哈希 = materials 准入文件 sha256 集合 + catalog.yaml 哈希 + 覆盖参数
//   manifest 外置 = <id>.compiled.level.yaml + <id>.compile.manifest.json
//   源为权威：载入卡前核对 manifest；键不匹配/缺失/卡哈希不符 → 拒绝缓存（fail-closed）
//   失效幂等：重编译失败保留旧产物但校验已失效 → 阻断使用，不得降级运行旧卡

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"vit-daw-agent/internal/edu"
)

// 缓存校验错误分类（errors.Is 可判别）。
var (
	// ErrCompiledMissing 表示 manifest 或卡文件缺失（缓存不完整，不可用）。
	ErrCompiledMissing = errors.New("legamedesign: compiled cache missing")
	// ErrCacheStale 表示四元组键不一致（设计源/装配输入/编译器/卡版本变化→须重编译）。
	ErrCacheStale = errors.New("legamedesign: compiled cache stale")
	// ErrCompiledTampered 表示 manifest/卡完整性不符（手改 compiled 卡、manifest 损坏）。
	ErrCompiledTampered = errors.New("legamedesign: compiled cache integrity mismatch")
)

// CacheKey 是 §6.5 的四元组缓存键。
type CacheKey struct {
	DesignSourceSHA256 string `json:"design_source_sha256"`
	CompilerVersion    string `json:"compiler_version"`
	AssemblyInputHash  string `json:"assembly_input_hash"`
	CardSchemaVersion  string `json:"card_schema_version"`
}

// Equal 比较四元组键（缓存命中判定）。
func (k CacheKey) Equal(other CacheKey) bool {
	return k.DesignSourceSHA256 == other.DesignSourceSHA256 &&
		k.CompilerVersion == other.CompilerVersion &&
		k.AssemblyInputHash == other.AssemblyInputHash &&
		k.CardSchemaVersion == other.CardSchemaVersion
}

// ManifestDiagnostic 是 manifest 中的一条 W 诊断（规则号+字段路径+人读提示）。
type ManifestDiagnostic struct {
	Rule    string `json:"rule"`
	Path    string `json:"path,omitempty"`
	Message string `json:"message"`
}

// AdmittedAssetRecord 是 T-13 准入资产的 manifest 回执（含 catalog 核验状态）。
type AdmittedAssetRecord struct {
	MaterialID    string `json:"material_id"`
	File          string `json:"file"`
	SHA256        string `json:"sha256"`
	CatalogID     string `json:"catalog_id,omitempty"`
	CatalogStatus string `json:"catalog_status,omitempty"`
	SoundCheck    string `json:"sound_check,omitempty"`
}

// ExcludedMaterialRecord 是待备料清单回执（未进卡资产及原因）。
type ExcludedMaterialRecord struct {
	MaterialID string `json:"material_id"`
	Kind       string `json:"kind,omitempty"`
	File       string `json:"file,omitempty"`
	Reason     string `json:"reason"`
}

// ManifestReceipt 是编译回执（T-21/R3 与准入/待备料清单的可审计记录）。
type ManifestReceipt struct {
	PendingQuestions  []string                 `json:"pending_questions,omitempty"`
	TrackMapping      []trackMappingRecord     `json:"track_mapping"`
	AdmittedAssets    []AdmittedAssetRecord    `json:"admitted_assets,omitempty"`
	ExcludedMaterials []ExcludedMaterialRecord `json:"excluded_materials,omitempty"`
}

// CompileManifest 是编译产物清单（§6.5）。
type CompileManifest struct {
	SchemaVersion       string               `json:"schema_version"`
	MappingTableVersion string               `json:"mapping_table_version"`
	Key                 CacheKey             `json:"cache_key"`
	CardFile            string               `json:"card_file"`
	CardSHA256          string               `json:"card_sha256"`
	Warnings            []ManifestDiagnostic `json:"warnings"`
	Receipt             ManifestReceipt      `json:"receipt"`
}

// assemblyInputCanonical 是装配输入清单哈希的规范形态（排序后序列化=确定性）。
type assemblyInputCanonical struct {
	CatalogSHA256      string   `json:"catalog_sha256"`
	AdmittedFileSHA256 []string `json:"admitted_file_sha256"`
	TempoBPM           *float64 `json:"tempo_bpm"`
	MaxTurns           *int     `json:"max_turns"`
	MaxLLMCalls        *int     `json:"max_llm_calls"`
}

// computeAssemblyInputHash 计算装配输入清单哈希（§6.5：准入文件 sha256 集合 +
// catalog 哈希 + 覆盖参数）。
func computeAssemblyInputHash(catalogSHA256 string, admittedFileSHAs []string, ov CompileOverrides) string {
	shas := append([]string(nil), admittedFileSHAs...)
	sort.Strings(shas)
	// 去重（集合语义）
	uniq := shas[:0]
	for _, s := range shas {
		if len(uniq) == 0 || uniq[len(uniq)-1] != s {
			uniq = append(uniq, s)
		}
	}
	payload, _ := json.Marshal(assemblyInputCanonical{
		CatalogSHA256:      catalogSHA256,
		AdmittedFileSHA256: uniq,
		TempoBPM:           ov.TempoBPM,
		MaxTurns:           ov.MaxTurns,
		MaxLLMCalls:        ov.MaxLLMCalls,
	})
	return sha256Hex(payload)
}

// BuildCacheKey 按当前输入重新计算四元组键（零写入；读取设计源/catalog/准入材料）。
// 设计源当前不合法或装配输入不可读时返回错误（缓存校验因此 fail-closed）。
func BuildCacheKey(input CompileInput) (CacheKey, error) {
	designData, err := os.ReadFile(input.DesignPath)
	if err != nil {
		return CacheKey{}, fmt.Errorf("读取设计源: %w", err)
	}
	d, report := ParseWithOptions(designData, Options{})
	if report.HasErrors() {
		return CacheKey{}, fmt.Errorf("设计源当前未过源校验，缓存不可用: %v", report.Err())
	}
	pkgRoot := input.packageRoot()
	catalog, err := loadMaterialCatalog(pkgRoot, input.catalogRel())
	if err != nil {
		return CacheKey{}, fmt.Errorf("catalog 不可读: %w", err)
	}
	plan := planMaterialAdmission(d, catalog)
	admittedSHAs := make([]string, 0, len(plan.admitted))
	for _, ap := range plan.admitted {
		admittedSHAs = append(admittedSHAs, fileSHA256OrMarker(filepath.Join(pkgRoot, filepath.FromSlash(ap.fileRel))))
	}
	catalogSHA := fileSHA256OrMarker(filepath.Join(pkgRoot, filepath.FromSlash(input.catalogRel())))
	return CacheKey{
		DesignSourceSHA256: sha256Hex(designData),
		CompilerVersion:    input.compilerVersion(),
		AssemblyInputHash:  computeAssemblyInputHash(catalogSHA, admittedSHAs, input.Overrides),
		CardSchemaVersion:  CardSchemaVersion,
	}, nil
}

// buildCompileManifest 构建成功编译的 manifest（卡哈希/W 清单/回执/四元组键）。
func buildCompileManifest(d *Design, cardBytes []byte, input CompileInput, admitted []admittedMaterial, plan admissionPlan, trackMapping []trackMappingRecord, designData []byte, pkgRoot string, r *Report) (*CompileManifest, error) {
	admittedSHAs := make([]string, 0, len(admitted))
	assetRecords := make([]AdmittedAssetRecord, 0, len(admitted))
	for _, a := range admitted {
		admittedSHAs = append(admittedSHAs, a.sha256)
		assetRecords = append(assetRecords, AdmittedAssetRecord{
			MaterialID:    a.material.ID,
			File:          a.fileRel,
			SHA256:        a.sha256,
			CatalogID:     a.catalog.ID,
			CatalogStatus: a.catalog.Status,
			SoundCheck:    a.catalog.SoundCheck,
		})
	}
	excludedRecords := make([]ExcludedMaterialRecord, 0, len(plan.excluded))
	for _, ex := range plan.excluded {
		file := ""
		if ex.material.File != nil {
			file = normalizeMaterialPath(*ex.material.File)
		}
		excludedRecords = append(excludedRecords, ExcludedMaterialRecord{
			MaterialID: ex.material.ID,
			Kind:       ex.material.Kind,
			File:       file,
			Reason:     ex.reason,
		})
	}
	catalogSHA := fileSHA256OrMarker(filepath.Join(pkgRoot, filepath.FromSlash(input.catalogRel())))
	warnings := make([]ManifestDiagnostic, 0)
	for _, w := range r.Warnings() {
		warnings = append(warnings, ManifestDiagnostic{Rule: w.Rule, Path: w.Path, Message: w.Message})
	}
	return &CompileManifest{
		SchemaVersion:       ManifestSchemaVersion,
		MappingTableVersion: MappingTableVersion,
		Key: CacheKey{
			DesignSourceSHA256: sha256Hex(designData),
			CompilerVersion:    input.compilerVersion(),
			AssemblyInputHash:  computeAssemblyInputHash(catalogSHA, admittedSHAs, input.Overrides),
			CardSchemaVersion:  CardSchemaVersion,
		},
		CardFile:   d.ID + ".compiled.level.yaml",
		CardSHA256: sha256Hex(cardBytes),
		Warnings:   warnings,
		Receipt: ManifestReceipt{
			PendingQuestions:  append([]string(nil), d.PendingQuestions...),
			TrackMapping:      trackMapping,
			AdmittedAssets:    assetRecords,
			ExcludedMaterials: excludedRecords,
		},
	}, nil
}

// VerifyCompiled 校验既有编译产物是否可安全复用（§6.5 源为权威 + fail-closed）：
//  1. manifest 存在且 schema/映射表版本匹配（否则缺失/损坏→不可用）；
//  2. 卡文件存在、sha256 与 manifest 一致（手改 compiled 卡→拒绝）；
//  3. 卡能过现行 ValidateLevelCard 且 schema_version 与键一致；
//  4. 按当前输入重算四元组键，与 manifest 一致（否则=缓存失效→须重编译）。
//
// 任一失败均返回错误，调用方必须拒绝使用旧卡（不得降级运行）。
func VerifyCompiled(cardPath, manifestPath string, input CompileInput) (*CompileManifest, error) {
	rawManifest, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("%w: manifest %s 不可读: %v", ErrCompiledMissing, manifestPath, err)
	}
	var manifest CompileManifest
	if err := json.Unmarshal(rawManifest, &manifest); err != nil {
		return nil, fmt.Errorf("%w: manifest 解码失败: %v", ErrCompiledTampered, err)
	}
	if manifest.SchemaVersion != ManifestSchemaVersion {
		return nil, fmt.Errorf("%w: manifest schema_version %q != %q", ErrCompiledTampered, manifest.SchemaVersion, ManifestSchemaVersion)
	}
	if manifest.MappingTableVersion != MappingTableVersion {
		return nil, fmt.Errorf("%w: 映射表版本 %q != 当前 %q（须重编译）", ErrCacheStale, manifest.MappingTableVersion, MappingTableVersion)
	}
	if base := filepath.Base(cardPath); manifest.CardFile != "" && manifest.CardFile != base {
		return nil, fmt.Errorf("%w: manifest.card_file %q != 实际卡文件 %q", ErrCompiledTampered, manifest.CardFile, base)
	}
	cardBytes, err := os.ReadFile(cardPath)
	if err != nil {
		return nil, fmt.Errorf("%w: 卡文件 %s 不可读: %v", ErrCompiledMissing, cardPath, err)
	}
	if sha256Hex(cardBytes) != manifest.CardSHA256 {
		return nil, fmt.Errorf("%w: 卡文件 sha256 与 manifest 不符（手改 compiled 卡无效，§6.5）", ErrCompiledTampered)
	}
	card, err := edu.ParseLevelCard(cardBytes)
	if err != nil {
		return nil, fmt.Errorf("%w: 卡未过现行 ValidateLevelCard: %v", ErrCompiledTampered, err)
	}
	if card.SchemaVersion != manifest.Key.CardSchemaVersion {
		return nil, fmt.Errorf("%w: 卡 schema_version %q != 缓存键 %q", ErrCompiledTampered, card.SchemaVersion, manifest.Key.CardSchemaVersion)
	}
	currentKey, err := BuildCacheKey(input)
	if err != nil {
		return nil, fmt.Errorf("%w: 重算缓存键失败（fail-closed，旧卡不得使用）: %v", ErrCacheStale, err)
	}
	if !currentKey.Equal(manifest.Key) {
		return nil, fmt.Errorf("%w: 四元组键不一致（source=%s→%s compiler=%s assembly=%s card=%s）",
			ErrCacheStale, shortHash(manifest.Key.DesignSourceSHA256), shortHash(currentKey.DesignSourceSHA256),
			manifest.Key.CompilerVersion, manifest.Key.AssemblyInputHash, manifest.Key.CardSchemaVersion)
	}
	return &manifest, nil
}

func shortHash(s string) string {
	if len(s) <= 8 {
		return s
	}
	return s[:8]
}
