#include "VitEncryptionCore.h"

#include <cstring>

#include <sodium.h>

namespace vit
{

namespace
{

constexpr size_t kHeaderBytes = 102; // 4 + 2 + 24 + 48 + 24

constexpr uint8_t kMagic0 = 'V';
constexpr uint8_t kMagic1 = 'I';
constexpr uint8_t kMagic2 = 'T';
constexpr uint8_t kMagic3 = '1';

constexpr uint16_t kFormatVersion = 1;

/** XOR of ASCII "VIT_V05_SUPER_SECRET_KEY_32BYTES" with 0x5A — no plaintext string in the binary. */
constexpr uint8_t kObfuscatedMasterKey[32] = {
    0x0c, 0x13, 0x0e, 0x05, 0x0c, 0x6a, 0x6f, 0x05, 0x09, 0x0f, 0x0a, 0x1f,
    0x08, 0x05, 0x09, 0x1f, 0x19, 0x08, 0x1f, 0x0e, 0x05, 0x11, 0x1f, 0x03,
    0x05, 0x69, 0x68, 0x18, 0x03, 0x0e, 0x1f, 0x09};

void writeU16LE (uint8_t* p, uint16_t v) noexcept
{
    p[0] = static_cast<uint8_t> (v & 0xff);
    p[1] = static_cast<uint8_t> ((v >> 8) & 0xff);
}

uint16_t readU16LE (const uint8_t* p) noexcept
{
    return static_cast<uint16_t> (p[0] | (static_cast<uint16_t> (p[1]) << 8));
}

} // namespace

std::array<unsigned char, 32> VitEncryptionCore::getAppMasterKey()
{
    std::array<unsigned char, 32> key {};

    for (size_t i = 0; i < key.size(); ++i)
        key[i] = static_cast<unsigned char> (kObfuscatedMasterKey[i] ^ 0x5A);

    return key;
}

std::vector<uint8_t> VitEncryptionCore::encryptProject (const juce::String& plainXml)
{
    if (sodium_init() < 0)
        return {};

    auto masterKey = getAppMasterKey();

    std::vector<uint8_t> dek (crypto_secretbox_KEYBYTES);
    randombytes_buf (dek.data(), dek.size());

    uint8_t nonce1[crypto_secretbox_NONCEBYTES];
    randombytes_buf (nonce1, sizeof (nonce1));

    std::vector<uint8_t> wrappedDek (crypto_secretbox_MACBYTES + dek.size());
    if (crypto_secretbox_easy (wrappedDek.data(),
                               dek.data(),
                               dek.size(),
                               nonce1,
                               masterKey.data()) != 0)
        return {};

    uint8_t nonce2[crypto_aead_xchacha20poly1305_ietf_NPUBBYTES];
    randombytes_buf (nonce2, sizeof (nonce2));

    const int numUtf8Bytes = plainXml.getNumBytesAsUTF8();
    const juce::MemoryBlock utf8 (plainXml.toRawUTF8(), (size_t) numUtf8Bytes);
    const auto* plain = static_cast<const unsigned char*> (utf8.getData());
    const auto mlen = static_cast<unsigned long long> (utf8.getSize());

    std::vector<uint8_t> bodyCipher (mlen + crypto_aead_xchacha20poly1305_ietf_ABYTES);
    unsigned long long bodyLen = 0;

    if (crypto_aead_xchacha20poly1305_ietf_encrypt (bodyCipher.data(),
                                                     &bodyLen,
                                                     plain,
                                                     mlen,
                                                     nullptr,
                                                     0,
                                                     nullptr,
                                                     nonce2,
                                                     dek.data()) != 0)
        return {};

    bodyCipher.resize (bodyLen);

    std::vector<uint8_t> out;
    out.resize (kHeaderBytes + bodyCipher.size());

    uint8_t* o = out.data();
    o[0] = kMagic0;
    o[1] = kMagic1;
    o[2] = kMagic2;
    o[3] = kMagic3;
    writeU16LE (o + 4, kFormatVersion);
    std::memcpy (o + 6, nonce1, sizeof (nonce1));
    std::memcpy (o + 30, wrappedDek.data(), wrappedDek.size());
    std::memcpy (o + 78, nonce2, sizeof (nonce2));
    std::memcpy (o + 102, bodyCipher.data(), bodyCipher.size());

    sodium_memzero (dek.data(), dek.size());
    sodium_memzero (masterKey.data(), masterKey.size());

    return out;
}

juce::String VitEncryptionCore::decryptProject (const std::vector<uint8_t>& data)
{
    if (sodium_init() < 0)
        return {};

    if (data.size() < kHeaderBytes + crypto_aead_xchacha20poly1305_ietf_ABYTES + 1)
        return {};

    const uint8_t* p = data.data();

    if (p[0] != kMagic0 || p[1] != kMagic1 || p[2] != kMagic2 || p[3] != kMagic3)
        return {};

    if (readU16LE (p + 4) != kFormatVersion)
        return {};

    auto masterKey = getAppMasterKey();

    uint8_t nonce1[crypto_secretbox_NONCEBYTES];
    std::memcpy (nonce1, p + 6, sizeof (nonce1));

    std::array<unsigned char, 32> dek {};

    if (crypto_secretbox_open_easy (dek.data(),
                                    p + 30,
                                    48,
                                    nonce1,
                                    masterKey.data()) != 0)
        return {};

    uint8_t nonce2[crypto_aead_xchacha20poly1305_ietf_NPUBBYTES];
    std::memcpy (nonce2, p + 78, sizeof (nonce2));

    const size_t cipherBodyLen = data.size() - kHeaderBytes;

    if (cipherBodyLen < crypto_aead_xchacha20poly1305_ietf_ABYTES)
        return {};

    const auto* cipherBody = p + kHeaderBytes;
    std::vector<unsigned char> plain (cipherBodyLen);

    unsigned long long plainLen = 0;

    if (crypto_aead_xchacha20poly1305_ietf_decrypt (plain.data(),
                                                     &plainLen,
                                                     nullptr,
                                                     cipherBody,
                                                     cipherBodyLen,
                                                     nullptr,
                                                     0,
                                                     nonce2,
                                                     dek.data()) != 0)
        return {};

    plain.resize (plainLen);
    sodium_memzero (dek.data(), dek.size());
    sodium_memzero (masterKey.data(), masterKey.size());

    return juce::String::fromUTF8 (reinterpret_cast<const char*> (plain.data()),
                                   static_cast<int> (plain.size()));
}

} // namespace vit
