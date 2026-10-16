//------------------------------------------------------------------------
// GenShell — UTF-8 ↔ UTF-16 转换 + 值名表（自实现，不依赖 SDK 版本差异）
//------------------------------------------------------------------------
#include "genshell_cids.h"
#include "genshell_convert.h"

#include <cstdint>

namespace Steinberg {
namespace Vst {

//------------------------------------------------------------------------
// 值名表（件一 §3 / 件二 §1 §3）
//------------------------------------------------------------------------
const char16_t* kSceneModeNames[kSceneModeCount] = {
	u"in_project", u"free_creation", u"experiment_control"};

const char16_t* const kJobStateNames[8] = {u"idle", u"queued", u"running", u"downloading",
                                           u"landing", u"done", u"failed", u"cancelled"};

const char16_t* const kJobErrorClassNames[15] = {
	u"none", u"config_error", u"auth_error", u"quota_error", u"rate_limit", u"not_found",
	u"provider_error", u"moderation_error", u"timeout", u"network_error", u"response_parse_error",
	u"unsupported_format", u"landing_error", u"context_error", u"interrupted"};

int32 sceneModeIndexFromName (const char* nameUtf8)
{
	if (nameUtf8 == nullptr)
		return -1;
	for (int32 i = 0; i < kSceneModeCount; ++i)
	{
		std::u16string wide (kSceneModeNames[i]);
		std::string narrow = GenShell::utf16ToUtf8 (wide);
		if (narrow == nameUtf8)
			return i;
	}
	return -1;
}

const char* sceneModeNameFromIndex (int32 index)
{
	static const char* names[kSceneModeCount] = {"in_project", "free_creation", "experiment_control"};
	if (index < 0 || index >= kSceneModeCount)
		return names[0]; // fail-closed 默认
	return names[index];
}

} // namespace Vst
} // namespace Steinberg

namespace GenShell {

//------------------------------------------------------------------------
void utf8ToUtf16 (const std::string& utf8, std::u16string& out)
{
	out.clear ();
	out.reserve (utf8.size ());
	size_t i = 0;
	const auto n = utf8.size ();
	while (i < n)
	{
		const auto c0 = static_cast<uint8_t> (utf8[i]);
		char32_t cp = 0xFFFD; // 无效码位替换字符
		size_t len = 1;
		if (c0 < 0x80)
		{
			cp = c0;
			len = 1;
		}
		else if ((c0 & 0xE0) == 0xC0 && i + 1 < n)
		{
			cp = (static_cast<char32_t> (c0 & 0x1F) << 6) |
			     (static_cast<uint8_t> (utf8[i + 1]) & 0x3F);
			len = 2;
		}
		else if ((c0 & 0xF0) == 0xE0 && i + 2 < n)
		{
			cp = (static_cast<char32_t> (c0 & 0x0F) << 12) |
			     ((static_cast<uint8_t> (utf8[i + 1]) & 0x3F) << 6) |
			     (static_cast<uint8_t> (utf8[i + 2]) & 0x3F);
			len = 3;
		}
		else if ((c0 & 0xF8) == 0xF0 && i + 3 < n)
		{
			cp = (static_cast<char32_t> (c0 & 0x07) << 18) |
			     ((static_cast<uint8_t> (utf8[i + 1]) & 0x3F) << 12) |
			     ((static_cast<uint8_t> (utf8[i + 2]) & 0x3F) << 6) |
			     (static_cast<uint8_t> (utf8[i + 3]) & 0x3F);
			len = 4;
		}
		if (cp <= 0xFFFF)
		{
			out.push_back (static_cast<char16_t> (cp));
		}
		else
		{
			cp -= 0x10000;
			out.push_back (static_cast<char16_t> (0xD800 + (cp >> 10)));
			out.push_back (static_cast<char16_t> (0xDC00 + (cp & 0x3FF)));
		}
		i += len;
	}
}

//------------------------------------------------------------------------
void utf16ToUtf8 (std::u16string_view utf16, std::string& out)
{
	out.clear ();
	out.reserve (utf16.size ());
	auto append = [&out] (char32_t cp) {
		if (cp < 0x80)
		{
			out.push_back (static_cast<char> (cp));
		}
		else if (cp < 0x800)
		{
			out.push_back (static_cast<char> (0xC0 | (cp >> 6)));
			out.push_back (static_cast<char> (0x80 | (cp & 0x3F)));
		}
		else if (cp < 0x10000)
		{
			out.push_back (static_cast<char> (0xE0 | (cp >> 12)));
			out.push_back (static_cast<char> (0x80 | ((cp >> 6) & 0x3F)));
			out.push_back (static_cast<char> (0x80 | (cp & 0x3F)));
		}
		else
		{
			out.push_back (static_cast<char> (0xF0 | (cp >> 18)));
			out.push_back (static_cast<char> (0x80 | ((cp >> 12) & 0x3F)));
			out.push_back (static_cast<char> (0x80 | ((cp >> 6) & 0x3F)));
			out.push_back (static_cast<char> (0x80 | (cp & 0x3F)));
		}
	};
	size_t i = 0;
	while (i < utf16.size ())
	{
		const auto u = utf16[i];
		if (u >= 0xD800 && u <= 0xDBFF && i + 1 < utf16.size ())
		{
			const auto u2 = utf16[i + 1];
			if (u2 >= 0xDC00 && u2 <= 0xDFFF)
			{
				const char32_t cp = 0x10000 + ((static_cast<char32_t> (u) - 0xD800) << 10) +
				                    (u2 - 0xDC00);
				append (cp);
				i += 2;
				continue;
			}
		}
		append (u);
		++i;
	}
}

std::string utf16ToUtf8 (std::u16string_view utf16)
{
	std::string s;
	utf16ToUtf8 (utf16, s);
	return s;
}

} // namespace GenShell
