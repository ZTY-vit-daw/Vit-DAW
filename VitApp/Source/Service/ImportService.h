#pragma once

#include <functional>
#include <memory>

#include <JuceHeader.h>
#include <tracktion_engine/tracktion_engine.h>

#include "AudioFeatureService.h"

namespace vit
{

namespace te = tracktion;

class ImportService final : private juce::Timer
{
public:
    using EditGetter = std::function<te::Edit*()>;
    using SaveProjectAction = std::function<bool()>;
    using PublishAction = std::function<void(const juce::String&)>;
    using CurrentProjectPathGetter = std::function<juce::String()>;

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
                   PublishAction publishAction,
                   CurrentProjectPathGetter currentProjectPathGetter = {});

    juce::String handleAddAudioClip (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleImportAudio (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleImportMediaToTrack (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleImportFolderAsStems (const juce::DynamicObject&, const juce::String&);
    juce::String handleAudioAnalysisStart (const juce::DynamicObject&, const juce::String&);
    juce::String handleAudioAnalysisStatus (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleAudioAnalysisCancel (const juce::DynamicObject&, const juce::String&);
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
    struct QueuedAnalysisClip
    {
        juce::String filePath;
        juce::String trackId;
        juce::String clipId;
        double durationSeconds = 0.0;
    };

    struct AudioAnalysisJob
    {
        juce::String jobId;
        juce::String status = "queued";
        juce::Array<QueuedAnalysisClip> clips;
        int nextClipIndex = 0;
        int submittedClips = 0;
        int submittedFeatureJobs = 0;
        int totalFeatureJobs = 0;
        int intervalMs = 750;
        int maxSubmitClips = 0;
        int maxSubmitFeatureJobs = 0;
        int canceledPendingClips = 0;
        juce::Time createdAt;
        juce::Time startedAt;
        juce::Time updatedAt;
        juce::Time finishedAt;
    };

    static juce::String makeStatusReply (const juce::String& status, const juce::String& message);
    static juce::String makeErrorReply (const juce::String& message);

    void timerCallback() override;
    juce::Array<QueuedAnalysisClip> collectCurrentProjectAnalysisClips() const;
    juce::String registerDeferredAudioAnalysisJob (const juce::Array<QueuedAnalysisClip>& clips, bool autoStart);
    AudioAnalysisJob* findAnalysisJob (const juce::String& jobId) const;
    AudioAnalysisJob* findLatestAnalysisJob() const;
    juce::var buildAudioAnalysisJobStatus (const AudioAnalysisJob& job) const;
    juce::String makeAudioAnalysisJobReply (const AudioAnalysisJob& job,
                                            const juce::String& command,
                                            const juce::String& message) const;
    void publishAudioAnalysisProgress (const AudioAnalysisJob& job, const juce::String& event) const;
    void refreshAnalysisTimer();

    EditGetter getEdit;
    SaveProjectAction saveProject;
    PublishAction publishMessage;
    CurrentProjectPathGetter getCurrentProjectPath;
    mutable juce::Array<std::shared_ptr<AudioAnalysisJob>> audioAnalysisJobs;
    mutable int nextAudioAnalysisJobNumber = 1;
};

} // namespace vit
