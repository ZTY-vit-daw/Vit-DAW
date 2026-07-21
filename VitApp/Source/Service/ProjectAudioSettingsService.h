#pragma once

#include <functional>

#include <JuceHeader.h>
#include <tracktion_engine/tracktion_engine.h>

namespace vit
{

namespace te = tracktion;

class ProjectAudioSettingsService final
{
public:
    using EditGetter = std::function<te::Edit*()>;
    using SaveProjectAction = std::function<bool()>;

    struct EnsureResult
    {
        bool changed = false;
        bool created = false;
        bool patched = false;
    };

    ProjectAudioSettingsService (EditGetter editGetter,
                                 SaveProjectAction saveProjectAction);

    static EnsureResult ensureDefaultAudioSettings (te::Edit& edit,
                                                    const juce::String& origin,
                                                    bool useUndo = false);
    static bool applyAudioSettingsToEdit (te::Edit& edit,
                                          const juce::DynamicObject& source,
                                          const juce::String& origin,
                                          bool useUndo,
                                          juce::Array<juce::var>* warnings = nullptr);

    juce::String handleGetAudioSettings (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleSetAudioSettings (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleValidateAudioSettingsChange (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleImportPreflight (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleInspectFiles (const juce::DynamicObject&, const juce::String&) const;

private:
    static juce::String makeStatusReply (const juce::String& status, const juce::String& message);
    static juce::String makeErrorReply (const juce::String& message);

    EditGetter getEdit;
    SaveProjectAction saveProject;
};

} // namespace vit
