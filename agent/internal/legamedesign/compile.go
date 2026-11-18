package legamedesign

// W01-B 编译器：设计源 vit.legame_design.v0 → 运行时乐局卡 vit.level_card.v0
// + compile manifest（契约 §6.2 映射表 T-1..T-21；§6.3 装配绑定；§6.4 缺项
// 诊断/缺省注入；§6.5 缓存键与 fail-closed 失效）。
//
// 硬约束（卡面 G0-LEGAME-COMPILE-1）：
//   - 编译入口必须先调 ValidateCompileReadiness(d, overrides)（R5 fail-closed 落点）；
//     源校验允许设计期 tempo/budget 显式 null，无覆盖参数时 R5=3E，给覆盖=0E。
//   - R5 之外歧义/缺项一律聚合 E/W/I 返回，禁止静默纠正；产物最终必须通过现行
//     edu.ParseLevelCard（=ValidateLevelCard）终检，不得绕过/复制/弱化该门。
//   - 单向依赖 compile→edu：本包 import edu，edu 不 import 本包。
//   - 表接口：T-14 判定面登记表（JudgementRegistry）/ T-17 CAP-BRIDGE 闭包表
//     （CapabilityBridge）本卡给出表结构与 first-course 登记条目；运行期接线归 W01-C。
//
// 编译期规则号（契约 §6 未给编译规则号，W01-B 落定；§5.2 规则号继续复用）：
//   E-MAP   §6.2 映射无法产出合法卡（T-4 混写/T-11 推导歧义/终检失败/SMF 无法产出
//           判定参照/写边界为空等）
//   E-FLOW  T-16 节点无可编译完成条件（节点不可运行）
//   W-MAP   §6.2/§6.4 缺项模板/默认注入/拆分降级（T-3/T-15/T-18/R6 落点）
// 其余沿用 W-CHK（T-14 判定面降级）、W-MAT（材料准入排除）、E-CAP/E-REF/E-MAT/
// E-REQ（T-12/T-13/T-17 与源校验）、I-NOTE（设计层字段不进卡）。

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"vit-daw-agent/internal/edu"
)

// 编译期版本常量（进入缓存四元组；映射表/模板变更须同步 bump）。
const (
	// CompilerVersion 是编译器版本号（缓存键四元组之一，§6.5）。
	CompilerVersion = "legamedesign-compile.v0"
	// ManifestSchemaVersion 是 compile manifest 的 schema 版本。
	ManifestSchemaVersion = "vit.legame_compile_manifest.v0"
	// MappingTableVersion 是映射表版本（T-1..T-21 + R3/R4/R5/R6 落点）。
	MappingTableVersion = "vit.legame_design.v0.mapping.v0"
	// CardSchemaVersion 是编译目标卡 schema 版本（=edu 现行校验门版本）。
	CardSchemaVersion = edu.LevelCardSchemaVersion
)

// 编译期规则号（见包注释）。
const (
	RuleMapping = "E-MAP"
	RuleFlow    = "E-FLOW"
	RuleDefault = "W-MAP"
)

// defaultCatalogRel 是工程包内材料目录的默认相对路径（TEXTBOOK-ALIGNMENT §3）。
const defaultCatalogRel = "assets/catalog.yaml"

// CompileInput 是编译一次设计源所需的全部装配上下文（零隐式全局态）。
type CompileInput struct {
	// DesignPath 是设计源 YAML 路径（必填；材料相对路径基准=所在目录=工程包根）。
	DesignPath string
	// PackageRoot 是工程包根（空=设计源所在目录）；materials[].file 与 catalog
	// 均相对此根解析（§6.3 第 2 条）。
	PackageRoot string
	// CatalogPath 是 catalog 相对包根路径（空=assets/catalog.yaml）。
	CatalogPath string
	// OutputDir 是编译产物目录（RefreshCompiled/WriteCompiled 使用）。
	OutputDir string
	// Overrides 是 R5 显式覆盖参数（tempo/budget；nil=未提供）。
	Overrides CompileOverrides
	// Judgements 是 T-14 判定面登记表（零值=无登记，全部降级 W-CHK/E-FLOW）。
	Judgements JudgementRegistry
	// Capabilities 是 T-17 CAP-BRIDGE 闭包表（零值=无登记，required 全部 E-CAP）。
	Capabilities CapabilityBridge
	// CompilerVersion 覆盖编译器版本（空=CompilerVersion 常量；测试/灰度用）。
	CompilerVersion string
}

func (in CompileInput) packageRoot() string {
	if strings.TrimSpace(in.PackageRoot) != "" {
		return in.PackageRoot
	}
	return filepath.Dir(in.DesignPath)
}

func (in CompileInput) catalogRel() string {
	if strings.TrimSpace(in.CatalogPath) != "" {
		return in.CatalogPath
	}
	return defaultCatalogRel
}

func (in CompileInput) compilerVersion() string {
	if strings.TrimSpace(in.CompilerVersion) != "" {
		return in.CompilerVersion
	}
	return CompilerVersion
}

// CompileResult 是一次成功编译的产物（零写入；写盘走 WriteCompiled）。
type CompileResult struct {
	Design   *Design
	CardYAML []byte
	Manifest *CompileManifest

	// admitted 保存已准入材料的文件字节（WriteCompiled 复制进包资产区）。
	admitted []admittedMaterial
}

// LevelCard 用现行校验门解析编译产物（供调用方读取卡结构）。
func (res *CompileResult) LevelCard() (*edu.LevelCard, error) {
	if res == nil {
		return nil, fmt.Errorf("legamedesign: nil compile result")
	}
	return edu.ParseLevelCard(res.CardYAML)
}

// absorb 把另一份报告的诊断并入本报告（编译诊断=源校验+R5+映射的聚合）。
func (r *Report) absorb(other *Report) {
	if other == nil {
		return
	}
	r.diags = append(r.diags, other.diags...)
}

// Compile 执行 design→level_card 编译（零写入；E 级存在时返回 nil result）。
//
// 顺序：设计源解析+校验（复用 W01-A 门）→ R5 编译就绪（强制）→ 材料准入/
// MAT-FILE → T-1..T-21 映射 → yaml 序列化 → 现行 ValidateLevelCard 终检。
func Compile(input CompileInput) (*CompileResult, *Report) {
	r := newReport()
	if strings.TrimSpace(input.DesignPath) == "" {
		r.errorf(RuleMapping, "", "CompileInput.DesignPath 为空（须提供设计源路径）")
		return nil, r
	}
	pkgRoot := input.packageRoot()

	designData, err := os.ReadFile(input.DesignPath)
	if err != nil {
		r.errorf(RuleMapping, "design", "读取设计源失败: %v", err)
		return nil, r
	}

	// 1) 源解析+语义校验（W01-A 门；未知字段/枚举/引用/必填段 fail-closed）。
	d, srcReport := ParseWithOptions(designData, Options{})
	r.absorb(srcReport)
	if srcReport.HasErrors() {
		return nil, r
	}

	// 2) R5 编译就绪（硬约束：编译入口必须调用；无覆盖参数时 tempo+2 budget=3E）。
	r.absorb(ValidateCompileReadiness(d, input.Overrides))
	if r.HasErrors() {
		return nil, r
	}

	// 3) 材料准入：catalog 状态 + 可播放类 + file 非空（T-13；未准入→W-MAT 待备料清单）。
	catalog, catalogErr := loadMaterialCatalog(pkgRoot, input.catalogRel())
	if catalogErr != nil {
		r.errorf(RuleMapping, input.catalogRel(), "catalog 读取/解析失败（装配输入非法，fail-closed）: %v", catalogErr)
		return nil, r
	}
	plan := planMaterialAdmission(d, catalog)
	for _, ex := range plan.excluded {
		// file:null 材料已由源校验出 W-MAT（登记未备料），此处只补偿"有文件但未进卡"项。
		if ex.material.File == nil {
			continue
		}
		r.warnf(RuleMatWarn, fmt.Sprintf("materials[%d].file", ex.index), "未进卡资产（待备料清单，§4.5/T-13）：%s", ex.reason)
	}

	// 4) MAT-FILE 收口：读准入文件、sha256、SMF→notes 表（判定参照输入）。
	admitted := make([]admittedMaterial, 0, len(plan.admitted))
	for _, ap := range plan.admitted {
		abs := filepath.Join(pkgRoot, filepath.FromSlash(ap.fileRel))
		data, readErr := os.ReadFile(abs)
		if readErr != nil {
			r.errorf(RuleMat, fmt.Sprintf("materials[%d].file", ap.index), "MAT-FILE：材料文件缺失/不可读（%s，§4.4/§6.3）: %v", ap.fileRel, readErr)
			continue
		}
		notes, midiErr := parseSMFNotes(data)
		if midiErr != nil {
			r.errorf(RuleMapping, fmt.Sprintf("materials[%d].file", ap.index), "SMF 解析失败，无法产出卡 notes 判定参照（T-13）: %v", midiErr)
			continue
		}
		if len(notes) == 0 {
			r.errorf(RuleMapping, fmt.Sprintf("materials[%d].file", ap.index), "SMF 无可解析音符，notes 表为空（卡 v0 notes_ref 表不得为空，T-13）")
			continue
		}
		admitted = append(admitted, admittedMaterial{
			index:    ap.index,
			material: ap.material,
			catalog:  ap.catalog,
			fileRel:  ap.fileRel,
			sha256:   sha256Hex(data),
			data:     data,
			notes:    notes,
		})
		if sc := strings.TrimSpace(ap.catalog.SoundCheck); sc != "" && sc != "performed" {
			r.infof(RuleNote, fmt.Sprintf("materials[%d].file", ap.index), "准入资产 catalog.sound_check=%q（判定面=符号数据；音频听核待运行期，不冒充已完成，§4.4 第 4 条）", sc)
		}
	}
	if r.HasErrors() {
		return nil, r
	}

	// 5) T-1..T-21 映射（产出卡结构；E/W/I 聚合到同一报告）。
	card, trackMapping, mapErr := buildCompiledCard(d, input, admitted, r)
	// 先判定映射诊断（避免把已定位到字段的 E 再包一层 E-MAP），再看内部映射错误。
	if r.HasErrors() {
		return nil, r
	}
	if mapErr != nil {
		r.errorf(RuleMapping, "", "编译映射失败: %v", mapErr)
		return nil, r
	}
	cardBytes, marshalErr := yaml.Marshal(card)
	if marshalErr != nil {
		r.errorf(RuleMapping, "", "卡 yaml 序列化失败: %v", marshalErr)
		return nil, r
	}

	// 6) 终检：编译产物必须通过现行 ValidateLevelCard（单向依赖 compile→edu；
	//    不得绕过/复制/弱化该校验门，§6.1）。
	if _, gateErr := edu.ParseLevelCard(cardBytes); gateErr != nil {
		r.errorf(RuleMapping, "", "编译产物未过现行 ValidateLevelCard 终检（§6.1）: %v", gateErr)
		return nil, r
	}

	// 7) manifest：四元组键 + 卡哈希 + W 清单 + 映射/回执记录（§6.5）。
	manifest, manifestErr := buildCompileManifest(d, cardBytes, input, admitted, plan, trackMapping, designData, pkgRoot, r)
	if manifestErr != nil {
		r.errorf(RuleMapping, "", "compile manifest 构建失败: %v", manifestErr)
		return nil, r
	}

	return &CompileResult{Design: d, CardYAML: cardBytes, Manifest: manifest, admitted: admitted}, r
}

// CompiledPaths 返回某设计源 id 对应的编译产物路径（§6.5 命名）。
func CompiledPaths(outDir, id string) (cardPath, manifestPath string) {
	cardPath = filepath.Join(outDir, id+".compiled.level.yaml")
	manifestPath = filepath.Join(outDir, id+".compile.manifest.json")
	return cardPath, manifestPath
}

// WriteCompiled 把编译产物写入输出目录：先落资产（保留材料相对路径），再落卡，
// 最后落 manifest。manifest 最后写=缓存完整性哨兵：中途失败时既有 manifest 保持旧值/
// 缺失，校验门判定缓存不可用（fail-closed，§6.5）。
func WriteCompiled(res *CompileResult, outDir string) (cardPath, manifestPath string, err error) {
	if res == nil || res.Design == nil || res.Manifest == nil {
		return "", "", fmt.Errorf("legamedesign: WriteCompiled 需要成功编译结果")
	}
	if strings.TrimSpace(outDir) == "" {
		return "", "", fmt.Errorf("legamedesign: WriteCompiled 需要输出目录")
	}
	cardPath, manifestPath = CompiledPaths(outDir, res.Design.ID)
	for _, a := range res.admitted {
		dest := filepath.Join(outDir, filepath.FromSlash(a.fileRel))
		if err := writeFileAtomic(dest, a.data); err != nil {
			return "", "", fmt.Errorf("复制准入资产 %s: %w", a.fileRel, err)
		}
	}
	if err := writeFileAtomic(cardPath, res.CardYAML); err != nil {
		return "", "", fmt.Errorf("写编译卡: %w", err)
	}
	manifestJSON, err := json.MarshalIndent(res.Manifest, "", "  ")
	if err != nil {
		return cardPath, "", fmt.Errorf("manifest json 序列化: %w", err)
	}
	if err := writeFileAtomic(manifestPath, append(manifestJSON, '\n')); err != nil {
		return cardPath, "", fmt.Errorf("写 compile manifest: %w", err)
	}
	return cardPath, manifestPath, nil
}

// RefreshCompiled 是"缓存有效→复用 / 失效→重编译"的幂等入口（§6.5）：
//   - 既有 manifest+卡通过四元组与完整性校验 → 返回 cached=true（不重编译）；
//   - 否则重编译；成功则原子写新卡+新 manifest；失败则**不触碰**既有产物——
//     旧缓存保持校验失效状态=阻断使用（fail-closed，§6.5 失效动作幂等）。
//
// 返回的 Report 在 cached=true 时由 manifest 的 W 清单回填，便于回执对账。
func RefreshCompiled(input CompileInput) (*CompileResult, *Report, bool) {
	r := newReport()
	if strings.TrimSpace(input.OutputDir) == "" {
		r.errorf(RuleMapping, "", "RefreshCompiled 需要 CompileInput.OutputDir")
		return nil, r, false
	}
	designData, readErr := os.ReadFile(input.DesignPath)
	if readErr != nil {
		r.errorf(RuleMapping, "design", "读取设计源失败: %v", readErr)
		return nil, r, false
	}
	d, srcReport := ParseWithOptions(designData, Options{})
	r.absorb(srcReport)
	if srcReport.HasErrors() {
		return nil, r, false
	}
	cardPath, manifestPath := CompiledPaths(input.OutputDir, d.ID)
	if manifest, verifyErr := VerifyCompiled(cardPath, manifestPath, input); verifyErr == nil {
		cardBytes, readCardErr := os.ReadFile(cardPath)
		if readCardErr == nil {
			cachedReport := newReport()
			for _, w := range manifest.Warnings {
				cachedReport.warnf(w.Rule, w.Path, "%s", w.Message)
			}
			return &CompileResult{Design: d, CardYAML: cardBytes, Manifest: manifest}, cachedReport, true
		}
	}
	res, rep := Compile(input)
	if rep.HasErrors() {
		return nil, rep, false
	}
	if _, _, writeErr := WriteCompiled(res, input.OutputDir); writeErr != nil {
		rep.errorf(RuleMapping, "", "写编译产物失败: %v", writeErr)
		return nil, rep, false
	}
	return res, rep, false
}

// ---- 零碎工具 ----

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// fileSHA256OrMarker 返回文件 sha256；文件不可读时返回说明标记（缓存键据此变化→重编译）。
func fileSHA256OrMarker(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return "unreadable:" + filepath.Base(path)
	}
	return sha256Hex(data)
}

func writeFileAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".tmp-*")
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
	if err := os.Chmod(tmpName, 0o644); err != nil {
		os.Remove(tmpName)
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		os.Remove(tmpName)
		return err
	}
	return nil
}
