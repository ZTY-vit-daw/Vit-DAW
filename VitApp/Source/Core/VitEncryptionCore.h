#pragma once

#include <JuceHeader.h>

#include <array>
#include <cstdint>
#include <vector>

namespace vit
{

/** Envelope encryption for Vit project files (V1 wire format, libsodium). */
class VitEncryptionCore
{
public:
    VitEncryptionCore() = delete;

    static std::vector<uint8_t> encryptProject (const juce::String& plainXml);
    static juce::String decryptProject (const std::vector<uint8_t>& data);

private:
    static std::array<unsigned char, 32> getAppMasterKey();
};

} // namespace vit
