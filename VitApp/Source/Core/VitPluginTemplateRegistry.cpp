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

bool isUnsetRole (const juce::String& role)
{
    const auto clean = role.trim().toLowerCase();
    return clean.isEmpty() || clean == "other" || clean == "unknown";
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

juce::String VitPluginTemplateRegistry::inferTemplateRoleFromParameterNames (const juce::StringArray& parameterNames,
                                                                             const juce::String& fallbackRole)
{
    const auto cleanFallback = fallbackRole.trim().toLowerCase();
    if (! isUnsetRole (cleanFallback))
        return cleanFallback;

    int eqScore = 0;
    int compScore = 0;
    int instrumentScore = 0;
    int busScore = 0;

    for (const auto& parameterName : parameterNames)
    {
        const auto label = parameterName.toLowerCase();

        if (containsAny (label, { "band", "filter", "frequency", "freq", "hz", "cutoff", "resonance", "reso" }))
            ++eqScore;
        if (containsAny (label, { " q", "q ", "width", "slope" }))
            ++eqScore;
        if (containsAny (label, { "threshold", "thresh", "ratio", "attack", "release", "compress", "limiter", "gate" }))
            ++compScore;
        if (containsAny (label, { "osc", "wave", "voice", "env", "envelope", "sustain", "decay", "lfo" }))
            ++instrumentScore;
        if (containsAny (label, { "send", "return", "aux", "bus" }))
            ++busScore;
    }

    if (eqScore >= 4 && eqScore >= compScore)
        return "eq";
    if (compScore >= 4)
        return "comp";
    if (instrumentScore >= 4)
        return "instrument";
    if (busScore >= 3)
        return "bus";

    return cleanFallback.isEmpty() ? juce::String ("other") : cleanFallback;
}

juce::String VitPluginTemplateRegistry::inferNormalizedRole (const juce::String& parameterName,
                                                             const juce::String& templateRole)
{
    const auto label = parameterName.toLowerCase();
    const auto role = templateRole.trim().toLowerCase();

    if (containsAny (label, { "bypass" })) return "common_bypass";

    if (role == "comp")
    {
        if (containsAny (label, { "threshold", "thresh" })) return "comp_threshold";
        if (containsAny (label, { "ratio" })) return "comp_ratio";
        if (containsAny (label, { "attack" })) return "comp_attack";
        if (containsAny (label, { "release" })) return "comp_release";
        if (containsAny (label, { "makeup", "gain" })) return "comp_makeup_gain";
    }

    if (role == "eq")
    {
        if (containsAny (label, { "threshold", "thresh" })) return "comp_threshold";
        if (containsAny (label, { "ratio" })) return "comp_ratio";
        if (containsAny (label, { "attack" })) return "comp_attack";
        if (containsAny (label, { "release" })) return "comp_release";
        if (containsAny (label, { "dyn", "dynamic" })) return "comp_dynamics_enable";
        if (containsAny (label, { "low", "bass" }) && containsAny (label, { "freq", "hz" })) return "eq_freq_low";
        if (containsAny (label, { "mid" }) && containsAny (label, { "freq", "hz" })) return "eq_freq_mid";
        if (containsAny (label, { "high", "treble" }) && containsAny (label, { "freq", "hz" })) return "eq_freq_high";
        if (containsAny (label, { "cutoff" })) return "eq_cutoff";
        if (containsAny (label, { "frequency", "freq", "hz" })) return "eq_frequency";
        if (containsAny (label, { "gain" })) return "eq_gain";
        if (label.endsWith (" q") || containsAny (label, { " q ", "quality", "width" })) return "eq_q";
        if (containsAny (label, { "type", "mode", "shape", "slope" })) return "eq_filter_type";
    }

    if (role == "instrument")
    {
        if (containsAny (label, { "attack" })) return "instrument_attack";
        if (containsAny (label, { "release" })) return "instrument_release";
        if (containsAny (label, { "decay" })) return "instrument_decay";
        if (containsAny (label, { "sustain" })) return "instrument_sustain";
        if (containsAny (label, { "cutoff" })) return "instrument_cutoff";
        if (containsAny (label, { "resonance", "reso" })) return "instrument_resonance";
        if (containsAny (label, { "osc", "waveform", "wave" })) return "instrument_oscillator";
    }

    if (role == "bus")
    {
        if (containsAny (label, { "send" })) return "bus_send_level";
        if (containsAny (label, { "return" })) return "bus_return_level";
        if (containsAny (label, { "mix" })) return "bus_mix";
    }

    if (containsAny (label, { "threshold", "thresh" })) return "comp_threshold";
    if (containsAny (label, { "ratio" })) return "comp_ratio";
    if (containsAny (label, { "attack" })) return "comp_attack";
    if (containsAny (label, { "release" })) return "comp_release";
    if (containsAny (label, { "rate", "speed" })) return "mod_rate";
    if (containsAny (label, { "depth", "amount" })) return "mod_depth";
    if (containsAny (label, { "feedback" })) return "mod_feedback";
    if (containsAny (label, { "drive", "distort", "saturat" })) return "tone_drive";
    if (containsAny (label, { "cutoff", "resonance", "reso" })) return "tone_filter";
    if (containsAny (label, { "mix", "dry", "wet", "blend" })) return "common_mix";
    if (containsAny (label, { "pan" })) return "common_pan";
    if (containsAny (label, { "width", "stereo" })) return "common_width";
    if (containsAny (label, { "gain", "volume", "level", "output", "input" })) return "common_gain";

    return "other";
}

juce::String VitPluginTemplateRegistry::inferDisplayGroup (const juce::String& normalizedRole,
                                                           const juce::String& templateRole)
{
    juce::ignoreUnused (templateRole);

    if (normalizedRole.startsWith ("comp_")) return "Dynamics";
    if (normalizedRole.startsWith ("eq_")) return "Tone";
    if (normalizedRole.startsWith ("instrument_")) return "Tone";
    if (normalizedRole.startsWith ("tone_")) return "Tone";
    if (normalizedRole.startsWith ("mod_")) return "Modulation";
    if (normalizedRole.startsWith ("bus_")) return "Routing";
    if (normalizedRole == "common_bypass") return "Utility";
    if (normalizedRole == "common_gain" || normalizedRole == "common_mix"
        || normalizedRole == "common_pan" || normalizedRole == "common_width")
        return "Mix";
    return "Other";
}

juce::String VitPluginTemplateRegistry::inferControlRelevance (const juce::String& parameterName,
                                                               const juce::String& normalizedRole)
{
    const auto label = parameterName.toLowerCase();
    const auto role = normalizedRole.trim().toLowerCase();

    if (containsAny (label, { "selected", "select", "page", "view", "focus", "display", "meter" }))
        return "ui_state";

    if (role == "common_bypass" || role.contains ("enable") || containsAny (label, { "active", "enable", "on/off" }))
        return "utility_control";

    if (containsAny (label, { "type", "mode", "shape", "slope", "quality", "oversampling", "split" }))
        return "mode_control";

    if (role == "other")
        return "unknown_control";

    return "musical_control";
}

int VitPluginTemplateRegistry::controlPriorityForRelevance (const juce::String& controlRelevance)
{
    const auto relevance = controlRelevance.trim().toLowerCase();
    if (relevance == "musical_control") return 100;
    if (relevance == "utility_control") return 75;
    if (relevance == "mode_control") return 65;
    if (relevance == "unknown_control") return 40;
    if (relevance == "ui_state") return 15;
    return 0;
}

} // namespace vit
