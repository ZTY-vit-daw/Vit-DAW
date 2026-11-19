package chat

// W02-A1b：有界课程上下文注入（contextruntime additive 字段）。
//
// 非命令轮且存在活跃乐局时，本面把「当前课程卡 / 节点 / 资产 / 选中 clip /
// 写入边界」投影成**纯内存**的有界 map，写入
// chatContext[contextruntime.LessonContextKey]；命令轮与无乐局轮不写。
// contextruntime.Build 以 lesson_context（omitempty）字段接收并再次 clone/
// 裁剪，因此模型快照能看到当前节点与选中 clip，而整份 design.v1.yaml、notes
// 表、原图和文件路径永远不进上下文。
//
// 上界（与 contextruntime 同源常量）：title ≤200 runes、goal ≤500 runes、
// assets/allowed ≤8；任一被截断时 truncated=true。selected_clip 复用 A1a 的
// 三键解析（卷帘焦点 > 单选 > 多选首项），track 与仓库既有
// conversation.intent.selectedClipTrackID 同序：piano_roll_focus_track_id >
// selected_clip_track_id > focused_track_id > selected_track_id > track_id，
// 避免 clip 与不同轨配对误导模型上下文。

import (
	"strings"

	"vit-daw-agent/internal/contextruntime"
	"vit-daw-agent/internal/edu"
)

// eduJamLessonContext 组装有界课程上下文；纯内存，不读内核。
func eduJamLessonContext(session eduJamSession, card *edu.LevelCard, requestContext map[string]any) map[string]any {
	if card == nil {
		return nil
	}
	truncated := false
	lesson := map[string]any{"schema_version": contextruntime.LessonContextSchemaVersion}

	cardRow := map[string]any{}
	if id := strings.TrimSpace(card.ID); id != "" {
		cardRow["id"] = id
	}
	if title, cut := eduJamLessonText(card.Title, contextruntime.LessonContextTitleMaxRunes); title != "" {
		cardRow["title"] = title
		truncated = truncated || cut
	}
	if len(cardRow) > 0 {
		lesson["card"] = cardRow
	}

	if nodeID := eduJamCurrentNodeID(session, card); nodeID != "" {
		for i := range card.Nodes {
			if card.Nodes[i].ID != nodeID {
				continue
			}
			node := map[string]any{"id": nodeID}
			if title, cut := eduJamLessonText(card.Nodes[i].Title, contextruntime.LessonContextTitleMaxRunes); title != "" {
				node["title"] = title
				truncated = truncated || cut
			}
			if goal, cut := eduJamLessonText(card.Nodes[i].Goal, contextruntime.LessonContextGoalMaxRunes); goal != "" {
				node["goal"] = goal
				truncated = truncated || cut
			}
			lesson["node"] = node
			break
		}
	}

	if assets := eduJamLessonAssets(card.Assets); len(assets) > 0 {
		if len(assets) > contextruntime.LessonContextMaxItems {
			assets = assets[:contextruntime.LessonContextMaxItems]
			truncated = true
		}
		lesson["assets"] = assets
	}

	if clip := eduJamLessonSelectedClip(requestContext); len(clip) > 0 {
		lesson["selected_clip"] = clip
	}

	if allowed := eduJamLessonAllowed(card.WriteBoundary.Allow); len(allowed) > 0 {
		if len(allowed) > contextruntime.LessonContextMaxItems {
			allowed = allowed[:contextruntime.LessonContextMaxItems]
			truncated = true
		}
		lesson["allowed"] = allowed
	}

	if len(lesson) == 1 {
		return nil // schema_version only: nothing to inject
	}
	lesson["truncated"] = truncated
	return lesson
}

// injectEduJamLessonContext 在非命令轮写入有界课程上下文。命令轮/无活跃乐局
// 已在调用点先行返回；chatContext 为 nil 时不动（与拉回注入同型）。
func injectEduJamLessonContext(chatContext map[string]any, session eduJamSession, card *edu.LevelCard) {
	if chatContext == nil {
		return
	}
	if lesson := eduJamLessonContext(session, card, chatContext); len(lesson) > 0 {
		chatContext[contextruntime.LessonContextKey] = lesson
	}
}

// eduJamLessonAssets 只投影资产 id/kind（不写 file 路径、sha256、notes 引用）。
func eduJamLessonAssets(assets []edu.AssetSpec) []map[string]any {
	out := make([]map[string]any, 0, len(assets))
	for _, asset := range assets {
		id := strings.TrimSpace(asset.ID)
		if id == "" {
			continue
		}
		row := map[string]any{"id": id}
		if kind := strings.TrimSpace(asset.Kind); kind != "" {
			row["kind"] = kind
		}
		out = append(out, row)
	}
	return out
}

// eduJamLessonAllowed 投影卡面写入边界 allow 名单（去空白、保序）。
func eduJamLessonAllowed(allow []string) []string {
	out := make([]string, 0, len(allow))
	for _, name := range allow {
		if name = strings.TrimSpace(name); name != "" {
			out = append(out, name)
		}
	}
	return out
}

// eduJamLessonSelectedClip 复用 A1a 的选中 clip 三键解析，并补上 clip 所属轨。
// 无选中返回 nil。
func eduJamLessonSelectedClip(requestContext map[string]any) map[string]any {
	clipID := eduJamRequestSelectedClipID(requestContext)
	if clipID == "" {
		return nil
	}
	row := map[string]any{"clip_id": clipID}
	if trackID := eduJamLessonTrackID(requestContext); trackID != "" {
		row["track_id"] = trackID
	}
	return row
}

// eduJamLessonTrackID 与 conversation.intent.selectedClipTrackID（intent.go:462）
// 同序：卷帘焦点轨 > 选中 clip 所属轨 > 焦点轨 > 选中轨 > 通用轨。契约修正
// （lead 2026-11-19）：此前 selected_track_id 优先会把 clip 配到别的轨。
func eduJamLessonTrackID(requestContext map[string]any) string {
	for _, key := range []string{
		"piano_roll_focus_track_id", "selected_clip_track_id", "focused_track_id",
		"selected_track_id", "track_id",
	} {
		text := strings.TrimSpace(strings.Trim(cleanContextText(requestContext[key]), "\""))
		if text != "" && text != "<nil>" {
			return text
		}
	}
	return ""
}

// eduJamLessonText 按 rune 上界裁剪文本；返回是否发生截断。
func eduJamLessonText(text string, max int) (string, bool) {
	text = strings.TrimSpace(text)
	if text == "" {
		return "", false
	}
	if max <= 0 {
		return text, false
	}
	runes := []rune(text)
	if len(runes) <= max {
		return text, false
	}
	return string(runes[:max]), true
}
