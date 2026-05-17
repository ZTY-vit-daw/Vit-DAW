#pragma once

#include <functional>

#include <JuceHeader.h>
#include <tracktion_engine/tracktion_engine.h>

namespace vit
{

namespace te = tracktion;

class ImportService;

class GeneratedAssetService final
{
public:
    using EditGetter = std::function<te::Edit*()>;
    using SaveProjectAction = std::function<bool()>;
    using CurrentProjectPathGetter = std::function<juce::String()>;

    GeneratedAssetService (EditGetter editGetter,
                           SaveProjectAction saveProjectAction,
                           CurrentProjectPathGetter currentProjectPathGetter,
                           ImportService* importService);

    juce::String handleBridgeIngestGeneratedAsset (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleSwitchAssetTake (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleSetAsyncGhostState (const juce::DynamicObject&, const juce::String&) const;

private:
    static juce::String makeStatusReply (const juce::String& status, const juce::String& message);
    static juce::String makeErrorReply (const juce::String& message);

    EditGetter getEdit;
    SaveProjectAction saveProject;
    CurrentProjectPathGetter getCurrentProjectPath;
    ImportService* importService = nullptr;
};

} // namespace vit
