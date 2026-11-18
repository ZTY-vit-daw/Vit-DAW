package projectpackage

// W01-C 工程绑定：乐局工程包的启用元数据（R2）与 design→level_card 装配链路。
//
// R2（契约 §3.3）：是否启用乐局由项目包元数据判定；本文件在既有项目包
// manifest/包配置上增加 legame.{enabled,design}（+装配覆盖参数），提供：
//   - 解析/写回/缺省兼容：旧包无段=普通工程（不编译不注入，惰性零行为差异）；
//   - M1 导入：设计源树复制进包（材料相对路径随包）→ legamedesign.Compile →
//     WriteCompiled 产物进包（legame/cards/<id>/）→ 元数据启用；
//   - 重开/移动/revise：缓存键按 424 四元组 manifest 重算（可命中或重编译）；
//     手改卡/manifest 不一致 → fail-closed；revise → 失效重编译新卡生效；
//   - 材料相对路径按设计源所在目录解析（契约 §6.3 第 2 条），MAT-FILE 哈希
//     对账在编译期收口（复用 424 Compile/VerifyCompiled）。
//
// 零真栈：本文件只做文件/进程内装配，不启内核、不碰 webui。

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"vit-daw-agent/internal/edu"
	"vit-daw-agent/internal/legamedesign"
)

const (
	// LegameSchemaVersion 是乐局工程包元数据 schema 版本（独立文件 legame.json）。
	LegameSchemaVersion = "vit.legame_package.v1"
	// LegameConfigFile 是包根下的乐局元数据文件（进 manifest 校验和）。
	LegameConfigFile = "legame.json"
	// LegameDirName 是包内乐局资产目录（设计源树 + 编译卡），随包迁移。
	LegameDirName = "legame"
	// LegameDesignFile 是导入后的设计源固定包内文件名（相对 legame/ 目录）。
	LegameDesignFile = "design.v1.yaml"
	// LegameCardsDirName 是编译卡目录（<id>.compiled.level.yaml + manifest + assets）。
	LegameCardsDirName = "cards"
	// LegameBindingDirName 是工程旁 agent sidecar 目录（与 projectstore 同域）。
	LegameBindingDirName = ".vit_agent"
)

// LegameOverrides 是 R5 装配覆盖参数（随包持久化：重开/移动后重编译仍可复现）。
type LegameOverrides struct {
	TempoBPM    *float64 `json:"tempo_bpm,omitempty"`
	MaxTurns    *int     `json:"max_turns,omitempty"`
	MaxLLMCalls *int     `json:"max_llm_calls,omitempty"`
}

// LegameMetadata 是项目包承载的乐局启用元数据（R2 判定点）。
// Enabled=false（或旧包无段）=普通工程：不编译、不注入、不报错；
// 转为乐局工程 = Enabled=true + Design 指向包内设计源。
type LegameMetadata struct {
	SchemaVersion string          `json:"schema_version,omitempty"`
	Enabled       bool            `json:"enabled"`
	Design        string          `json:"design,omitempty"`
	Overrides     LegameOverrides `json:"overrides,omitempty"`
}

// LegameImportOptions 是 M1 导入（导入已定稿设计源→编译→装配进包）输入。
type LegameImportOptions struct {
	ProjectPath string
	ProjectUUID string
	// SourceDir 是设计源所在目录（材料相对路径基准；导入时整树复制进包）。
	SourceDir string
	// DesignFile 是 SourceDir 内设计源文件名（空=design.v1.yaml）。
	DesignFile string
	// Overrides 是 R5 显式覆盖参数（设计源 tempo/budget 为 null 时必填）。
	Overrides legamedesign.CompileOverrides
	// Judgements/Capabilities 是装配表输入（零值=first-course 登记表）。
	Judgements   legamedesign.JudgementRegistry
	Capabilities legamedesign.CapabilityBridge
}

// LegameImportResult 是一次成功导入的包内相对路径回执（POSIX 斜杠）。
type LegameImportResult struct {
	Enabled     bool
	CardID      string
	DesignRel   string
	CardRel     string
	ManifestRel string
	Warnings    int
	Infos       int
}

// LegameAssemblyResult 是一次装配/重开解析结果。
type LegameAssemblyResult struct {
	Enabled      bool
	CardID       string
	Card         *edu.LevelCard
	CacheKey     legamedesign.CacheKey
	CacheKeyID   string
	CardPath     string
	ManifestPath string
	DesignPath   string
	Cached       bool
	Warnings     int
}

func (m LegameMetadata) normalized() LegameMetadata {
	if strings.TrimSpace(m.SchemaVersion) == "" {
		m.SchemaVersion = LegameSchemaVersion
	}
	return m
}

func (m LegameMetadata) compileOverrides() legamedesign.CompileOverrides {
	return legamedesign.CompileOverrides{
		TempoBPM:    m.Overrides.TempoBPM,
		MaxTurns:    m.Overrides.MaxTurns,
		MaxLLMCalls: m.Overrides.MaxLLMCalls,
	}
}

func overridesFromCompile(o legamedesign.CompileOverrides) LegameOverrides {
	return LegameOverrides{TempoBPM: o.TempoBPM, MaxTurns: o.MaxTurns, MaxLLMCalls: o.MaxLLMCalls}
}

// LegameBindingPath 返回工程旁乐局 binding sidecar 路径：按 project UUID 目录隔离
// （.vit_agent/<uuid>/legame_binding.json，与 projectstore.Agent root 同域），
// 修复旧父目录静态推导 (.vit_agent/legame_binding.json) 的同目录多工程串绑定。
// projectUUID 为空时回退旧父目录路径（UUID 缺省兼容：旧布位脚本/旧调用）。
func LegameBindingPath(projectPath, projectUUID string) string {
	projectPath = strings.TrimSpace(projectPath)
	if projectPath == "" {
		return ""
	}
	projectDir := filepath.Dir(filepath.Clean(projectPath))
	if uuid := safeName(projectUUID); uuid != "" {
		return filepath.Join(projectDir, LegameBindingDirName, uuid, "legame_binding.json")
	}
	return LegameLegacyBindingPath(projectPath)
}

// LegameLegacyBindingPath 是旧版父目录 binding 路径（兼容读取/旧调用）。
func LegameLegacyBindingPath(projectPath string) string {
	projectPath = strings.TrimSpace(projectPath)
	if projectPath == "" {
		return ""
	}
	return filepath.Join(filepath.Dir(filepath.Clean(projectPath)), LegameBindingDirName, "legame_binding.json")
}

// ---- 包内布局 ----

func legameConfigPath(packageDir string) string { return filepath.Join(packageDir, LegameConfigFile) }
func legameDirPath(packageDir string) string    { return filepath.Join(packageDir, LegameDirName) }
func legameDesignPath(packageDir string) string {
	return filepath.Join(legameDirPath(packageDir), LegameDesignFile)
}
func legameCardsPath(packageDir string) string {
	return filepath.Join(legameDirPath(packageDir), LegameCardsDirName)
}
func legameCardDir(packageDir, cardID string) string {
	return filepath.Join(legameCardsPath(packageDir), safeName(cardID))
}

// ---- 元数据读写（legame.json 为权威；manifest.legame 为镜像/旧包兼容） ----

// loadLegameConfig 读包根 legame.json；缺文件→false。
func loadLegameConfig(packageDir string) (LegameMetadata, bool, error) {
	data, err := os.ReadFile(legameConfigPath(packageDir))
	if err != nil {
		if os.IsNotExist(err) {
			return LegameMetadata{}, false, nil
		}
		return LegameMetadata{}, false, err
	}
	var meta LegameMetadata
	if err := json.Unmarshal(data, &meta); err != nil {
		return LegameMetadata{}, false, fmt.Errorf("乐局包元数据 %s 解码失败: %w", LegameConfigFile, err)
	}
	return meta.normalized(), true, nil
}

func writeLegameConfig(packageDir string, meta LegameMetadata) error {
	data, err := json.MarshalIndent(meta.normalized(), "", "  ")
	if err != nil {
		return err
	}
	return atomicWriteJSON(legameConfigPath(packageDir), json.RawMessage(data))
}

// discoverPackageForRead 解析工程对应项目包；projectUUID 为空时退化为"按工程路径
// 找唯一项目包"（激活前/旧调用兼容），找到后以包 manifest 的 ProjectUUID 为准。
func discoverPackageForRead(projectPath, projectUUID string) (string, Manifest, bool, error) {
	projectPath = cleanAbs(projectPath)
	if projectPath == "" {
		return "", Manifest{}, false, errors.New("乐局工程包解析需要工程路径")
	}
	if uuid := safeName(projectUUID); uuid != "" {
		packageDir, manifest, err := Discover(projectPath, uuid)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return "", Manifest{}, false, nil
			}
			return "", Manifest{}, false, err
		}
		return packageDir, manifest, true, nil
	}
	if packageDir, manifest, ok := discoverAnyPackage(projectPath); ok {
		return packageDir, manifest, true, nil
	}
	return "", Manifest{}, false, nil
}

// discoverAnyPackage 按 Discover 的候选顺序找第一个 schema/项目身份自洽的项目包
// （UUID 缺省兼容：调用方尚无 project UUID 时仍能读包元数据）。
func discoverAnyPackage(projectPath string) (string, Manifest, bool) {
	projectPath = cleanAbs(projectPath)
	base := strings.TrimSuffix(filepath.Base(projectPath), filepath.Ext(projectPath))
	projectDir := filepath.Dir(projectPath)
	candidates := []string{
		filepath.Join(projectDir, PortableDirName),
		filepath.Join(projectDir, base+LoosePackageSuffix),
	}
	if strings.EqualFold(filepath.Base(projectDir), base+LoosePackageSuffix) {
		candidates = append(candidates, projectDir)
	}
	for _, candidate := range candidates {
		manifest := Manifest{}
		if err := readJSON(filepath.Join(candidate, ManifestFile), &manifest); err != nil {
			continue
		}
		if manifest.SchemaVersion != SchemaVersion || safeName(manifest.ProjectUUID) == "" {
			continue
		}
		return candidate, manifest, true
	}
	return "", Manifest{}, false
}

// LoadLegameMetadata 读取工程包乐局元数据（R2 启用判定）。
// packageFound=false=无项目包（普通工程；不编译不注入）；packageFound=true 且
// Enabled=false 或旧包无段 → 普通工程。legacy 兼容：manifest.legame 镜像可读。
func LoadLegameMetadata(projectPath, projectUUID string) (LegameMetadata, bool, error) {
	packageDir, manifest, found, err := discoverPackageForRead(projectPath, projectUUID)
	if err != nil || !found {
		return LegameMetadata{}, false, err
	}
	if meta, ok, err := loadLegameConfig(packageDir); err != nil {
		return LegameMetadata{}, true, err
	} else if ok {
		return meta, true, nil
	}
	if manifest.Legame != nil {
		return manifest.Legame.normalized(), true, nil
	}
	return LegameMetadata{SchemaVersion: LegameSchemaVersion}, true, nil
}

// validateLegameMetadata 校验元数据（fail-closed，不静默纠正）：
//   - Enabled=true：Design 必填、包内相对路径、不得越出包根、文件必须存在；
//   - 覆盖参数若给：tempo>0 有限数、budget 两值 ≥1；
//   - Enabled=false：Design 若给仍必须是合法包内相对路径（保留再启用路径）。
func validateLegameMetadata(packageDir string, meta *LegameMetadata) error {
	meta.SchemaVersion = firstNonEmpty(strings.TrimSpace(meta.SchemaVersion), LegameSchemaVersion)
	if meta.SchemaVersion != LegameSchemaVersion {
		return fmt.Errorf("乐局包元数据 schema %q 非当前 %q", meta.SchemaVersion, LegameSchemaVersion)
	}
	if o := meta.Overrides; o.TempoBPM != nil && !(*o.TempoBPM > 0) {
		return fmt.Errorf("乐局装配覆盖参数 tempo_bpm 须 >0（got %v）", *o.TempoBPM)
	}
	if o := meta.Overrides; o.MaxTurns != nil && *o.MaxTurns < 1 {
		return fmt.Errorf("乐局装配覆盖参数 max_turns 须 ≥1（got %d）", *o.MaxTurns)
	}
	if o := meta.Overrides; o.MaxLLMCalls != nil && *o.MaxLLMCalls < 1 {
		return fmt.Errorf("乐局装配覆盖参数 max_llm_calls 须 ≥1（got %d）", *o.MaxLLMCalls)
	}
	design := strings.TrimSpace(meta.Design)
	if design == "" {
		if meta.Enabled {
			return errors.New("乐局工程启用（enabled=true）须同时声明 design（包内设计源相对路径）")
		}
		return nil
	}
	rel, err := legamePackageRelPath(packageDir, design)
	if err != nil {
		return err
	}
	meta.Design = rel
	abs := filepath.Join(packageDir, filepath.FromSlash(rel))
	info, statErr := os.Stat(abs)
	if statErr != nil || info.IsDir() {
		return fmt.Errorf("乐局设计源 %q 不在项目包内或不可读: %v", rel, statErr)
	}
	return nil
}

// legamePackageRelPath 归一/校验包内相对路径（POSIX 斜杠；禁绝对路径/.. 越界）。
func legamePackageRelPath(packageDir, rel string) (string, error) {
	cleaned := strings.ReplaceAll(strings.TrimSpace(rel), "\\", "/")
	cleaned = strings.TrimPrefix(cleaned, "./")
	if cleaned == "" || filepath.IsAbs(cleaned) || strings.HasPrefix(cleaned, "/") {
		return "", fmt.Errorf("乐局设计源须为包内相对路径（got %q）", rel)
	}
	slashPath := filepath.ToSlash(filepath.Clean(filepath.FromSlash(cleaned)))
	for _, segment := range strings.Split(slashPath, "/") {
		if segment == ".." {
			return "", fmt.Errorf("乐局设计源路径不得越出包根（got %q）", rel)
		}
	}
	if strings.HasPrefix(slashPath, "../") {
		return "", fmt.Errorf("乐局设计源路径不得越出包根（got %q）", rel)
	}
	_ = packageDir
	return slashPath, nil
}

// SaveLegameMetadata 写回乐局启用元数据（包不存在时新建 loose sidecar 项目包）。
func SaveLegameMetadata(projectPath, projectUUID string, meta LegameMetadata) error {
	projectPath = cleanAbs(projectPath)
	uuid := safeName(projectUUID)
	if projectPath == "" || uuid == "" {
		return errors.New("保存乐局包元数据需要工程路径与 project UUID")
	}
	packageDir, manifest, _, err := discoverPackageForRead(projectPath, uuid)
	if err != nil {
		return err
	}
	if packageDir == "" {
		if packageDir, err = ensureProjectPackage(projectPath, uuid); err != nil {
			return err
		}
		if _, manifest, err = Discover(projectPath, uuid); err != nil {
			return err
		}
	}
	return saveLegameMetadataInPackage(packageDir, &manifest, meta)
}

func saveLegameMetadataInPackage(packageDir string, manifest *Manifest, meta LegameMetadata) error {
	if err := validateLegameMetadata(packageDir, &meta); err != nil {
		return err
	}
	if err := writeLegameConfig(packageDir, meta); err != nil {
		return err
	}
	stored := meta
	manifest.Legame = &stored
	manifest.UpdatedAt = time.Now().UTC()
	return writeManifest(packageDir, *manifest)
}

// ensureProjectPackage 确保 loose sidecar 项目包存在（无则建最小 manifest）。
// 不触碰 history/其他包内容；供 M1 导入在未保存过的工程上落装配产物。
func ensureProjectPackage(projectPath, projectUUID string) (string, error) {
	if packageDir, _, err := Discover(projectPath, projectUUID); err == nil {
		return packageDir, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	packageDir := PackagePath(projectPath, projectUUID, false)
	if err := os.MkdirAll(packageDir, 0o755); err != nil {
		return "", err
	}
	now := time.Now().UTC()
	manifest := Manifest{
		SchemaVersion: SchemaVersion, ProjectUUID: safeName(projectUUID),
		OriginProjectUUID: safeName(projectUUID), ProjectPath: cleanAbs(projectPath),
		ProjectFile: filepath.Base(projectPath), GenerationID: "legame_import",
		SaveKind: "legame_import", PackageMode: "loose_sidecar", CreatedAt: now, UpdatedAt: now,
	}
	if err := writeManifest(packageDir, manifest); err != nil {
		return "", err
	}
	return packageDir, nil
}

// ---- M1 导入 / 重开 / revise ----

// ImportLegameDesign 是 M1 导入：设计源树复制进包 → 编译 → 产物进包 → 元数据启用。
// 任何 E 级诊断如实回执且**不写启用元数据**（fail-closed，不静默失败/不半启用）。
func ImportLegameDesign(opts LegameImportOptions) (*LegameImportResult, *legamedesign.Report, error) {
	projectPath := cleanAbs(opts.ProjectPath)
	uuid := safeName(opts.ProjectUUID)
	if projectPath == "" || uuid == "" {
		return nil, nil, errors.New("乐局设计源导入需要工程路径与 project UUID")
	}
	if info, err := os.Stat(projectPath); err != nil || info.IsDir() {
		return nil, nil, fmt.Errorf("工程文件不存在: %s", projectPath)
	}
	sourceDir := cleanAbs(opts.SourceDir)
	if info, err := os.Stat(sourceDir); err != nil || !info.IsDir() {
		return nil, nil, fmt.Errorf("设计源目录不存在: %s", sourceDir)
	}
	designName := strings.TrimSpace(opts.DesignFile)
	if designName == "" {
		designName = LegameDesignFile
	}
	sourceDesign := filepath.Join(sourceDir, designName)
	if info, err := os.Stat(sourceDesign); err != nil || info.IsDir() {
		return nil, nil, fmt.Errorf("设计源文件不存在: %s", sourceDesign)
	}
	packageDir, err := ensureProjectPackage(projectPath, uuid)
	if err != nil {
		return nil, nil, err
	}

	_, manifest, err := Discover(projectPath, uuid)
	if err != nil {
		return nil, nil, err
	}

	staging := filepath.Join(packageDir, ".legame_import_"+shortID())
	if pathWithin(staging, sourceDir) || samePath(sourceDir, packageDir) {
		return nil, nil, errors.New("乐局设计源目录包含导入 staging（不得导入工程包快照/自身目录）")
	}
	if err := os.MkdirAll(staging, 0o755); err != nil {
		return nil, nil, err
	}
	defer os.RemoveAll(staging)
	if err := copyDir(sourceDir, staging); err != nil {
		return nil, nil, fmt.Errorf("复制设计源树失败: %w", err)
	}
	stagingDesign := filepath.Join(staging, LegameDesignFile)
	if err := atomicCopyFile(sourceDesign, stagingDesign); err != nil {
		return nil, nil, fmt.Errorf("落盘设计源副本失败: %w", err)
	}
	designBytes, err := os.ReadFile(stagingDesign)
	if err != nil {
		return nil, nil, err
	}
	design, report := legamedesign.Parse(designBytes)
	if report.HasErrors() {
		return nil, report, fmt.Errorf("乐局设计源解析失败（fail-closed，未写入工程包）: %v", report.Err())
	}
	cardID := design.ID
	if safeName(cardID) != cardID || cardID == "" {
		return nil, report, fmt.Errorf("乐局卡 id %q 不是目录安全名（[A-Za-z0-9_-]，卡装载按 <dir>/<id> 约定）", cardID)
	}
	cardOutputDir := filepath.Join(staging, LegameCardsDirName, safeName(cardID))
	input := legamedesign.CompileInput{
		DesignPath:   stagingDesign,
		PackageRoot:  staging,
		OutputDir:    cardOutputDir,
		Overrides:    opts.Overrides,
		Judgements:   resolvedJudgements(opts.Judgements),
		Capabilities: resolvedCapabilities(opts.Capabilities),
	}
	compiled, report := legamedesign.Compile(input)
	if report.HasErrors() {
		return nil, report, fmt.Errorf("乐局设计源编译失败（fail-closed，未写入工程包）: %v", report.Err())
	}
	if _, _, err := legamedesign.WriteCompiled(compiled, cardOutputDir); err != nil {
		return nil, report, fmt.Errorf("乐局编译产物写入失败: %w", err)
	}
	cardPath, manifestPath := legamedesign.CompiledPaths(cardOutputDir, cardID)
	if _, err := legamedesign.VerifyCompiled(cardPath, manifestPath, input); err != nil {
		return nil, report, fmt.Errorf("乐局编译产物自检失败（fail-closed）: %w", err)
	}
	if _, err := edu.LoadLevelCard(cardPath); err != nil {
		return nil, report, fmt.Errorf("乐局编译产物未过现行 LoadLevelCard（fail-closed）: %w", err)
	}

	// 装配目录原子替换进包：legame/{design.v1.yaml, assets/, cards/<id>/...}
	if err := atomicReplaceDir(staging, legameDirPath(packageDir)); err != nil {
		return nil, report, fmt.Errorf("乐局装配目录进包失败: %w", err)
	}
	meta := LegameMetadata{
		SchemaVersion: LegameSchemaVersion,
		Enabled:       true,
		Design:        LegameDirName + "/" + LegameDesignFile,
		Overrides:     overridesFromCompile(opts.Overrides),
	}
	if err := saveLegameMetadataInPackage(packageDir, &manifest, meta); err != nil {
		return nil, report, fmt.Errorf("乐局启用元数据写入失败: %w", err)
	}
	_, warnings, infos := report.Counts()
	return &LegameImportResult{
		Enabled:     true,
		CardID:      cardID,
		DesignRel:   meta.Design,
		CardRel:     filepath.ToSlash(filepath.Join(LegameDirName, LegameCardsDirName, safeName(cardID), filepath.Base(cardPath))),
		ManifestRel: filepath.ToSlash(filepath.Join(LegameDirName, LegameCardsDirName, safeName(cardID), filepath.Base(manifestPath))),
		Warnings:    warnings,
		Infos:       infos,
	}, report, nil
}

func resolvedJudgements(reg legamedesign.JudgementRegistry) legamedesign.JudgementRegistry {
	if reg.Deterministic == nil && reg.UserJudgment == nil {
		return legamedesign.FirstCourseJudgementRegistry()
	}
	return reg
}

func resolvedCapabilities(bridge legamedesign.CapabilityBridge) legamedesign.CapabilityBridge {
	if bridge.Closures == nil {
		return legamedesign.FirstCourseCapabilityBridge()
	}
	return bridge
}

// legameCompileInput 由包元数据构造编译器输入（材料相对设计源目录；产物目录
// legame/cards/<card id>）。设计源当前不合法时返回其 E 报告（fail-closed）。
func legameCompileInput(packageDir string, meta LegameMetadata) (legamedesign.CompileInput, string, *legamedesign.Report, error) {
	if !meta.Enabled {
		return legamedesign.CompileInput{}, "", nil, errors.New("乐局工程未启用（legame.enabled=false），不编译")
	}
	designRel, err := legamePackageRelPath(packageDir, meta.Design)
	if err != nil {
		return legamedesign.CompileInput{}, "", nil, err
	}
	designPath := filepath.Join(packageDir, filepath.FromSlash(designRel))
	designBytes, err := os.ReadFile(designPath)
	if err != nil {
		return legamedesign.CompileInput{}, "", nil, fmt.Errorf("乐局设计源不可读: %w", err)
	}
	design, report := legamedesign.Parse(designBytes)
	if report.HasErrors() {
		return legamedesign.CompileInput{}, "", report, fmt.Errorf("乐局设计源当前不可编译（fail-closed）: %v", report.Err())
	}
	cardID := design.ID
	if safeName(cardID) != cardID || cardID == "" {
		return legamedesign.CompileInput{}, "", report, fmt.Errorf("乐局卡 id %q 不是目录安全名（[A-Za-z0-9_-]，卡装载按 <dir>/<id> 约定）", cardID)
	}
	outputDir := legameCardDir(packageDir, cardID)
	return legamedesign.CompileInput{
		DesignPath:   designPath,
		PackageRoot:  filepath.Dir(designPath),
		OutputDir:    outputDir,
		Overrides:    meta.compileOverrides(),
		Judgements:   legamedesign.FirstCourseJudgementRegistry(),
		Capabilities: legamedesign.FirstCourseCapabilityBridge(),
	}, cardID, report, nil
}

// LoadLegameCard 读取工程包编译卡（重开/移动/revise 入口）：
//   - 普通工程（无包/未启用）→ Enabled=false，零编译零注入（惰性）；
//   - 缓存四元组校验通过 → 直接装载（移动后相对路径不变→命中）；
//   - 键变化/手改卡/manifest 不一致 → 重编译；重编译 E → 返回报告与错误
//     （旧卡保留但校验不通过=阻断使用，fail-closed）。
func LoadLegameCard(projectPath, projectUUID string) (*LegameAssemblyResult, *legamedesign.Report, error) {
	meta, found, err := LoadLegameMetadata(projectPath, projectUUID)
	if err != nil {
		return nil, nil, err
	}
	if !found || !meta.Enabled {
		return &LegameAssemblyResult{Enabled: false}, nil, nil
	}
	packageDir, _, found, err := discoverPackageForRead(projectPath, projectUUID)
	if err != nil || !found {
		return nil, nil, fmt.Errorf("乐局工程已启用但项目包不可解析（fail-closed）: %v", err)
	}
	input, cardID, report, err := legameCompileInput(packageDir, meta)
	if err != nil {
		return nil, report, fmt.Errorf("乐局设计源当前不可编译（fail-closed）: %w", err)
	}
	cardPath, manifestPath := legamedesign.CompiledPaths(input.OutputDir, cardID)
	if _, verifyErr := legamedesign.VerifyCompiled(cardPath, manifestPath, input); verifyErr == nil {
		return assembleLegameCard(input, cardID, cardPath, manifestPath, true)
	}
	return RefreshLegameAssembly(projectPath, projectUUID)
}

// RefreshLegameAssembly 是缓存失效→重编译入口（revise/重开/移动共用）：
// 成功后写回包内产物并刷新项目包 manifest 校验和；失败保留旧产物（不删除），
// 但调用方必须视为不可用（fail-closed）。
func RefreshLegameAssembly(projectPath, projectUUID string) (*LegameAssemblyResult, *legamedesign.Report, error) {
	meta, found, err := LoadLegameMetadata(projectPath, projectUUID)
	if err != nil {
		return nil, nil, err
	}
	if !found || !meta.Enabled {
		return &LegameAssemblyResult{Enabled: false}, nil, nil
	}
	packageDir, manifest, found, err := discoverPackageForRead(projectPath, projectUUID)
	if err != nil || !found {
		return nil, nil, fmt.Errorf("乐局工程已启用但项目包不可解析（fail-closed）: %v", err)
	}
	input, cardID, report, err := legameCompileInput(packageDir, meta)
	if err != nil {
		return nil, report, fmt.Errorf("乐局设计源当前不可编译（fail-closed）: %w", err)
	}
	result, report, cached := legamedesign.RefreshCompiled(input)
	if report.HasErrors() {
		return nil, report, fmt.Errorf("乐局重编译失败（旧卡保留但不得使用，fail-closed）: %v", report.Err())
	}
	if result == nil {
		return nil, report, errors.New("乐局重编译未返回产物（fail-closed）")
	}
	cleanupStaleCardDirs(packageDir, cardID)
	manifest.Legame = &meta
	manifest.UpdatedAt = time.Now().UTC()
	if err := writeManifest(packageDir, manifest); err != nil {
		return nil, report, fmt.Errorf("乐局装配后项目包 manifest 刷新失败: %w", err)
	}
	cardPath, manifestPath := legamedesign.CompiledPaths(input.OutputDir, cardID)
	assembled, report, err := assembleLegameCard(input, cardID, cardPath, manifestPath, cached)
	return assembled, report, err
}

// assembleLegameCard 装载编译卡并回填四元组缓存键（fail-closed）。
func assembleLegameCard(input legamedesign.CompileInput, cardID, cardPath, manifestPath string, cached bool) (*LegameAssemblyResult, *legamedesign.Report, error) {
	raw, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, nil, fmt.Errorf("乐局 compile manifest 不可读（fail-closed）: %w", err)
	}
	var compiled legamedesign.CompileManifest
	if err := json.Unmarshal(raw, &compiled); err != nil {
		return nil, nil, fmt.Errorf("乐局 compile manifest 解码失败（fail-closed）: %w", err)
	}
	card, err := edu.LoadLevelCard(cardPath)
	if err != nil {
		return nil, nil, fmt.Errorf("乐局编译卡装载失败（fail-closed）: %w", err)
	}
	return &LegameAssemblyResult{
		Enabled:      true,
		CardID:       cardID,
		Card:         card,
		CacheKey:     compiled.Key,
		CacheKeyID:   legamedesign.CacheKeyID(compiled.Key),
		CardPath:     cardPath,
		ManifestPath: manifestPath,
		DesignPath:   input.DesignPath,
		Cached:       cached,
		Warnings:     len(compiled.Warnings),
	}, nil, nil
}

// cleanupStaleCardDirs 删除 cards/ 下非当前 card id 的目录（revise 改 id 后不留孤儿）。
func cleanupStaleCardDirs(packageDir, keepCardID string) {
	cardsRoot := legameCardsPath(packageDir)
	entries, err := os.ReadDir(cardsRoot)
	if err != nil {
		return
	}
	keep := safeName(keepCardID)
	for _, entry := range entries {
		if !entry.IsDir() || entry.Name() == keep {
			continue
		}
		_ = os.RemoveAll(filepath.Join(cardsRoot, entry.Name()))
	}
}

// carryLegameIntoPrepare 把既有项目包的乐局装配内容（设计源树/编译卡/元数据）
// 带进新的 prepared 包：保存（含另存）后乐局工程仍是乐局工程，不丢装配。
func carryLegameIntoPrepare(packageDir, projectPath, projectUUID string) (*LegameMetadata, error) {
	previousDir, previousManifest, err := Discover(projectPath, projectUUID)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	meta, found, err := loadLegameConfig(previousDir)
	if err != nil {
		return nil, err
	}
	if !found && previousManifest.Legame != nil {
		meta, found = previousManifest.Legame.normalized(), true
	}
	if !found {
		return nil, nil
	}
	previousLegameDir := legameDirPath(previousDir)
	if dirExists(previousLegameDir) {
		if err := copyDir(previousLegameDir, legameDirPath(packageDir)); err != nil {
			return nil, fmt.Errorf("携带乐局装配目录失败: %w", err)
		}
	}
	if err := writeLegameConfig(packageDir, meta); err != nil {
		return nil, err
	}
	stored := meta
	return &stored, nil
}
