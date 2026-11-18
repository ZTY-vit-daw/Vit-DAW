package legamedesign

// 编译卡缓存键/装载 helper（W01-C 复用点）：聊天/装配层禁用以 level_id 单键缓存卡
// （H3 审计：level_id 单键会串版本），改用 §6.5 四元组 manifest 键；legacy 卡
// （无 manifest）退化为内容 sha。所有 helper fail-closed：manifest 自洽/卡哈希/
// 现行校验门任一不过即拒绝，不返回"看起来能用"的旧卡。

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"vit-daw-agent/internal/edu"
)

// CompiledCardManifestPath 由编译卡文件名推导同目录 manifest 路径（§6.5 命名）。
// 非编译卡（如 legacy <levels>/<id>/level.yaml）返回空串。
func CompiledCardManifestPath(cardPath string) string {
	base := filepath.Base(cardPath)
	id := strings.TrimSuffix(base, ".compiled.level.yaml")
	if id == base || id == "" {
		return ""
	}
	return filepath.Join(filepath.Dir(cardPath), id+".compile.manifest.json")
}

// CacheKeyID 返回四元组缓存键的稳定标识（进程内卡缓存键=该串；
// 禁用 level_id 单键：同一 level id 的不同编译版本产生不同 CacheKeyID）。
func CacheKeyID(key CacheKey) string {
	payload, _ := json.Marshal(key)
	return "manifest:" + sha256Hex(payload)
}

// CompiledCardCacheKey 计算编译卡的可缓存键并做自洽校验（fail-closed）：
//   - 同目录存在 §6.5 manifest → schema/映射表版本须匹配当前编译器；卡文件 sha256
//     须等于 manifest.card_sha256（手改 compiled 卡→拒绝）；卡须过现行校验门；
//     键=四元组 CacheKey 的稳定标识；
//   - 无 manifest（legacy levels 目录卡）→ 键=卡文件内容 sha256（内容寻址，
//     同 level id 不同版本不互覆）。
func CompiledCardCacheKey(cardPath string) (string, error) {
	manifestPath := CompiledCardManifestPath(cardPath)
	if manifestPath != "" {
		if raw, err := os.ReadFile(manifestPath); err == nil {
			var manifest CompileManifest
			if err := json.Unmarshal(raw, &manifest); err != nil {
				return "", fmt.Errorf("编译卡 manifest 解码失败（fail-closed）: %w", err)
			}
			if manifest.SchemaVersion != ManifestSchemaVersion {
				return "", fmt.Errorf("编译卡 manifest schema %q 非当前 %q（须重编译）", manifest.SchemaVersion, ManifestSchemaVersion)
			}
			if manifest.MappingTableVersion != MappingTableVersion {
				return "", fmt.Errorf("编译卡映射表版本 %q 非当前 %q（须重编译）", manifest.MappingTableVersion, MappingTableVersion)
			}
			cardBytes, err := os.ReadFile(cardPath)
			if err != nil {
				return "", fmt.Errorf("编译卡不可读: %w", err)
			}
			if sha256Hex(cardBytes) != manifest.CardSHA256 {
				return "", fmt.Errorf("%w: 编译卡 sha256 与 manifest 不符（手改卡无效）", ErrCompiledTampered)
			}
			if _, err := edu.ParseLevelCard(cardBytes); err != nil {
				return "", fmt.Errorf("%w: 编译卡未过现行校验门: %v", ErrCompiledTampered, err)
			}
			return CacheKeyID(manifest.Key), nil
		} else if !os.IsNotExist(err) {
			return "", fmt.Errorf("编译卡 manifest 不可读: %w", err)
		}
	}
	cardBytes, err := os.ReadFile(cardPath)
	if err != nil {
		return "", fmt.Errorf("乐局卡不可读: %w", err)
	}
	if _, err := edu.ParseLevelCard(cardBytes); err != nil {
		return "", fmt.Errorf("乐局卡未过现行校验门: %w", err)
	}
	return "file:" + sha256Hex(cardBytes), nil
}

// LoadCompiledCard 以现行校验门加载一张卡文件（含资产存在+sha256 对账）并返回
// 内容/四元组缓存键；fail-closed：卡不可用即 error，不返回半可用状态。
func LoadCompiledCard(cardPath string) (*edu.LevelCard, string, error) {
	key, err := CompiledCardCacheKey(cardPath)
	if err != nil {
		return nil, "", err
	}
	card, err := edu.LoadLevelCard(cardPath)
	if err != nil {
		return nil, "", fmt.Errorf("乐局卡装载失败: %w", err)
	}
	return card, key, nil
}
