#pragma once

#include <JuceHeader.h>
#include <cstdio>

namespace vit
{

// ===================================================================
// Plugin class taxonomy
// ===================================================================
namespace pluginClass
{
    constexpr auto unknown    = "unknown";
    constexpr auto eq         = "eq";
    constexpr auto compressor = "compressor";
    constexpr auto limiter    = "limiter";
    constexpr auto reverb     = "reverb";
    constexpr auto delay      = "delay";
    constexpr auto chorus     = "chorus";
    constexpr auto flanger    = "flanger";
    constexpr auto phaser     = "phaser";
    constexpr auto distortion = "distortion";
    constexpr auto filter     = "filter";
    constexpr auto gate       = "gate";
    constexpr auto deesser    = "deesser";
    constexpr auto saturator  = "saturator";
    constexpr auto analyser   = "analyser";
    constexpr auto synth      = "synth";
    constexpr auto sampler    = "sampler";
    constexpr auto utility    = "utility";
    constexpr auto controller = "controller";
    constexpr auto modulation = "modulation";
    constexpr auto multiFx    = "multi_fx";

    inline bool isValid (const juce::String& cls)
    {
        const auto lower = cls.trim().toLowerCase();
        return lower == eq || lower == compressor || lower == limiter
            || lower == reverb || lower == delay || lower == chorus
            || lower == flanger || lower == phaser || lower == distortion
            || lower == filter || lower == gate || lower == deesser
            || lower == saturator || lower == analyser
            || lower == synth || lower == sampler || lower == utility
            || lower == controller || lower == modulation || lower == multiFx;
    }
} // namespace pluginClass

// ===================================================================
// Extended profile field keys
// ===================================================================
namespace profileField
{
    constexpr auto pluginClass         = "class";
    constexpr auto groups              = "groups";
    constexpr auto groupId             = "id";
    constexpr auto groupRole           = "role";
    constexpr auto groupParams         = "params";
    constexpr auto virtualControls     = "virtual_controls";
    constexpr auto virtualControlName  = "name";
    constexpr auto virtualControlInputs= "inputs";
    constexpr auto virtualControlResolver = "resolver";
    constexpr auto safety              = "safety";
    constexpr auto pluginSkill         = "plugin_skill";
    constexpr auto pluginSkillValidatorWarnings = "plugin_skill_validator_warnings";
    constexpr auto safetyMaxGainDb     = "max_gain_change_db";
    constexpr auto safetyMinQ          = "min_q";
    constexpr auto safetyMaxQ          = "max_q";
    constexpr auto safetyMinThreshold  = "min_threshold_db";
    constexpr auto safetyMaxThreshold  = "max_threshold_db";
    constexpr auto safetyMinRatio      = "min_ratio";
    constexpr auto safetyMaxRatio      = "max_ratio";
} // namespace profileField

// ===================================================================
// Parameter signature hash ? checksum of sorted param_id list
// ===================================================================
namespace paramSignature
{
    inline juce::String fnv1a64Hex (const juce::String& text)
    {
        constexpr unsigned long long offset = 14695981039346656037ull;
        constexpr unsigned long long prime = 1099511628211ull;
        auto hash = offset;

        for (const auto* p = reinterpret_cast<const unsigned char*> (text.toRawUTF8()); *p != 0; ++p)
        {
            hash ^= static_cast<unsigned long long> (*p);
            hash *= prime;
        }

        char buffer[19] {};
        std::snprintf (buffer, sizeof (buffer), "p_%016llx", hash);
        return juce::String (buffer);
    }

    inline juce::String computeHash (const juce::Array<juce::var>& parameters)
    {
        juce::StringArray ids;
        for (const auto& p : parameters)
            if (auto* obj = p.getDynamicObject())
                if (const auto id = obj->getProperty ("id").toString().trim(); id.isNotEmpty())
                    ids.add (id);

        ids.sort (true);
        const auto joined = ids.joinIntoString ("|");
        return fnv1a64Hex (joined);
    }

    inline juce::String loadFromProfile (const juce::var& profile)
    {
        if (auto* obj = profile.getDynamicObject())
        {
            if (const auto topLevel = obj->getProperty ("param_signature_hash").toString().trim(); topLevel.isNotEmpty())
                return topLevel;
            if (auto* skill = obj->getProperty (profileField::pluginSkill).getDynamicObject())
                if (auto* identity = skill->getProperty ("identity").getDynamicObject())
                    return identity->getProperty ("param_signature_hash").toString().trim();
            return obj->getProperty ("param_signature_hash").toString().trim();
        }
        return {};
    }
} // namespace paramSignature

} // namespace vit
