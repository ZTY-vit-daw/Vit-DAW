#include "VitPluginGrabber.h"

#include "VitPluginTemplateRegistry.h"

#include <unordered_set>
#include <unordered_map>

namespace vit
{

namespace
{

juce::var stringArrayToVarArray (const juce::StringArray& strings)
{
    juce::Array<juce::var> out;
    for (const auto& text : strings)
        out.add (text);
    return juce::var (out);
}

juce::var buildDisplayProbeSample (te::AutomatableParameter& parameter, float normalisedValue)
{
    const auto clippedNormalised = juce::jlimit (0.0f, 1.0f, normalisedValue);
    const auto mappedValue = parameter.valueRange.convertFrom0to1 (clippedNormalised);
    const auto text = parameter.valueToString (mappedValue);

    auto sample = std::make_unique<juce::DynamicObject>();
    sample->setProperty ("normalized_value", clippedNormalised);
    sample->setProperty ("value", mappedValue);
    sample->setProperty ("text", text);
    return juce::var (sample.release());
}

juce::var buildDiscreteLabelRows (te::AutomatableParameter& parameter, int numStates)
{
    juce::Array<juce::var> rows;
    const auto labels = parameter.getAllLabels();

    for (int state = 0; state < numStates; ++state)
    {
        const auto value = parameter.getValueForState (state);
        auto label = state < labels.size() ? labels[state] : juce::String();
        if (label.isEmpty())
            label = parameter.getLabelForValue (value);

        auto row = std::make_unique<juce::DynamicObject>();
        row->setProperty ("index", state);
        row->setProperty ("value", value);
        row->setProperty ("label", label);
        rows.add (juce::var (row.release()));
    }

    return juce::var (rows);
}

juce::var buildParameterDisplayProbe (te::AutomatableParameter& parameter, bool isDiscrete, int numStates)
{
    auto probe = std::make_unique<juce::DynamicObject>();
    juce::Array<juce::var> samples;
    juce::Array<juce::var> capabilities;
    juce::Array<juce::var> issues;

    probe->setProperty ("mode", "read_only_value_to_string");
    probe->setProperty ("current_text", parameter.getCurrentValueAsString());
    probe->setProperty ("label", parameter.getLabel());

    for (const auto normalisedValue : { 0.0f, 0.25f, 0.5f, 0.75f, 1.0f })
        samples.add (buildDisplayProbeSample (parameter, normalisedValue));

    capabilities.add ("current_value_text");
    capabilities.add ("value_to_string_samples");

    if (parameter.getLabel().isNotEmpty())
        capabilities.add ("unit_label");
    else
        issues.add ("empty_label");

    if (isDiscrete && numStates > 0)
    {
        probe->setProperty ("discrete_labels", buildDiscreteLabelRows (parameter, numStates));
        probe->setProperty ("all_labels", stringArrayToVarArray (parameter.getAllLabels()));
        capabilities.add ("discrete_labels");
    }

    if (parameter.getCurrentValueAsString().isEmpty())
        issues.add ("empty_current_text");

    probe->setProperty ("samples", juce::var (samples));
    probe->setProperty ("capabilities", juce::var (capabilities));
    probe->setProperty ("issues", juce::var (issues));
    return juce::var (probe.release());
}

} // namespace

juce::String VitPluginGrabber::inferTemplateRoleForParameters (te::ExternalPlugin& plugin,
                                                               const juce::String& fallbackRole)
{
    juce::StringArray parameterNames;
    const auto automatableParams = plugin.getAutomatableParameters();

    for (auto* parameter : automatableParams)
    {
        if (parameter == nullptr)
            continue;

        const auto rawName = parameter->paramName.isNotEmpty() ? parameter->paramName
                                                               : parameter->paramID;
        if (rawName.isNotEmpty())
            parameterNames.add (rawName);
    }

    return VitPluginTemplateRegistry::inferTemplateRoleFromParameterNames (parameterNames, fallbackRole);
}

juce::Array<juce::var> VitPluginGrabber::buildParameterDescriptors (te::ExternalPlugin& plugin,
                                                                    const juce::String& templateRole)
{
    juce::Array<juce::var> parameters;
    const auto automatableParams = plugin.getAutomatableParameters();

    for (int i = 0; i < automatableParams.size(); ++i)
    {
        auto* parameter = automatableParams[i];
        if (parameter == nullptr)
            continue;

        const auto rawParamId = parameter->paramID.isNotEmpty() ? parameter->paramID
                                                                : "param_" + juce::String (i + 1);
        const auto rawName = parameter->paramName.isNotEmpty() ? parameter->paramName
                                                               : rawParamId;
        const auto normalizedRole = VitPluginTemplateRegistry::inferNormalizedRole (rawName, templateRole);
        const auto displayGroup = VitPluginTemplateRegistry::inferDisplayGroup (normalizedRole, templateRole);
        const auto controlRelevance = VitPluginTemplateRegistry::inferControlRelevance (rawName, normalizedRole);
        const auto valueRange = parameter->getValueRange();
        const auto isDiscrete = parameter->isDiscrete();
        const auto numStates = isDiscrete ? parameter->getNumberOfStates() : 0;
        auto row = std::make_unique<juce::DynamicObject>();
        row->setProperty ("id", rawParamId);
        row->setProperty ("param_id", rawParamId);
        row->setProperty ("raw_param_id", rawParamId);
        row->setProperty ("raw_param_name", rawName);
        row->setProperty ("name", rawName);
        row->setProperty ("value", parameter->getCurrentValue());
        row->setProperty ("normalized_value", parameter->getCurrentNormalisedValue());
        row->setProperty ("value_text", parameter->getCurrentValueAsString());
        row->setProperty ("min", valueRange.getStart());
        row->setProperty ("max", valueRange.getEnd());
        row->setProperty ("is_discrete", isDiscrete);
        row->setProperty ("num_steps", numStates);
        row->setProperty ("is_boolean", numStates == 2 || normalizedRole == "common_bypass" || normalizedRole.contains ("enable"));
        row->setProperty ("normalized_role", normalizedRole);
        row->setProperty ("display_group", displayGroup);
        row->setProperty ("host_controllable", true);
        row->setProperty ("control_relevance", controlRelevance);
        row->setProperty ("control_priority", VitPluginTemplateRegistry::controlPriorityForRelevance (controlRelevance));
        row->setProperty ("alias", rawName);
        row->setProperty ("supports_automation", true);
        row->setProperty ("source", "tracktion_automatable");
        row->setProperty ("display_probe", buildParameterDisplayProbe (*parameter, isDiscrete, numStates));
        parameters.add (juce::var (row.release()));
    }

    return parameters;
}

juce::Array<juce::var> VitPluginGrabber::buildRecommendedGroups (const juce::Array<juce::var>& parameterDescriptors)
{
    std::unordered_map<std::string, juce::Array<juce::var>> grouped;
    juce::StringArray firstSeenGroups;

    for (const auto& param : parameterDescriptors)
    {
        auto* object = param.getDynamicObject();
        if (object == nullptr)
            continue;

        const auto group = object->getProperty ("display_group").toString();
        const auto cleanGroup = group.isNotEmpty() ? group : juce::String ("Other");
        const auto key = cleanGroup.toStdString();
        if (! grouped.contains (key))
            firstSeenGroups.add (cleanGroup);
        grouped[key].add (object->getProperty ("id"));
    }

    juce::Array<juce::var> groups;
    juce::StringArray emittedGroups;

    auto addGroup = [&] (const juce::String& groupName)
    {
        const auto key = groupName.toStdString();
        auto it = grouped.find (key);
        if (it == grouped.end())
            return;

        auto group = std::make_unique<juce::DynamicObject>();
        group->setProperty ("name", groupName);
        group->setProperty ("parameter_ids", juce::var (it->second));
        groups.add (juce::var (group.release()));
        emittedGroups.add (groupName);
    };

    for (const auto& groupName : { "Tone", "Dynamics", "Mix", "Modulation", "Utility", "Routing", "Other" })
        addGroup (groupName);

    for (const auto& groupName : firstSeenGroups)
    {
        if (! emittedGroups.contains (groupName))
            addGroup (groupName);
    }

    return groups;
}

juce::Array<juce::var> VitPluginGrabber::buildQuickControls (const juce::Array<juce::var>& parameterDescriptors,
                                                             const juce::String& templateRole)
{
    juce::StringArray preferredRoles;

    if (templateRole == "comp")
        preferredRoles.addTokens ("comp_threshold,comp_ratio,comp_attack,comp_release,comp_makeup_gain,common_mix,common_gain,common_bypass", ",", {});
    else if (templateRole == "eq")
        preferredRoles.addTokens ("eq_frequency,eq_cutoff,eq_freq_low,eq_freq_mid,eq_freq_high,eq_gain,eq_q,comp_threshold,comp_ratio,common_mix,common_gain,common_bypass", ",", {});
    else if (templateRole == "instrument")
        preferredRoles.addTokens ("instrument_cutoff,instrument_resonance,instrument_attack,instrument_release,instrument_decay,instrument_sustain,common_gain,common_bypass", ",", {});
    else if (templateRole == "bus")
        preferredRoles.addTokens ("bus_send_level,bus_return_level,bus_mix,common_gain,common_pan,common_bypass", ",", {});
    else
        preferredRoles.addTokens ("common_mix,common_gain,common_pan,common_width,common_bypass,tone_filter,tone_drive,mod_rate,mod_depth,comp_threshold,comp_ratio", ",", {});

    auto makeControl = [] (const juce::var& param, const juce::String& widget)
    {
        auto* object = param.getDynamicObject();
        if (object == nullptr)
            return juce::var();

        auto control = std::make_unique<juce::DynamicObject>();
        control->setProperty ("param_id", object->getProperty ("id"));
        control->setProperty ("raw_param_id", object->getProperty ("raw_param_id"));
        control->setProperty ("label", object->getProperty ("alias"));
        control->setProperty ("widget", widget);
        control->setProperty ("normalized_role", object->getProperty ("normalized_role"));
        control->setProperty ("display_group", object->getProperty ("display_group"));
        control->setProperty ("host_controllable", object->getProperty ("host_controllable"));
        control->setProperty ("control_relevance", object->getProperty ("control_relevance"));
        control->setProperty ("control_priority", object->getProperty ("control_priority"));
        control->setProperty ("value", object->getProperty ("value"));
        return juce::var (control.release());
    };

    auto widgetForRole = [] (const juce::String& normalizedRole)
    {
        if (normalizedRole == "common_bypass" || normalizedRole.contains ("enable"))
            return juce::String ("toggle");

        if (normalizedRole.contains ("freq") || normalizedRole.contains ("cutoff") || normalizedRole.contains ("resonance")
            || normalizedRole.contains ("pan") || normalizedRole.contains ("ratio"))
            return juce::String ("knob");

        return juce::String ("slider");
    };

    juce::Array<juce::var> controls;
    std::unordered_set<std::string> seenParamIds;

    for (const auto& preferredRole : preferredRoles)
    {
        for (const auto& param : parameterDescriptors)
        {
            auto* object = param.getDynamicObject();
            if (object == nullptr)
                continue;

            const auto paramId = object->getProperty ("id").toString();
            if (seenParamIds.contains (paramId.toStdString()))
                continue;

            const auto normalizedRole = object->getProperty ("normalized_role").toString();
            if (normalizedRole != preferredRole)
                continue;

            controls.add (makeControl (param, widgetForRole (normalizedRole)));
            seenParamIds.insert (paramId.toStdString());
            break;
        }
    }

    for (const auto& param : parameterDescriptors)
    {
        if (controls.size() >= 8)
            break;

        auto* object = param.getDynamicObject();
        if (object == nullptr)
            continue;

        const auto paramId = object->getProperty ("id").toString();
        if (seenParamIds.contains (paramId.toStdString()))
            continue;

        const auto normalizedRole = object->getProperty ("normalized_role").toString();
        if (normalizedRole == "other")
            continue;
        controls.add (makeControl (param, widgetForRole (normalizedRole)));
        seenParamIds.insert (paramId.toStdString());
    }

    return controls;
}

} // namespace vit
