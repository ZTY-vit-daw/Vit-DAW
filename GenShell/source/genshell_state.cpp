//------------------------------------------------------------------------
// GenShell — T3 chunk JSON 编解码实现（nlohmann 单头，thirdparty/json）
//------------------------------------------------------------------------
#include "genshell_state.h"

#include "../../thirdparty/json/nlohmann_json.hpp"

namespace GenShell {

using nlohmann::json;

//------------------------------------------------------------------------
std::string ShellState::toJson () const
{
	json j;
	j["schema_version"] = schema_version;
	j["selected_template"] = selected_template;
	json draft = json::object ();
	for (const auto& [k, v] : form_draft)
		draft[k] = v;
	j["form_draft"] = std::move (draft);
	j["scene_mode"] = scene_mode;
	json pendings = json::array ();
	for (const auto& prText : pending_request_jsons)
	{
		try
		{
			auto pr = json::parse (prText);
			if (pr.is_object ())
				pendings.push_back (std::move (pr));
		}
		catch (const json::exception&)
		{
			// 坏载荷文本：跳过（fail-closed 不静默破坏整份 chunk）
		}
	}
	j["pending_requests"] = std::move (pendings);
	json history = json::array ();
	for (const auto& e : job_history)
	{
		json he;
		he["job_id"] = e.job_id;
		he["terminal_state"] = e.terminal_state;
		he["clip_id"] = e.clip_id;
		he["take_stack_id"] = e.take_stack_id;
		he["artifact_paths"] = e.artifact_paths;
		history.push_back (std::move (he));
	}
	j["job_history"] = std::move (history);
	json ref;
	ref["job_id"] = audition_job_id ? json (*audition_job_id) : json (nullptr);
	ref["take_id"] = audition_take_id ? json (*audition_take_id) : json (nullptr);
	j["audition_buffer_ref"] = std::move (ref);
	return j.dump ();
}

//------------------------------------------------------------------------
ShellState ShellState::fromJson (const std::string& jsonText, bool& schemaRejected)
{
	schemaRejected = false;
	ShellState s;
	json parsed;
	try
	{
		parsed = json::parse (jsonText);
	}
	catch (const json::exception&)
	{
		// 坏 JSON：丢弃整份取默认（fail-closed，不静默半载）
		schemaRejected = true;
		return s;
	}
	if (!parsed.is_object ())
	{
		schemaRejected = true;
		return s;
	}
	const auto version = parsed.value ("schema_version", std::string ());
	if (version != s.schema_version)
	{
		// 未知 schema_version：丢弃内容、保默认；GUI 提示由编辑器接线卡处理
		schemaRejected = true;
		return s;
	}
	s.selected_template = parsed.value ("selected_template", s.selected_template);
	if (parsed.contains ("form_draft") && parsed["form_draft"].is_object ())
	{
		for (auto it = parsed["form_draft"].begin (); it != parsed["form_draft"].end (); ++it)
			if (it.value ().is_string ())
				s.form_draft[it.key ()] = it.value ().get<std::string> ();
	}
	s.scene_mode = parsed.value ("scene_mode", s.scene_mode);
	if (parsed.contains ("pending_requests") && parsed["pending_requests"].is_array ())
	{
		for (const auto& pr : parsed["pending_requests"])
		{
			if (pr.is_object ())
				s.pending_request_jsons.push_back (pr.dump ()); // 载荷对象以紧凑文本保存
			else if (pr.is_string ())
				s.pending_request_jsons.push_back (pr.get<std::string> ()); // 兼容字符串形态
		}
	}
	if (parsed.contains ("job_history") && parsed["job_history"].is_array ())
	{
		for (const auto& he : parsed["job_history"])
		{
			if (!he.is_object ())
				continue;
			JobHistoryEntry e;
			e.job_id = he.value ("job_id", std::string ());
			e.terminal_state = he.value ("terminal_state", std::string ());
			e.clip_id = he.value ("clip_id", std::string ());
			e.take_stack_id = he.value ("take_stack_id", std::string ());
			if (he.contains ("artifact_paths") && he["artifact_paths"].is_array ())
				for (const auto& p : he["artifact_paths"])
					if (p.is_string ())
						e.artifact_paths.push_back (p.get<std::string> ());
			s.job_history.push_back (std::move (e));
		}
	}
	if (parsed.contains ("audition_buffer_ref") && parsed["audition_buffer_ref"].is_object ())
	{
		const auto& ref = parsed["audition_buffer_ref"];
		if (ref.contains ("job_id") && ref["job_id"].is_string ())
			s.audition_job_id = ref["job_id"].get<std::string> ();
		if (ref.contains ("take_id") && ref["take_id"].is_string ())
			s.audition_take_id = ref["take_id"].get<std::string> ();
	}
	return s;
}

} // namespace GenShell
