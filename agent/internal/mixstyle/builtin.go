package mixstyle

import (
	"embed"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

//go:embed styles/*.vms
var builtinFiles embed.FS

var builtinAliases = map[string][]string{
	"modern_pop":       {"modern pop", "pop", "流行", "现代流行", "當代流行"},
	"indie_rock":       {"indie rock", "indie", "rock", "独立摇滚", "獨立搖滾", "摇滚", "搖滾"},
	"edm_pop":          {"edm pop", "edm", "dance pop", "electronic dance", "电子舞曲", "電子舞曲", "舞曲"},
	"cinematic_ballad": {"cinematic ballad", "ballad", "cinematic", "影视抒情", "影視抒情", "抒情", "电影感", "電影感"},
	"neutral":          {"neutral", "通用", "中性"},
}

func Builtin(id string) (MixStyle, error) {
	id = strings.ToLower(strings.TrimSpace(id))
	if id == "" {
		id = "neutral"
	}
	data, err := builtinFiles.ReadFile(filepath.ToSlash("styles/" + id + FileExtension))
	if err != nil {
		return MixStyle{}, fmt.Errorf("unknown built-in mix style %q", id)
	}
	var style MixStyle
	if err := json.Unmarshal(data, &style); err != nil {
		return MixStyle{}, fmt.Errorf("decode built-in mix style %q: %w", id, err)
	}
	if err := Validate(style); err != nil {
		return MixStyle{}, err
	}
	return Normalize(style), nil
}

func Default() MixStyle {
	style, _ := Builtin("neutral")
	return style
}

func Match(userText string) (MixStyle, bool) {
	text := strings.ToLower(strings.TrimSpace(userText))
	if text != "" {
		type matchAlias struct{ id, alias string }
		aliases := []matchAlias{}
		for id, values := range builtinAliases {
			for _, alias := range values {
				aliases = append(aliases, matchAlias{id: id, alias: strings.ToLower(alias)})
			}
		}
		sort.SliceStable(aliases, func(i, j int) bool {
			if len([]rune(aliases[i].alias)) == len([]rune(aliases[j].alias)) {
				return aliases[i].id < aliases[j].id
			}
			return len([]rune(aliases[i].alias)) > len([]rune(aliases[j].alias))
		})
		for _, candidate := range aliases {
			if strings.Contains(text, candidate.alias) {
				style, err := Builtin(candidate.id)
				return style, err == nil
			}
		}
	}
	return Default(), false
}

func IDs() []string {
	entries, _ := builtinFiles.ReadDir("styles")
	ids := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != FileExtension {
			continue
		}
		ids = append(ids, strings.TrimSuffix(entry.Name(), FileExtension))
	}
	sort.Strings(ids)
	return ids
}
