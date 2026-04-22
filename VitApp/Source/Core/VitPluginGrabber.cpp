#include "VitPluginGrabber.h"

#include "VitPluginTemplateRegistry.h"

#include <unordered_set>
#include <unordered_map>

namespace vit
{

juce::Array<juce::var> VitPluginGrabber::buildParameterDescriptors (te::ExternalPlugin& plugin,
                                                                    const juce::String& templateRole)
{
    juce::Array<juce::var> parameters;
    auto* instance = plugin.getAudioPluginInstance();

    if (instance == nullptr)
        return parameters;

    const auto params = instance->getParameters();

    for (int i = 0; i < params.size(); ++i)
    {
        auto* parameter = params[i];
        if (parameter == nullptr)
            continue;

        const auto rawName = parameter->getName (128);
        const auto normalizedRole = VitPluginTemplateRegistry::inferNormalizedRole (rawName, templateRole);
        const auto displayGroup = VitPluginTemplateRegistry::inferDisplayGroup (normalizedRole, templateRole);
        auto row = std::make_unique<juce::DynamicObject>();
        row->setProperty ("id", "param_" + juce::String (i + 1));
        row->setProperty ("raw_param_id", "param_" + juce::String (i + 1));
        row->setProperty ("raw_param_name", rawName);
        row->setProperty ("name", rawName);
        row->setProperty ("value", parameter->getValue());
        row->setProperty ("min", 0.0);
        row->setProperty ("max", 1.0);
        row->setProperty ("normalized_role", normalizedRole);
        row->setProperty ("display_group", displayGroup);
        row->setProperty ("alias", rawName);
        row->setProperty ("supports_automation", parameter->isAutomatable());
        parameters.add (juce::var (row.release()));
    }

    return parameters;
}

juce::Array<juce::var> VitPluginGrabber::buildRecommendedGroups (const juce::Array<juce::var>& parameterDescriptors)
{
    std::unordered_map<std::string, juce::Array<juce::var>> grouped;

    for (const auto& param : parameterDescriptors)
    {
        auto* object = param.getDynamicObject();
        if (object == nullptr)
            continue;

        const auto group = object->getProperty ("display_group").toString();
        grouped[group.toStdString()].add (object->getProperty ("id"));
    }

    juce::Array<juce::var> groups;

    for (auto& [groupName, ids] : grouped)
    {
        auto group = std::make_unique<juce::DynamicObject>();
        group->setProperty ("name", juce::String (groupName));
        group->setProperty ("parameter_ids", juce::var (ids));
        groups.add (juce::var (group.release()));
    }

    return groups;
}

juce::Array<juce::var> VitPluginGrabber::buildQuickControls (const juce::Array<juce::var>& parameterDescriptors,
                                                             const juce::String& templateRole)
{
    juce::StringArray preferredRoles;

    if (templateRole == "comp")
        preferredRoles.addTokens ("comp_threshold,comp_ratio,comp_attack,comp_release,comp_makeup_gain,common_mix,common_bypass", ",", {});
    else if (templateRole == "eq")
        preferredRoles.addTokens ("eq_freq_low,eq_freq_mid,eq_freq_high,eq_gain,eq_q,common_mix,common_bypass", ",", {});
    else if (templateRole == "instrument")
        preferredRoles.addTokens ("instrument_cutoff,instrument_resonance,instrument_attack,instrument_release,common_gain,common_bypass", ",", {});
    else if (templateRole == "bus")
        preferredRoles.addTokens ("bus_send_level,bus_return_level,bus_mix,common_gain,common_pan,common_bypass", ",", {});
    else
        preferredRoles.addTokens ("common_gain,common_mix,common_pan,common_bypass", ",", {});

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
        control->setProperty ("value", object->getProperty ("value"));
        return juce::var (control.release());
    };

    auto widgetForRole = [] (const juce::String& normalizedRole)
    {
        if (normalizedRole == "common_bypass")
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
        controls.add (makeControl (param, widgetForRole (normalizedRole)));
        seenParamIds.insert (paramId.toStdString());
    }

    return controls;
}

} // namespace vit
