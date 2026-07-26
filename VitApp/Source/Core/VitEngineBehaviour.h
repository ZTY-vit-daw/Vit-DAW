#pragma once

#include <JuceHeader.h>
#include <tracktion_engine/tracktion_engine.h>

namespace vit
{

namespace te = tracktion;

/**
 * Routes new audio recordings under Workspace/Cache/Recordings (see VitPaths).
 */
class VitEngineBehaviour final : public te::EngineBehaviour
{
public:
    VitEngineBehaviour() = default;

    bool canScanPluginsOutOfProcess() override { return true; }
    juce::File getDefaultFolderForAudioRecordings (te::Edit& edit) override;
    juce::File getFileForNewAudioRecording (te::Track& track, const juce::String& fileExtension) override;

    JUCE_DECLARE_NON_COPYABLE_WITH_LEAK_DETECTOR (VitEngineBehaviour)
};

} // namespace vit
