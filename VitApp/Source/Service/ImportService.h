#pragma once

#include <functional>

#include <JuceHeader.h>
#include <tracktion_engine/tracktion_engine.h>

#include "TiledSpectrogramBaker.h"

namespace vit
{

namespace te = tracktion;

class ImportService final
{
public:
    using EditGetter = std::function<te::Edit*()>;
    using SaveProjectAction = std::function<bool()>;
    using PublishAction = std::function<void(const juce::String&)>;

    struct AudioImportInsertResult
    {
        bool ok = false;
        juce::String errorMessage;
        juce::String clipId;
        juce::String clipName;
        juce::String trackItemId;
        double startTimeSeconds = 0.0;
        double audioLengthSeconds = 0.0;
        double editLengthSeconds = 0.0;
    };

    ImportService (EditGetter editGetter,
                   SaveProjectAction saveProjectAction,
                   PublishAction publishAction);

    juce::String handleAddAudioClip (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleImportAudio (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleImportMediaToTrack (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleWarmWaveformBake (const juce::DynamicObject&, const juce::String&) const;

    AudioImportInsertResult insertWaveClipWithUndoAndStartBake (
        te::Edit& edit,
        te::AudioTrack& targetTrack,
        const juce::File& sourceFile,
        double startTimeSeconds,
        double audioLengthSeconds,
        bool deleteExistingClips,
        const juce::String& appliedMode) const;

private:
    static juce::String makeStatusReply (const juce::String& status, const juce::String& message);
    static juce::String makeErrorReply (const juce::String& message);

    EditGetter getEdit;
    SaveProjectAction saveProject;
    PublishAction publishMessage;
};

} // namespace vit
