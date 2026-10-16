//------------------------------------------------------------------------
// GenShell — UTF-8 ↔ UTF-16 转换助手
//------------------------------------------------------------------------
#pragma once

#include <string>
#include <string_view>

namespace GenShell {

void utf8ToUtf16 (const std::string& utf8, std::u16string& out);
void utf16ToUtf8 (std::u16string_view utf16, std::string& out);
std::string utf16ToUtf8 (std::u16string_view utf16);

} // namespace GenShell
