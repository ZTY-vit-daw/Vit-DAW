#include "VitParamSurface.h"

namespace vit
{

juce::String VitParamSurface::normaliseControlKind (const juce::String& kind)
{
    const auto lowered = kind.trim().toLowerCase();

    if (lowered == "slider" || lowered == "knob" || lowered == "xypad" || lowered == "toggle"
        || lowered == "button" || lowered == "numberbox" || lowered == "range"
        || lowered == "envelope" || lowered == "lfo" || lowered == "stepsequencer"
        || lowered == "macropanel")
        return lowered;

    return "slider";
}

juce::StringArray VitParamSurface::supportedControlKinds()
{
    return { "slider", "knob", "xypad", "toggle", "button", "numberbox", "range", "envelope", "lfo", "stepsequencer", "macropanel" };
}

juce::Identifier VitParamSurface::valuePropertyForOutput (const juce::String& outputName)
{
    const auto normalized = outputName.trim().toLowerCase();

    if (normalized.isEmpty() || normalized == "value")
        return juce::Identifier ("current_value");

    return juce::Identifier (normalized + "_value");
}

juce::ValueTree VitParamSurface::createControlNodeState (const juce::String& nodeId,
                                                         const juce::String& kind,
                                                         const juce::String& name,
                                                         float x,
                                                         float y)
{
    juce::ValueTree state ("VIT_CONTROL_NODE");
    state.setProperty ("node_id", nodeId, nullptr);
    state.setProperty ("kind", normaliseControlKind (kind), nullptr);
    state.setProperty ("name", name.trim().isNotEmpty() ? name.trim() : kind.trim(), nullptr);
    state.setProperty ("x", x, nullptr);
    state.setProperty ("y", y, nullptr);
    state.setProperty ("min", 0.0, nullptr);
    state.setProperty ("max", 1.0, nullptr);
    state.setProperty ("default_value", 0.0, nullptr);
    state.setProperty ("current_value", 0.0, nullptr);
    state.setProperty ("enabled", true, nullptr);
    return state;
}

juce::var VitParamSurface::createNodeSnapshot (const juce::ValueTree& nodeState)
{
    auto object = std::make_unique<juce::DynamicObject>();
    auto currentValues = std::make_unique<juce::DynamicObject>();

    object->setProperty ("node_id", nodeState.getProperty ("node_id").toString());
    object->setProperty ("kind", normaliseControlKind (nodeState.getProperty ("kind").toString()));
    object->setProperty ("name", nodeState.getProperty ("name").toString());
    object->setProperty ("x", nodeState.getProperty ("x"));
    object->setProperty ("y", nodeState.getProperty ("y"));
    object->setProperty ("min", nodeState.getProperty ("min"));
    object->setProperty ("max", nodeState.getProperty ("max"));
    object->setProperty ("default_value", nodeState.getProperty ("default_value"));
    object->setProperty ("current_value", nodeState.getProperty ("current_value"));
    object->setProperty ("enabled", nodeState.getProperty ("enabled"));

    for (int i = 0; i < nodeState.getNumProperties(); ++i)
    {
        const auto propertyIdentifier = nodeState.getPropertyName (i);
        const auto propertyName = propertyIdentifier.toString();
        if (! propertyName.endsWithIgnoreCase ("_value"))
            continue;

        const auto outputName = propertyName == "current_value"
                                    ? juce::String ("value")
                                    : propertyName.upToLastOccurrenceOf ("_value", false, false);
        currentValues->setProperty (outputName, nodeState.getProperty (propertyIdentifier));
    }

    object->setProperty ("current_values", juce::var (currentValues.release()));
    return juce::var (object.release());
}

} // namespace vit
