#pragma once

#include <JuceHeader.h>
#include <tracktion_engine/tracktion_engine.h>

namespace vit
{

namespace te = tracktion;

class VitClipRouteRegistry final
{
public:
    static juce::String normaliseClipScope (const juce::String& clipScope);
    static juce::Result validateClipScope (te::Track&, const juce::String& clipScope);
    static juce::String getNodeClipScope (te::RackType&, te::EditItemID nodeId);
    static juce::Array<juce::var> createClipProxyNodes (te::Track&, const juce::String& requestScope);
    static juce::Array<juce::var> createClipRoutes (te::Track&, te::RackType&, const juce::String& requestScope);
    static void annotateNodesAndEdges (te::Track&, te::RackType&, juce::Array<juce::var>& nodes, juce::Array<juce::var>& edges, const juce::String& requestScope);
};

} // namespace vit
