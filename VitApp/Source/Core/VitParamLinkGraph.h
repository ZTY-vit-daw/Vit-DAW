#pragma once

#include <JuceHeader.h>
#include <tracktion_engine/tracktion_engine.h>

namespace vit
{

namespace te = tracktion;

class VitParamLinkGraph final
{
public:
    static juce::ValueTree ensureGraphState (te::Edit& edit, juce::UndoManager* undoManager);
    static juce::var createSnapshot (te::Edit& edit);

    static juce::ValueTree addControlNode (te::Edit& edit,
                                           const juce::String& kind,
                                           const juce::String& name,
                                           float x,
                                           float y,
                                           juce::UndoManager* undoManager);
    static juce::ValueTree addMacroNode (te::Edit& edit,
                                         const juce::String& name,
                                         float x,
                                         float y,
                                         int macroCount,
                                         juce::UndoManager* undoManager);
    static juce::ValueTree addBinding (te::Edit& edit,
                                       const juce::String& sourceNodeId,
                                       const juce::String& sourceOutput,
                                       const juce::String& targetKind,
                                       const juce::String& targetPluginId,
                                       const juce::String& targetParamId,
                                       const juce::String& targetNodeId,
                                       const juce::String& targetInput,
                                       double minValue,
                                       double maxValue,
                                       const juce::String& curve,
                                       juce::UndoManager* undoManager);

    static bool hasControlNode (te::Edit& edit, const juce::String& nodeId);
    static juce::ValueTree findControlNode (te::Edit& edit, const juce::String& nodeId);
    static juce::ValueTree findBinding (te::Edit& edit, const juce::String& bindingId);
    static juce::Array<juce::ValueTree> getAllBindings (te::Edit& edit);
    static juce::Array<juce::ValueTree> getBindingsForSource (te::Edit& edit,
                                                              const juce::String& sourceNodeId,
                                                              const juce::String& sourceOutput);
    static juce::Array<juce::ValueTree> getBindingsReferencingNode (te::Edit& edit, const juce::String& nodeId);
    static double getNodeOutputValue (const juce::ValueTree& nodeState, const juce::String& outputName);
    static void setNodeOutputValue (juce::ValueTree& nodeState,
                                    const juce::String& outputName,
                                    double value,
                                    juce::UndoManager* undoManager);
    static bool removeBinding (te::Edit& edit, const juce::String& bindingId, juce::UndoManager* undoManager);
    static int removeControlNodeAndBindings (te::Edit& edit, const juce::String& nodeId, juce::UndoManager* undoManager);
    static void updateBinding (juce::ValueTree& bindingState,
                               const juce::NamedValueSet& updates,
                               juce::UndoManager* undoManager);
    static void updateNodeLayoutAndRange (juce::ValueTree& nodeState,
                                          const juce::NamedValueSet& updates,
                                          juce::UndoManager* undoManager);

private:
    static juce::String makeStableId (const juce::String& prefix);
};

} // namespace vit
