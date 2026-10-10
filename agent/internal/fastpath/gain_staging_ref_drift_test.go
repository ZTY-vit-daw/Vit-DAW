package fastpath

// HYGIENE-FASTPATH-1 挂账①：gain_staging_ref.go 三份"逐字副本"的机械等价守卫。
//
// L1-5-IMPL-C 把三个 gain staging 意图判定函数自 agentloop
// static_mix_gain_staging_context.go 逐字平移到本包（原件原地保留，agentloop
// 继续消费原件；副本仅供本包平移函数依赖）。平移只登记了引用改名
// （messageLoop 前缀剥离）——本测试锁定"除登记改名与空白外零差异"：
// 原件任何后续改动而副本未同步（或反向），在此显式失败并打印双方差异定位。
//
// 口径（对齐卡面）：
//   - 按声明名提取（go/parser），不按行号——行号锚点只是 IMPL-C 时点快照；
//   - 归一仅限 IMPL-C 登记的引用改名 + 空白，其余任何差异=失败；
//   - 发现未登记的 messageLoop 前缀引用=失败（强制显式登记新改名，禁止静默放行）；
//   - 失败消息打印双方差异定位与归一后全文（fail-visible）。
//
// 测试只读两份源码，不写源码树。

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

const (
	gainStagingRefCopyFile     = "gain_staging_ref.go"
	gainStagingRefOriginalFile = "../agentloop/static_mix_gain_staging_context.go"
)

// gainStagingRefRegisteredRenames 是 IMPL-C 登记的引用改名全集（原件 messageLoop
// 命名空间前缀 → 本包导出名）。出现清单之外的引用改名必须先在此显式登记。
var gainStagingRefRegisteredRenames = []struct{ from, to string }{
	{"messageLoopGainStagingExplicitRequest", "GainStagingExplicitRequest"},
	{"messageLoopStaticBalanceIntentReference", "StaticBalanceIntentReference"},
	{"messageLoopGainStagingStrictReferenceIntent", "GainStagingStrictReferenceIntent"},
	{"messageLoopTextHasAny", "TextHasAny"},
}

// gainStagingRefVerbatimPairs 是守卫对象：本包逐字副本 ↔ agentloop 原件声明名。
var gainStagingRefVerbatimPairs = []struct{ copyName, originalName string }{
	{"GainStagingExplicitRequest", "messageLoopGainStagingExplicitRequest"},
	{"StaticBalanceIntentReference", "messageLoopStaticBalanceIntentReference"},
	{"GainStagingStrictReferenceIntent", "messageLoopGainStagingStrictReferenceIntent"},
}

// extractFuncDecl 按声明名提取函数声明体源码（自 func 关键字至收尾大括号，
// 不含 Doc 注释——副本头部的溯源注释不属于逐字契约面）。返回源码文本与
// "文件:行" 锚点。
func extractFuncDecl(t *testing.T, src []byte, displayPath, funcName string) (string, string) {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, displayPath, src, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse %s: %v", displayPath, err)
	}
	for _, decl := range file.Decls {
		funcDecl, ok := decl.(*ast.FuncDecl)
		if !ok || funcDecl.Name.Name != funcName {
			continue
		}
		start := fset.Position(funcDecl.Pos()).Offset
		end := fset.Position(funcDecl.End()).Offset
		if start < 0 || end > len(src) || start >= end {
			t.Fatalf("extract %s from %s: bad offsets [%d,%d)", funcName, displayPath, start, end)
		}
		line := fset.Position(funcDecl.Pos()).Line
		return string(src[start:end]), fmt.Sprintf("%s:%d", displayPath, line)
	}
	t.Fatalf("declaration %q not found in %s（声明被改名/删除时须同步本守卫的登记清单）", funcName, displayPath)
	return "", ""
}

// applyRegisteredRenames 在原件声明文本上执行 IMPL-C 登记的引用改名
// （整词边界替换），并拒绝未登记的 messageLoop 前缀引用。
func applyRegisteredRenames(t *testing.T, originalText string) string {
	t.Helper()
	normalized := originalText
	for _, rename := range gainStagingRefRegisteredRenames {
		pattern := regexp.MustCompile(`\b` + rename.from + `\b`)
		normalized = pattern.ReplaceAllString(normalized, rename.to)
	}
	if strings.Contains(normalized, "messageLoop") {
		t.Fatalf("原件声明含未登记的 messageLoop 前缀引用改名——请先在 gainStagingRefRegisteredRenames 显式登记，再同步副本：\n%s", normalized)
	}
	return normalized
}

// normalizeDeclarationText 空白归一：逐行 trim + 行内空白折叠为单空格 + 丢空行。
// 保留行结构与行内 token 边界——跨边界的 token 增删仍会被检出。
func normalizeDeclarationText(text string) string {
	lines := strings.Split(text, "\n")
	normalized := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.Join(strings.Fields(line), " ")
		if line == "" {
			continue
		}
		normalized = append(normalized, line)
	}
	return strings.Join(normalized, "\n")
}

// compareNormalizedDeclarations 逐字节比对归一后文本；不一致时返回带双方差异
// 定位（首个差异行号 + 上下文窗口）的 fail-visible 错误。
func compareNormalizedDeclarations(want, got string) error {
	if want == got {
		return nil
	}
	wantLines := strings.Split(want, "\n")
	gotLines := strings.Split(got, "\n")
	window := func(lines []string, center int) string {
		lo := center - 2
		if lo < 0 {
			lo = 0
		}
		hi := center + 3
		if hi > len(lines) {
			hi = len(lines)
		}
		return strings.Join(lines[lo:hi], "\n")
	}
	for i := 0; i < len(wantLines) || i < len(gotLines); i++ {
		wantLine, gotLine := "<EOF>", "<EOF>"
		if i < len(wantLines) {
			wantLine = wantLines[i]
		}
		if i < len(gotLines) {
			gotLine = gotLines[i]
		}
		if wantLine != gotLine {
			return fmt.Errorf("首个差异在归一后第 %d 行：\n--- 原件上下文 ---\n%s\n--- 副本上下文 ---\n%s", i+1, window(wantLines, i), window(gotLines, i))
		}
	}
	return fmt.Errorf("归一后文本不等但未定位到差异行（比较器缺陷）")
}

func TestGainStagingRefCopiesStayVerbatimWithOriginal(t *testing.T) {
	copySrc, err := os.ReadFile(gainStagingRefCopyFile)
	if err != nil {
		t.Fatalf("read copy source: %v", err)
	}
	originalSrc, err := os.ReadFile(gainStagingRefOriginalFile)
	if err != nil {
		t.Fatalf("read original source: %v", err)
	}
	for _, pair := range gainStagingRefVerbatimPairs {
		t.Run(pair.copyName, func(t *testing.T) {
			copyText, copyAnchor := extractFuncDecl(t, copySrc, "internal/fastpath/"+gainStagingRefCopyFile, pair.copyName)
			originalText, originalAnchor := extractFuncDecl(t, originalSrc, "internal/agentloop/"+filepath.ToSlash(filepath.Base(gainStagingRefOriginalFile)), pair.originalName)
			t.Logf("对照锚点：原件 %s ↔ 副本 %s", originalAnchor, copyAnchor)

			want := normalizeDeclarationText(applyRegisteredRenames(t, originalText))
			got := normalizeDeclarationText(copyText)
			if err := compareNormalizedDeclarations(want, got); err != nil {
				t.Fatalf("逐字副本漂移：%s ↔ %s 在登记改名+空白归一后不一致：%v\n--- 原件（归一后）---\n%s\n--- 副本（归一后）---\n%s",
					originalAnchor, copyAnchor, err, want, got)
			}
		})
	}
}

// TestGainStagingRefDriftGuardDetectsContentDrift 是守卫自检：比较器对真实内容
// 漂移必须报错且差异可定位（防恒真比较器使本文件退化为无效守卫）。
func TestGainStagingRefDriftGuardDetectsContentDrift(t *testing.T) {
	verbatim := "func f(text string) bool {\n\tif strings.TrimSpace(text) == \"\" {\n\t\treturn false\n\t}\n\treturn hasAny(text, \"b1\")\n}"
	drifted := strings.Replace(verbatim, `hasAny(text, "b1")`, `hasAny(text, "b1", "b 1")`, 1)

	want := normalizeDeclarationText(verbatim)
	got := normalizeDeclarationText(drifted)
	err := compareNormalizedDeclarations(want, got)
	if err == nil {
		t.Fatalf("guard self-check failed: drifted content must be detected")
	}
	if !strings.Contains(err.Error(), "首个差异在归一后第") || !strings.Contains(err.Error(), `"b 1"`) {
		t.Fatalf("guard self-check: drift error must localize the difference, got: %v", err)
	}
}
