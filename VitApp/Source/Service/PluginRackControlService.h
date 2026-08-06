#pragma once

#include <functional>
#include <memory>

#include <JuceHeader.h>
#include <tracktion_engine/tracktion_engine.h>

namespace vit
{

namespace te = tracktion;

class PluginScanCoordinator;

class PluginRackControlService final
{
public:
    using EditGetter = std::function<te::Edit*()>;
    using SaveProjectAction = std::function<bool()>;
    using CurrentProjectPathGetter = std::function<juce::String()>;

    PluginRackControlService (EditGetter editGetter,
                              SaveProjectAction saveProjectAction,
                              CurrentProjectPathGetter currentProjectPathGetter);
    ~PluginRackControlService();

    juce::String handleSetPluginParam (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleSetPluginParamAliases (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleControlAddNode (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleControlAddMacro (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleControlAddBinding (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleControlUpdateNode (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleControlSetNodeValue (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleControlUpdateBinding (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleControlRemoveBinding (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleControlRemoveNode (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleControlSetMacroValues (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleConnectorUpsertProfile (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleConnectorRemoveProfile (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleListPlugins (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleSearchPlugins (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleScanPlugins (const juce::DynamicObject&, const juce::String&) const;
    juce::String handlePluginScanStatus (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleCancelPluginScan (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleInstantiatePlugin (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleOpenPluginUI (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleGetPluginParameters (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleDeletePlugin (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleMovePlugin (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleRackAddNode (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleRackConnectPins (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleRackRemoveConnection (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleRackSetNodeClipScope (const juce::DynamicObject&, const juce::String&) const;

private:
    static juce::String makeStatusReply (const juce::String& status, const juce::String& message);
    static juce::String makeErrorReply (const juce::String& message);

    EditGetter getEdit;
    SaveProjectAction saveProject;
    CurrentProjectPathGetter getCurrentProjectPath;
    std::unique_ptr<PluginScanCoordinator> pluginScanCoordinator;
};

} // namespace vit
