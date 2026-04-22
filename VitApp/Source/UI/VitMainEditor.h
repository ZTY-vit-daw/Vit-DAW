#pragma once

#include <JuceHeader.h>
#include <tracktion_engine/tracktion_engine.h>

#include "../Bridge/VitSemanticBridge.h"
#include "../Core/VitEngineDevice.h"

class EditComponent;

namespace vit
{

namespace te = tracktion;

class VitMainEditor final : public juce::Component,
                            private VitSemanticBridge
{
public:
    explicit VitMainEditor (VitEngineDevice& engineDevice);
    ~VitMainEditor() override;

    void paint (juce::Graphics&) override;
    void resized() override;

private:
    void reloadEditFromDefaultXml();
    void setStatusMessage (const juce::String& message);

    void onEditReloaded (const juce::File& xmlFile) override;
    void onGhostTrackDetected (const GhostTrackDescriptor& descriptor) override;

    VitEngineDevice& engineDevice;
    te::SelectionManager selectionManager;
    std::unique_ptr<te::Edit> edit;
    std::unique_ptr<EditComponent> editComponent;

    juce::TextButton reloadButton { "Reload XML" };
    juce::TextButton audioSettingsButton { "Audio Settings" };
    juce::Label statusLabel;
    juce::String lastGhostSummary;

    JUCE_DECLARE_NON_COPYABLE_WITH_LEAK_DETECTOR (VitMainEditor)
};

} // namespace vit
