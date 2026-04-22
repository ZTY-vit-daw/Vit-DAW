#include "VitProjectFile.h"

#include "VitPaths.h"

namespace vit
{

bool isRecognizedProjectExtension (const juce::File& file) noexcept
{
    return file.hasFileExtension (vitProjectFileSuffix())
        || file.hasFileExtension (".tracktionedit")
        || file.hasFileExtension (".xml");
}

juce::File normalizeProjectPathForSave (const juce::File& file)
{
    return file.withFileExtension (vitProjectFileSuffix());
}

bool isVitDumpClearXmlEnabled() noexcept
{
    return juce::SystemStats::getEnvironmentVariable ("VIT_DUMP_CLEAR_XML", {}).trim() == "1";
}

void writeClearXmlSidecarSilently (const juce::File& vitFile, const juce::XmlElement& xml)
{
    const auto text = xml.toString();
    const auto sidecar = vitFile.withFileExtension (".xml");

    if (sidecar.replaceWithText (text))
        return;

    auto logsDir = paths::getLogsDirectory();

    if (! paths::ensureDirectoryExists (logsDir, "logs"))
        return;

    juce::ignoreUnused (logsDir.getChildFile (vitFile.getFileNameWithoutExtension() + ".xml")
                            .replaceWithText (text));
}

juce::String encodeProjectXmlForDisk (const juce::String& plaintextXml)
{
    // 预留：压缩 / AES 加密 / 二进制封装等
    return plaintextXml;
}

juce::String decodeProjectXmlFromDisk (const juce::String& encodedOrPlainXml)
{
    // 预留：与 encodeProjectXmlForDisk 对称解密
    return encodedOrPlainXml;
}

} // namespace vit
