#include "VitMacroNode.h"

#include "VitParamSurface.h"

namespace vit
{

juce::ValueTree VitMacroNode::createMacroNodeState (const juce::String& nodeId,
                                                    const juce::String& name,
                                                    float x,
                                                    float y,
                                                    int macroCount)
{
    auto state = VitParamSurface::createControlNodeState (nodeId,
                                                          "macropanel",
                                                          name.trim().isNotEmpty() ? name : "Macro Panel",
                                                          x,
                                                          y);
    const auto clampedMacroCount = juce::jlimit (1, 16, macroCount);
    state.setProperty ("macro_count", clampedMacroCount, nullptr);

    for (int i = 0; i < clampedMacroCount; ++i)
    {
        const auto outputName = "macro_" + juce::String (i + 1);
        state.setProperty (VitParamSurface::valuePropertyForOutput (outputName), 0.0, nullptr);
    }

    return state;
}

juce::var VitMacroNode::createSnapshot (const juce::ValueTree& nodeState)
{
    auto base = VitParamSurface::createNodeSnapshot (nodeState);
    if (auto* object = base.getDynamicObject())
        object->setProperty ("macro_count", static_cast<int> (nodeState.getProperty ("macro_count")));
    return base;
}

} // namespace vit
