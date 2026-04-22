#include "VitPluginTemplateRegistry.h"

namespace vit
{

namespace
{

bool containsAny (const juce::String& haystack, std::initializer_list<const char*> needles)
{
    for (const auto* needle : needles)
        if (haystack.contains (needle))
            return true;

    return false;
}

} // namespace

juce::String VitPluginTemplateRegistry::inferTemplateRole (te::Plugin& plugin)
{
    if (plugin.isSynth())
        return "instrument";

    const auto label = (plugin.getName() + " " + plugin.getPluginType()).toLowerCase();

    if (containsAny (label, { "eq", "equalizer", "equaliser", "filter" }))
        return "eq";

    if (containsAny (label, { "comp", "compress", "limiter", "gate" }))
        return "comp";

    if (containsAny (label, { "bus", "aux", "send", "return", "mix", "sum" }))
        return "bus";

    return "other";
}

juce::String VitPluginTemplateRegistry::inferNormalizedRole (const juce::String& parameterName,
                                                             const juce::String& templateRole)
{
    const auto label = parameterName.toLowerCase();

    if (templateRole == "comp")
    {
        if (containsAny (label, { "threshold", "thresh" })) return "comp_threshold";
        if (containsAny (label, { "ratio" })) return "comp_ratio";
        if (containsAny (label, { "attack" })) return "comp_attack";
        if (containsAny (label, { "release" })) return "comp_release";
        if (containsAny (label, { "makeup", "gain" })) return "comp_makeup_gain";
    }

    if (templateRole == "eq")
    {
        if (containsAny (label, { "low", "bass" }) && containsAny (label, { "freq", "hz" })) return "eq_freq_low";
        if (containsAny (label, { "mid" }) && containsAny (label, { "freq", "hz" })) return "eq_freq_mid";
        if (containsAny (label, { "high", "treble" }) && containsAny (label, { "freq", "hz" })) return "eq_freq_high";
        if (containsAny (label, { "gain" })) return "eq_gain";
        if (containsAny (label, { "q", "width" })) return "eq_q";
    }

    if (templateRole == "instrument")
    {
        if (containsAny (label, { "attack" })) return "instrument_attack";
        if (containsAny (label, { "release" })) return "instrument_release";
        if (containsAny (label, { "cutoff" })) return "instrument_cutoff";
        if (containsAny (label, { "resonance", "reso" })) return "instrument_resonance";
    }

    if (templateRole == "bus")
    {
        if (containsAny (label, { "send" })) return "bus_send_level";
        if (containsAny (label, { "return" })) return "bus_return_level";
        if (containsAny (label, { "mix" })) return "bus_mix";
    }

    if (containsAny (label, { "bypass" })) return "common_bypass";
    if (containsAny (label, { "gain", "volume", "level" })) return "common_gain";
    if (containsAny (label, { "mix", "dry", "wet" })) return "common_mix";
    if (containsAny (label, { "pan" })) return "common_pan";

    return "other";
}

juce::String VitPluginTemplateRegistry::inferDisplayGroup (const juce::String& normalizedRole,
                                                           const juce::String& templateRole)
{
    juce::ignoreUnused (templateRole);

    if (normalizedRole.startsWith ("comp_")) return "Dynamics";
    if (normalizedRole.startsWith ("eq_")) return "EQ";
    if (normalizedRole.startsWith ("instrument_")) return "Tone";
    if (normalizedRole.startsWith ("bus_")) return "Routing";
    if (normalizedRole == "common_bypass") return "Utility";
    if (normalizedRole == "common_gain" || normalizedRole == "common_mix" || normalizedRole == "common_pan") return "Mix";
    return "Other";
}

} // namespace vit
