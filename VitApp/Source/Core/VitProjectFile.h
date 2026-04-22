#pragma once

#include <JuceHeader.h>

namespace vit
{

/** Vit 工程默认后缀（替代 .tracktionedit）。 */
inline constexpr const char* vitProjectFileSuffix() noexcept { return ".vit"; }

/** 仍允许打开的旧版/调试后缀。 */
bool isRecognizedProjectExtension (const juce::File& file) noexcept;

/** 保存时强制使用 .vit 后缀（不改变父目录）。 */
juce::File normalizeProjectPathForSave (const juce::File& file);

/** VIT_DUMP_CLEAR_XML=1 时，在保存 .vit 后额外写出明文 .xml 侧车文件。 */
bool isVitDumpClearXmlEnabled() noexcept;

/** 静默写出调试 XML：优先与 .vit 同级；失败则尝试 Workspace/Logs/。 */
void writeClearXmlSidecarSilently (const juce::File& vitFile, const juce::XmlElement& xml);

//==============================================================================
/** 预留：保存前将明文 XML 转为最终写入磁盘的字节流（当前透传，后续可接 AES 等）。 */
juce::String encodeProjectXmlForDisk (const juce::String& plaintextXml);

/** 预留：加载时将磁盘字节还原为明文 XML 文本（当前透传）。 */
juce::String decodeProjectXmlFromDisk (const juce::String& encodedOrPlainXml);

} // namespace vit
