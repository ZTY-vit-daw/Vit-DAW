//------------------------------------------------------------------------
// GenShell — T3 chunk 状态模型（vit.genshell.state.v1，件一 §5 schema）
//
// processor 与 controller 各持一份独立实例：processor 持久化权威（getState/
// setState），controller 经 setComponentState 同步。骨架卡字段集完整定义但
// 只有 scene_mode 有宿主可写入口（GUI 业务接线在卡 4）。
//------------------------------------------------------------------------
#pragma once

#include <map>
#include <optional>
#include <string>
#include <vector>

namespace GenShell {

//------------------------------------------------------------------------
struct JobHistoryEntry
{
	std::string job_id;
	std::string terminal_state;
	std::string clip_id;
	std::string take_stack_id;
	std::vector<std::string> artifact_paths;
};

//------------------------------------------------------------------------
struct ShellState
{
	std::string schema_version = "vit.genshell.state.v1";
	std::string selected_template = "song_segment_v1";
	std::map<std::string, std::string> form_draft;
	std::string scene_mode = "in_project";
	// pending_requests：submit_request 载荷**对象**数组（件一 §5/件二 §2）；每元素存
	// 为紧凑 JSON 文本（toJson 时重新以对象形态嵌入；坏文本 fail-closed 跳过）
	std::vector<std::string> pending_request_jsons;
	std::vector<JobHistoryEntry> job_history;
	std::optional<std::string> audition_job_id; // nullopt = JSON null
	std::optional<std::string> audition_take_id;

	// 未知 schema_version 的 fail-closed 语义（件一 §5）：丢弃内容仅保 job_id
	// 引用（骨架实现=整份丢弃取默认，job_history 以 job_id 空引用保留计数）
	static ShellState fromJson (const std::string& json, bool& schemaRejected);
	std::string toJson () const;
};

} // namespace GenShell
