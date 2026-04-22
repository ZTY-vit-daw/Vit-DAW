#include "VitParamLinkGraph.h"

#include "VitMacroNode.h"
#include "VitParamBinding.h"
#include "VitParamSurface.h"

namespace vit
{

namespace
{

constexpr auto kGraphStateType = "VIT_PARAM_LINK_GRAPH";
constexpr auto kSurfaceStateType = "VIT_PARAM_SURFACE";
constexpr auto kControlNodeStateType = "VIT_CONTROL_NODE";
constexpr auto kBindingStateType = "VIT_PARAM_BINDING";

juce::ValueTree findChildByType (const juce::ValueTree& parent, const juce::Identifier& type)
{
    for (int i = 0; i < parent.getNumChildren(); ++i)
        if (auto child = parent.getChild (i); child.hasType (type))
            return child;

    return {};
}

juce::Array<juce::var> createSupportedControlKindsArray()
{
    juce::Array<juce::var> array;
    for (const auto& kind : VitParamSurface::supportedControlKinds())
        array.add (kind);
    return array;
}

juce::var snapshotSurface (const juce::ValueTree& surfaceState)
{
    auto object = std::make_unique<juce::DynamicObject>();
    object->setProperty ("surface_id", surfaceState.getProperty ("surface_id").toString());
    object->setProperty ("name", surfaceState.getProperty ("name").toString());
    object->setProperty ("kind", surfaceState.getProperty ("kind").toString());
    return juce::var (object.release());
}

} // namespace

juce::ValueTree VitParamLinkGraph::ensureGraphState (te::Edit& edit, juce::UndoManager* undoManager)
{
    auto graphState = findChildByType (edit.state, kGraphStateType);
    if (graphState.isValid())
        return graphState;

    graphState = juce::ValueTree (kGraphStateType);
    graphState.setProperty ("graph_id", "control_graph", undoManager);
    graphState.setProperty ("domain", "control", undoManager);

    juce::ValueTree surfaceState (kSurfaceStateType);
    surfaceState.setProperty ("surface_id", "main_surface", undoManager);
    surfaceState.setProperty ("name", "Main Control Surface", undoManager);
    surfaceState.setProperty ("kind", "macro_panel", undoManager);
    graphState.addChild (surfaceState, -1, undoManager);

    edit.state.addChild (graphState, -1, undoManager);
    return findChildByType (edit.state, kGraphStateType);
}

juce::var VitParamLinkGraph::createSnapshot (te::Edit& edit)
{
    auto graphState = findChildByType (edit.state, kGraphStateType);
    juce::Array<juce::var> nodes;
    juce::Array<juce::var> bindings;
    juce::Array<juce::var> surfaces;

    for (int i = 0; i < graphState.getNumChildren(); ++i)
    {
        auto child = graphState.getChild (i);

        if (child.hasType (kControlNodeStateType))
        {
            const auto kind = VitParamSurface::normaliseControlKind (child.getProperty ("kind").toString());
            nodes.add (kind == "macropanel" ? VitMacroNode::createSnapshot (child)
                                             : VitParamSurface::createNodeSnapshot (child));
        }
        else if (child.hasType (kBindingStateType))
        {
            bindings.add (VitParamBinding::createSnapshot (child));
        }
        else if (child.hasType (kSurfaceStateType))
        {
            surfaces.add (snapshotSurface (child));
        }
    }

    auto object = std::make_unique<juce::DynamicObject>();
    object->setProperty ("graph_id", graphState.isValid() ? graphState.getProperty ("graph_id").toString() : "control_graph");
    object->setProperty ("domain", graphState.isValid() ? graphState.getProperty ("domain").toString() : "control");
    object->setProperty ("supported_control_kinds", juce::var (createSupportedControlKindsArray()));
    object->setProperty ("nodes", juce::var (nodes));
    object->setProperty ("bindings", juce::var (bindings));
    object->setProperty ("surfaces", juce::var (surfaces));
    return juce::var (object.release());
}

juce::ValueTree VitParamLinkGraph::addControlNode (te::Edit& edit,
                                                   const juce::String& kind,
                                                   const juce::String& name,
                                                   float x,
                                                   float y,
                                                   juce::UndoManager* undoManager)
{
    auto graphState = ensureGraphState (edit, undoManager);
    auto nodeState = VitParamSurface::createControlNodeState (makeStableId ("control"), kind, name, x, y);
    graphState.addChild (nodeState, -1, undoManager);
    return nodeState;
}

juce::ValueTree VitParamLinkGraph::addMacroNode (te::Edit& edit,
                                                 const juce::String& name,
                                                 float x,
                                                 float y,
                                                 int macroCount,
                                                 juce::UndoManager* undoManager)
{
    auto graphState = ensureGraphState (edit, undoManager);
    auto nodeState = VitMacroNode::createMacroNodeState (makeStableId ("macro"), name, x, y, macroCount);
    graphState.addChild (nodeState, -1, undoManager);
    return nodeState;
}

juce::ValueTree VitParamLinkGraph::addBinding (te::Edit& edit,
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
                                               juce::UndoManager* undoManager)
{
    auto graphState = ensureGraphState (edit, undoManager);
    auto bindingState = VitParamBinding::createBindingState (makeStableId ("binding"),
                                                             sourceNodeId,
                                                             sourceOutput,
                                                             targetKind,
                                                             targetPluginId,
                                                             targetParamId,
                                                             targetNodeId,
                                                             targetInput,
                                                             minValue,
                                                             maxValue,
                                                             curve);
    graphState.addChild (bindingState, -1, undoManager);
    return bindingState;
}

bool VitParamLinkGraph::hasControlNode (te::Edit& edit, const juce::String& nodeId)
{
    return findControlNode (edit, nodeId).isValid();
}

juce::ValueTree VitParamLinkGraph::findControlNode (te::Edit& edit, const juce::String& nodeId)
{
    auto graphState = findChildByType (edit.state, kGraphStateType);
    if (! graphState.isValid())
        return {};

    for (int i = 0; i < graphState.getNumChildren(); ++i)
    {
        auto child = graphState.getChild (i);
        if (child.hasType (kControlNodeStateType) && child.getProperty ("node_id").toString() == nodeId)
            return child;
    }

    return {};
}

juce::ValueTree VitParamLinkGraph::findBinding (te::Edit& edit, const juce::String& bindingId)
{
    auto graphState = findChildByType (edit.state, kGraphStateType);
    if (! graphState.isValid())
        return {};

    for (int i = 0; i < graphState.getNumChildren(); ++i)
    {
        auto child = graphState.getChild (i);
        if (child.hasType (kBindingStateType) && child.getProperty ("binding_id").toString() == bindingId)
            return child;
    }

    return {};
}

juce::Array<juce::ValueTree> VitParamLinkGraph::getAllBindings (te::Edit& edit)
{
    juce::Array<juce::ValueTree> bindings;
    auto graphState = findChildByType (edit.state, kGraphStateType);

    if (! graphState.isValid())
        return bindings;

    for (int i = 0; i < graphState.getNumChildren(); ++i)
    {
        auto child = graphState.getChild (i);
        if (child.hasType (kBindingStateType))
            bindings.add (child);
    }

    return bindings;
}

juce::Array<juce::ValueTree> VitParamLinkGraph::getBindingsForSource (te::Edit& edit,
                                                                      const juce::String& sourceNodeId,
                                                                      const juce::String& sourceOutput)
{
    juce::Array<juce::ValueTree> bindings;
    const auto normalizedOutput = sourceOutput.trim().isNotEmpty() ? sourceOutput.trim() : "value";

    for (const auto& child : getAllBindings (edit))
    {
        if (child.getProperty ("source_node_id").toString() == sourceNodeId
            && child.getProperty ("source_output").toString() == normalizedOutput)
            bindings.add (child);
    }

    return bindings;
}

juce::Array<juce::ValueTree> VitParamLinkGraph::getBindingsReferencingNode (te::Edit& edit, const juce::String& nodeId)
{
    juce::Array<juce::ValueTree> bindings;

    for (const auto& child : getAllBindings (edit))
    {
        if (child.getProperty ("source_node_id").toString() == nodeId
            || child.getProperty ("target_node_id").toString() == nodeId)
            bindings.add (child);
    }

    return bindings;
}

double VitParamLinkGraph::getNodeOutputValue (const juce::ValueTree& nodeState, const juce::String& outputName)
{
    return static_cast<double> (nodeState.getProperty (VitParamSurface::valuePropertyForOutput (outputName)));
}

void VitParamLinkGraph::setNodeOutputValue (juce::ValueTree& nodeState,
                                            const juce::String& outputName,
                                            double value,
                                            juce::UndoManager* undoManager)
{
    const auto normalizedOutput = outputName.trim().isNotEmpty() ? outputName.trim() : "value";
    nodeState.setProperty (VitParamSurface::valuePropertyForOutput (normalizedOutput), value, undoManager);

    if (normalizedOutput == "value")
        nodeState.setProperty ("current_value", value, undoManager);
}

bool VitParamLinkGraph::removeBinding (te::Edit& edit, const juce::String& bindingId, juce::UndoManager* undoManager)
{
    auto bindingState = findBinding (edit, bindingId);
    if (! bindingState.isValid())
        return false;

    auto graphState = bindingState.getParent();
    if (! graphState.isValid())
        return false;

    graphState.removeChild (bindingState, undoManager);
    return true;
}

int VitParamLinkGraph::removeControlNodeAndBindings (te::Edit& edit, const juce::String& nodeId, juce::UndoManager* undoManager)
{
    auto nodeState = findControlNode (edit, nodeId);
    if (! nodeState.isValid())
        return 0;

    auto graphState = nodeState.getParent();
    if (! graphState.isValid())
        return 0;

    const auto relatedBindings = getBindingsReferencingNode (edit, nodeId);
    int removedCount = 0;

    for (const auto& bindingState : relatedBindings)
    {
        if (! bindingState.isValid())
            continue;

        auto parent = bindingState.getParent();
        if (! parent.isValid())
            continue;

        parent.removeChild (bindingState, undoManager);
        ++removedCount;
    }

    graphState.removeChild (nodeState, undoManager);
    return removedCount + 1;
}

void VitParamLinkGraph::updateBinding (juce::ValueTree& bindingState,
                                       const juce::NamedValueSet& updates,
                                       juce::UndoManager* undoManager)
{
    for (int i = 0; i < updates.size(); ++i)
        bindingState.setProperty (updates.getName (i), updates.getValueAt (i), undoManager);
}

void VitParamLinkGraph::updateNodeLayoutAndRange (juce::ValueTree& nodeState,
                                                  const juce::NamedValueSet& updates,
                                                  juce::UndoManager* undoManager)
{
    for (int i = 0; i < updates.size(); ++i)
        nodeState.setProperty (updates.getName (i), updates.getValueAt (i), undoManager);

    const auto minValue = static_cast<double> (nodeState.getProperty ("min"));
    const auto maxValue = static_cast<double> (nodeState.getProperty ("max"));
    const auto clampedDefault = juce::jlimit (minValue, maxValue, static_cast<double> (nodeState.getProperty ("default_value")));
    const auto clampedCurrent = juce::jlimit (minValue, maxValue, static_cast<double> (nodeState.getProperty ("current_value")));
    nodeState.setProperty ("default_value", clampedDefault, undoManager);
    nodeState.setProperty ("current_value", clampedCurrent, undoManager);

    for (int i = 0; i < nodeState.getNumProperties(); ++i)
    {
        const auto propertyIdentifier = nodeState.getPropertyName (i);
        const auto propertyName = propertyIdentifier.toString();
        if (propertyName == "default_value" || propertyName == "current_value" || ! propertyName.endsWithIgnoreCase ("_value"))
            continue;

        const auto clampedValue = juce::jlimit (minValue,
                                                maxValue,
                                                static_cast<double> (nodeState.getProperty (propertyIdentifier)));
        nodeState.setProperty (propertyIdentifier, clampedValue, undoManager);
    }
}

juce::String VitParamLinkGraph::makeStableId (const juce::String& prefix)
{
    return prefix + "_" + juce::Uuid().toString();
}

} // namespace vit
