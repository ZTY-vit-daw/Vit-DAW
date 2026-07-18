#include "ImportService.h"

#include "ProjectAudioSettingsService.h"
#include "WaveformEnvelopeBaker.h"

#include "../Core/VitKernelUtils.h"

#include <algorithm>
#include <cmath>
#include <cstdlib>
#include <memory>
#include <set>
#include <utility>

namespace vit
{

namespace
{

constexpr int kImportAnalysisFeaturesPerClip = 1;
constexpr int kImportAnalysisAutoSubmitIntervalMs = 100;

juce::String describeTransportState (te::Edit& edit)
{
    auto& transport = edit.getTransport();
    return "isPlaying=" + juce::String (transport.isPlaying() ? "true" : "false")
        + " isRecording=" + juce::String (transport.isRecording() ? "true" : "false")
        + " playContextActive=" + juce::String (transport.isPlayContextActive() ? "true" : "false")
        + " positionSeconds=" + juce::String (transport.getPosition().inSeconds(), 3)
        + " editLengthSeconds=" + juce::String (edit.getLength().inSeconds(), 3);
}

te::Clip* findClipByID (te::Edit& edit, const juce::String& clipId)
{
    if (clipId.isEmpty())
        return nullptr;

    for (auto* track : te::getAllTracks (edit))
    {
        if (track == nullptr)
            continue;

        const int n = track->getNumTrackItems();
        for (int i = 0; i < n; ++i)
        {
            auto* item = track->getTrackItem (i);
            auto* clip = dynamic_cast<te::Clip*> (item);
            if (clip != nullptr && clip->itemID.toString() == clipId)
                return clip;
        }
    }
    return nullptr;
}

te::AudioTrack* findAudioTrackByID (te::Edit& edit, const juce::String& trackID)
{
    for (auto* track : te::getAllTracks (edit))
    {
        auto* audioTrack = dynamic_cast<te::AudioTrack*> (track);

        if (audioTrack != nullptr && audioTrack->itemID.toString() == trackID)
            return audioTrack;
    }

    return nullptr;
}

bool importPerfLogEnabled()
{
    static const bool enabled = []
    {
        const auto raw = juce::SystemStats::getEnvironmentVariable ("VIT_IMPORT_PERF_LOG", "1")
            .trim()
            .toLowerCase();
        return raw != "0" && raw != "false" && raw != "off" && raw != "no";
    }();

    return enabled;
}

bool numericVarToDouble (const juce::var& value, double& out)
{
    if (value.isDouble() || value.isInt() || value.isInt64())
    {
        out = static_cast<double> (value);
        return true;
    }

    const auto text = value.toString().trim();
    if (text.isEmpty())
        return false;

    const auto* raw = text.toRawUTF8();
    char* end = nullptr;
    const auto parsed = std::strtod (raw, &end);
    if (end != raw && *end == '\0' && std::isfinite (parsed))
    {
        out = parsed;
        return true;
    }

    return false;
}

bool readIntVar (const juce::var& value, int& out)
{
    double parsed = 0.0;
    if (! numericVarToDouble (value, parsed))
        return false;

    out = juce::roundToInt (parsed);
    return true;
}

bool boolProperty (const juce::DynamicObject& object, const juce::String& key, bool fallback)
{
    const auto value = object.getProperty (key);
    return value.isBool() ? static_cast<bool> (value) : fallback;
}

double doubleProperty (const juce::DynamicObject& object, const juce::String& key, double fallback)
{
    double out = fallback;
    return numericVarToDouble (object.getProperty (key), out) ? out : fallback;
}

juce::String stringProperty (const juce::ValueTree& tree,
                             const juce::Identifier& id,
                             const juce::String& fallback)
{
    const auto value = tree.getProperty (id).toString().trim();
    return value.isNotEmpty() ? value : fallback;
}

int intProperty (const juce::ValueTree& tree, const juce::Identifier& id, int fallback)
{
    int out = fallback;
    return readIntVar (tree.getProperty (id), out) ? out : fallback;
}

juce::String getStringFromObject (const juce::DynamicObject& object,
                                  const juce::String& key,
                                  const juce::String& fallback = {})
{
    const auto value = object.getProperty (key).toString().trim();
    return value.isNotEmpty() ? value : fallback;
}

int getIntFromObject (const juce::DynamicObject& object, const juce::String& key, int fallback = 0)
{
    int out = fallback;
    return readIntVar (object.getProperty (key), out) ? out : fallback;
}

juce::Array<juce::File> filesFromVar (const juce::var& value)
{
    juce::Array<juce::File> files;

    if (auto* arr = value.getArray())
        for (const auto& item : *arr)
            if (item.toString().trim().isNotEmpty())
                files.add (juce::File (item.toString().trim()));

    return files;
}

juce::String lowerExtensionWithoutDot (const juce::File& file)
{
    auto ext = file.getFileExtension().toLowerCase();
    if (ext.startsWithChar ('.'))
        ext = ext.substring (1);
    return ext;
}

bool isSupportedAudioExtension (const juce::File& file)
{
    const auto ext = lowerExtensionWithoutDot (file);
    return ext == "wav" || ext == "aif" || ext == "aiff" || ext == "flac" || ext == "mp3";
}

juce::String suggestedTrackName (const juce::File& file)
{
    auto name = file.getFileNameWithoutExtension().replaceCharacter ('_', ' ').trim();
    while (name.contains ("  "))
        name = name.replace ("  ", " ");
    return name.isNotEmpty() ? name : file.getFileNameWithoutExtension();
}

juce::Array<juce::File> audioFilesInFolder (const juce::File& folder, bool recursive)
{
    juce::Array<juce::File> out;
    std::set<juce::String> seen;

    for (auto pattern : { "*.wav", "*.aif", "*.aiff", "*.flac", "*.mp3" })
    {
        juce::Array<juce::File> matches;
        folder.findChildFiles (matches, juce::File::findFiles, recursive, pattern);

        for (auto file : matches)
        {
            const auto key = file.getFullPathName().toLowerCase();
            if (seen.insert (key).second)
                out.add (file);
        }
    }

    return out;
}

juce::Array<juce::File> uniqueImportFilesFromCommand (const juce::DynamicObject& object)
{
    auto files = filesFromVar (object.getProperty ("file_paths"));

    const auto singleFilePath = getStringFromObject (object, "file_path");
    if (singleFilePath.isNotEmpty())
        files.add (juce::File (singleFilePath));

    const auto folderPath = getStringFromObject (object, "folder_path");
    const bool recursive = boolProperty (object, "recursive", false);
    if (folderPath.isNotEmpty())
    {
        const juce::File folder (folderPath);
        if (folder.isDirectory())
            files.addArray (audioFilesInFolder (folder, recursive));
    }

    std::set<juce::String> seen;
    juce::Array<juce::File> uniqueFiles;
    for (auto file : files)
    {
        const auto key = file.getFullPathName().toLowerCase();
        if (seen.insert (key).second)
            uniqueFiles.add (file);
    }

    return uniqueFiles;
}

struct ImportAudioSettings
{
    int sampleRateHz = 48000;
    int recordBitDepth = 24;
    juce::String importSampleRatePolicy = "ask";
    juce::String importBitDepthPolicy = "keep_source";
    juce::String mediaCopyPolicy = "reference_original";
    juce::String channelImportPolicy = "preserve_interleaved";
};

ImportAudioSettings importSettingsFromEdit (te::Edit& edit)
{
    ProjectAudioSettingsService::ensureDefaultAudioSettings (edit, "import_folder_as_stems");

    const auto tree = edit.state.getChildWithName (juce::Identifier ("VIT_AUDIO_SETTINGS"));
    ImportAudioSettings settings;
    settings.sampleRateHz = intProperty (tree, juce::Identifier ("sample_rate_hz"), settings.sampleRateHz);
    settings.recordBitDepth = intProperty (tree, juce::Identifier ("record_bit_depth"), settings.recordBitDepth);
    settings.importSampleRatePolicy = stringProperty (tree, juce::Identifier ("import_sample_rate_policy"), settings.importSampleRatePolicy);
    settings.importBitDepthPolicy = stringProperty (tree, juce::Identifier ("import_bit_depth_policy"), settings.importBitDepthPolicy);
    settings.mediaCopyPolicy = stringProperty (tree, juce::Identifier ("media_copy_policy"), settings.mediaCopyPolicy);
    settings.channelImportPolicy = stringProperty (tree, juce::Identifier ("channel_import_policy"), settings.channelImportPolicy);
    return settings;
}

ImportAudioSettings settingsWithCommandOverrides (const juce::DynamicObject& object, ImportAudioSettings settings)
{
    if (auto* snapshot = object.getProperty ("audio_settings_snapshot").getDynamicObject())
    {
        settings.sampleRateHz = getIntFromObject (*snapshot, "sample_rate_hz", settings.sampleRateHz);
        settings.recordBitDepth = getIntFromObject (*snapshot, "record_bit_depth", settings.recordBitDepth);
        settings.importSampleRatePolicy = getStringFromObject (*snapshot, "import_sample_rate_policy", settings.importSampleRatePolicy);
        settings.importBitDepthPolicy = getStringFromObject (*snapshot, "import_bit_depth_policy", settings.importBitDepthPolicy);
        settings.mediaCopyPolicy = getStringFromObject (*snapshot, "media_copy_policy", settings.mediaCopyPolicy);
        settings.channelImportPolicy = getStringFromObject (*snapshot, "channel_import_policy", settings.channelImportPolicy);
    }

    settings.mediaCopyPolicy = getStringFromObject (object, "media_copy_policy", settings.mediaCopyPolicy);
    settings.importSampleRatePolicy = getStringFromObject (object, "import_sample_rate_policy", settings.importSampleRatePolicy);
    settings.importBitDepthPolicy = getStringFromObject (object, "import_bit_depth_policy", settings.importBitDepthPolicy);
    settings.channelImportPolicy = getStringFromObject (object, "channel_import_policy", settings.channelImportPolicy);
    return settings;
}

struct ImportCandidate
{
    juce::File sourceFile;
    juce::File insertFile;
    juce::String trackName;
    double durationSeconds = 0.0;
    int sampleRateHz = 0;
    int bitDepth = 0;
    int channelCount = 0;
    juce::String pcmFormat = "unknown";
    bool needsSampleRateConversion = false;
    bool bitDepthOrFormatDiffers = false;
};

juce::var candidateToFileRow (const ImportCandidate& candidate)
{
    auto row = std::make_unique<juce::DynamicObject>();
    row->setProperty ("file_path", candidate.sourceFile.getFullPathName());
    row->setProperty ("file_name", candidate.sourceFile.getFileName());
    row->setProperty ("file_type", lowerExtensionWithoutDot (candidate.sourceFile));
    row->setProperty ("readable", true);
    row->setProperty ("duration_seconds", candidate.durationSeconds);
    row->setProperty ("sample_rate_hz", candidate.sampleRateHz);
    row->setProperty ("bit_depth", candidate.bitDepth > 0 ? juce::var (candidate.bitDepth) : juce::var());
    row->setProperty ("pcm_format", candidate.pcmFormat);
    row->setProperty ("channel_count", candidate.channelCount);
    row->setProperty ("suggested_track_name", candidate.trackName);
    row->setProperty ("needs_sample_rate_conversion", candidate.needsSampleRateConversion);
    row->setProperty ("bit_depth_or_format_differs", candidate.bitDepthOrFormatDiffers);
    return juce::var (row.release());
}

juce::Result inspectImportCandidate (te::Edit& edit,
                                     const juce::File& sourceFile,
                                     const ImportAudioSettings& settings,
                                     ImportCandidate& out)
{
    if (! sourceFile.existsAsFile())
        return juce::Result::fail ("Audio file does not exist: " + sourceFile.getFullPathName());

    if (! isSupportedAudioExtension (sourceFile))
        return juce::Result::fail ("Unsupported audio file type: " + sourceFile.getFullPathName());

    std::unique_ptr<juce::AudioFormatReader> reader (
        edit.engine.getAudioFileFormatManager().readFormatManager.createReaderFor (sourceFile));

    if (reader == nullptr || reader->sampleRate <= 0.0 || reader->lengthInSamples <= 0)
        return juce::Result::fail ("Unsupported or unreadable audio file: " + sourceFile.getFullPathName());

    out.sourceFile = sourceFile;
    out.insertFile = sourceFile;
    out.trackName = suggestedTrackName (sourceFile);
    out.durationSeconds = (double) reader->lengthInSamples / reader->sampleRate;
    out.sampleRateHz = juce::roundToInt (reader->sampleRate);
    out.channelCount = (int) reader->numChannels;

    if (lowerExtensionWithoutDot (sourceFile) == "mp3")
    {
        out.bitDepth = 0;
        out.pcmFormat = "compressed_mp3_decoded_float32";
    }
    else
    {
        out.bitDepth = (int) reader->bitsPerSample;
        out.pcmFormat = reader->usesFloatingPointData ? "float" + juce::String (juce::jmax (out.bitDepth, 32))
                                                      : "int" + juce::String (out.bitDepth);
    }

    out.needsSampleRateConversion = out.sampleRateHz > 0 && out.sampleRateHz != settings.sampleRateHz;
    const bool bitMismatch = out.bitDepth > 0 && out.bitDepth != settings.recordBitDepth;
    const bool floatMismatch = out.pcmFormat.startsWithIgnoreCase ("float")
                            && settings.importBitDepthPolicy == "convert_to_project_format";
    out.bitDepthOrFormatDiffers = bitMismatch || floatMismatch;
    return juce::Result::ok();
}

juce::File projectImportMediaDirectory (const juce::String& currentProjectPath)
{
    const juce::File projectFile (currentProjectPath);
    if (currentProjectPath.trim().isEmpty() || ! projectFile.getParentDirectory().isDirectory())
        return {};

    auto baseName = projectFile.getFileNameWithoutExtension();
    if (baseName.isEmpty())
        baseName = "VitProject";

    return projectFile.getParentDirectory()
        .getChildFile (baseName + "_Media")
        .getChildFile ("Imported");
}

juce::Result applyMediaCopyPolicy (const juce::DynamicObject& object,
                                   const ImportAudioSettings& settings,
                                   const std::function<juce::String()>& getCurrentProjectPath,
                                   juce::Array<ImportCandidate>& candidates,
                                   juce::Array<juce::var>& copiedMediaRows)
{
    const auto policy = settings.mediaCopyPolicy.trim().toLowerCase();
    if (policy != "copy_to_project_media")
        return juce::Result::ok();

    juce::File destDir;
    const auto explicitDest = getStringFromObject (object, "destination_media_folder");
    if (explicitDest.isNotEmpty())
        destDir = juce::File (explicitDest);
    else if (getCurrentProjectPath != nullptr)
        destDir = projectImportMediaDirectory (getCurrentProjectPath());

    if (destDir.getFullPathName().trim().isEmpty())
        return juce::Result::fail ("media_copy_policy=copy_to_project_media requires a saved project path or destination_media_folder");

    if (! destDir.exists() && ! destDir.createDirectory())
        return juce::Result::fail ("Failed to create import media folder: " + destDir.getFullPathName());

    for (auto& candidate : candidates)
    {
        const auto hash = juce::String::toHexString (static_cast<juce::int64> (candidate.sourceFile.hashCode64()));
        const auto targetName = candidate.sourceFile.getFileNameWithoutExtension()
                              + "_"
                              + hash
                              + candidate.sourceFile.getFileExtension();
        const auto destFile = destDir.getChildFile (targetName);

        if (! destFile.existsAsFile() && ! candidate.sourceFile.copyFileTo (destFile))
            return juce::Result::fail ("Failed to copy media into project media folder: " + candidate.sourceFile.getFullPathName());

        candidate.insertFile = destFile;

        auto row = std::make_unique<juce::DynamicObject>();
        row->setProperty ("source_file_path", candidate.sourceFile.getFullPathName());
        row->setProperty ("copied_file_path", destFile.getFullPathName());
        row->setProperty ("media_copy_policy", "copy_to_project_media");
        copiedMediaRows.add (juce::var (row.release()));
    }

    return juce::Result::ok();
}

juce::var buildImportSettingsSnapshot (const ImportAudioSettings& settings)
{
    auto out = std::make_unique<juce::DynamicObject>();
    out->setProperty ("sample_rate_hz", settings.sampleRateHz);
    out->setProperty ("record_bit_depth", settings.recordBitDepth);
    out->setProperty ("import_sample_rate_policy", settings.importSampleRatePolicy);
    out->setProperty ("import_bit_depth_policy", settings.importBitDepthPolicy);
    out->setProperty ("media_copy_policy", settings.mediaCopyPolicy);
    out->setProperty ("channel_import_policy", settings.channelImportPolicy);
    return juce::var (out.release());
}

juce::String importPerfMs (double value)
{
    return juce::String (value, 2);
}

void logImportPerf (const juce::String& operation,
                    const juce::String& stage,
                    double stageMs,
                    double totalMs,
                    const juce::String& details = {})
{
    if (! importPerfLogEnabled())
        return;

    juce::Logger::writeToLog ("[VitImportPerf] op=" + operation
                              + " stage=" + stage
                              + " stage_ms=" + importPerfMs (stageMs)
                              + " total_ms=" + importPerfMs (totalMs)
                              + (details.isNotEmpty() ? " " + details : juce::String()));
}

class ImportPerfTrace
{
public:
    ImportPerfTrace (juce::String operationIn, juce::String trackIdIn, juce::String filePathIn)
        : operation (std::move (operationIn)),
          trackId (std::move (trackIdIn)),
          fileName (juce::File (filePathIn).getFileName()),
          totalStartMs (juce::Time::getMillisecondCounterHiRes()),
          lastStageMs (totalStartMs)
    {
        mark ("begin", "track_id=" + trackId + " file=\"" + fileName + "\"");
    }

    void mark (const juce::String& stage, const juce::String& details = {})
    {
        const auto nowMs = juce::Time::getMillisecondCounterHiRes();
        logImportPerf (operation, stage, nowMs - lastStageMs, nowMs - totalStartMs, details);
        lastStageMs = nowMs;
    }

    void finish (const juce::String& status, const juce::String& details = {})
    {
        mark ("finish", "status=" + status + (details.isNotEmpty() ? " " + details : juce::String()));
    }

private:
    juce::String operation;
    juce::String trackId;
    juce::String fileName;
    double totalStartMs = 0.0;
    double lastStageMs = 0.0;
};

bool ensureMonitoringPlugins (te::AudioTrack& track)
{
    auto& edit = track.pluginList.getEdit();
    bool changed = false;

    if (track.getVolumePlugin() == nullptr)
    {
        auto plugin = edit.getPluginCache().createNewPlugin (te::VolumeAndPanPlugin::xmlTypeName, {});

        if (plugin != nullptr)
        {
            track.pluginList.insertPlugin (plugin, -1, nullptr);
            changed = true;
        }
    }

    if (track.getLevelMeterPlugin() == nullptr)
    {
        auto plugin = edit.getPluginCache().createNewPlugin (te::LevelMeterPlugin::xmlTypeName, {});

        if (plugin != nullptr)
        {
            track.pluginList.insertPlugin (plugin, -1, nullptr);
            changed = true;
        }
    }

    if (changed)
    {
        track.flushStateToValueTree();
        edit.getTransport().ensureContextAllocated (true);
        edit.dispatchPendingUpdatesSynchronously();
    }

    return changed;
}

juce::String sourceRevisionForImport (const juce::File& sourceFile, double audioLengthSeconds)
{
    return sourceFile.getFullPathName()
        + "|size=" + juce::String ((int64) sourceFile.getSize())
        + "|mtime=" + juce::String ((int64) sourceFile.getLastModificationTime().toMilliseconds())
        + "|length=" + juce::String (audioLengthSeconds, 4);
}

juce::String clipRevisionForFeatureRequest (const juce::String& trackId,
                                            const juce::String& clipId,
                                            const juce::String& sourceRevision,
                                            double sourceOffsetSeconds,
                                            double lengthSeconds)
{
    if (clipId.trim().isEmpty())
        return {};

    return "clip=" + clipId.trim()
        + "|track=" + trackId.trim()
        + "|source=" + sourceRevision.trim()
        + "|offset=" + juce::String (sourceOffsetSeconds, 4)
        + "|length=" + juce::String (lengthSeconds, 4);
}

void requestImportAcousticPackages (const juce::File& sourceFile,
                                    const juce::String& trackId,
                                    const juce::String& clipId,
                                    double audioLengthSeconds,
                                    const AudioFeatureService::PublishCallback& publish)
{
    const auto sourceRevision = sourceRevisionForImport (sourceFile, audioLengthSeconds);
    const auto clipRevision = clipRevisionForFeatureRequest (trackId, clipId, sourceRevision, 0.0, audioLengthSeconds);

    AudioFeatureBakeRequest waveformRequest;
    waveformRequest.filePath = sourceFile.getFullPathName();
    waveformRequest.trackId = trackId;
    waveformRequest.clipId = clipId;
    waveformRequest.sourceId = sourceFile.getFullPathName();
    waveformRequest.sourceRevision = sourceRevision;
    waveformRequest.clipRevision = clipRevision;
    waveformRequest.featureType = AudioFeatureType::WaveformEnvelope;
    waveformRequest.priority = AudioFeaturePriority::ImportImmediate;
    waveformRequest.range.lengthSeconds = audioLengthSeconds;
    waveformRequest.resolution.frameWidth = 1024;
    AudioFeatureService::requestBake (std::move (waveformRequest), publish);

    // Timeline import stays lightweight. Heavy spectral/L3 analysis is requested
    // explicitly by detail/mixboard views through warm_waveform_bake.
}

} // namespace

ImportService::ImportService (EditGetter editGetter,
                              SaveProjectAction saveProjectAction,
                              PublishAction publishAction,
                              CurrentProjectPathGetter currentProjectPathGetter)
    : getEdit (std::move (editGetter)),
      saveProject (std::move (saveProjectAction)),
      publishMessage (std::move (publishAction)),
      getCurrentProjectPath (std::move (currentProjectPathGetter))
{
}

juce::Array<ImportService::QueuedAnalysisClip> ImportService::collectCurrentProjectAnalysisClips() const
{
    juce::Array<QueuedAnalysisClip> clips;
    auto* edit = getEdit != nullptr ? getEdit() : nullptr;
    if (edit == nullptr)
        return clips;

    for (auto* track : te::getAllTracks (*edit))
    {
        auto* audioTrack = dynamic_cast<te::AudioTrack*> (track);
        if (audioTrack == nullptr)
            continue;

        for (auto* clip : audioTrack->getClips())
        {
            auto* audioClip = dynamic_cast<te::AudioClipBase*> (clip);
            if (audioClip == nullptr)
                continue;

            auto sourceFile = audioClip->getCurrentSourceFile();
            if (! sourceFile.existsAsFile())
                sourceFile = audioClip->getOriginalFile();
            if (! sourceFile.existsAsFile())
                continue;

            double durationSeconds = 0.0;
            std::unique_ptr<juce::AudioFormatReader> reader (
                edit->engine.getAudioFileFormatManager().readFormatManager.createReaderFor (sourceFile));
            if (reader != nullptr && reader->sampleRate > 0.0 && reader->lengthInSamples > 0)
                durationSeconds = (double) reader->lengthInSamples / reader->sampleRate;
            if (durationSeconds <= 0.0)
                durationSeconds = juce::jmax (0.0, clip->getPosition().getLength().inSeconds());

            QueuedAnalysisClip queued;
            queued.filePath = sourceFile.getFullPathName();
            queued.trackId = audioTrack->itemID.toString();
            queued.clipId = clip->itemID.toString();
            queued.durationSeconds = durationSeconds;
            clips.add (queued);
            break;
        }
    }
    return clips;
}

juce::String ImportService::registerDeferredAudioAnalysisJob (const juce::Array<QueuedAnalysisClip>& clips, bool autoStart)
{
    if (clips.isEmpty())
        return {};

    auto job = std::make_shared<AudioAnalysisJob>();
    job->jobId = "audio_analysis_" + juce::String (juce::Time::currentTimeMillis())
               + "_" + juce::String (nextAudioAnalysisJobNumber++);
    job->status = "queued";
    job->clips = clips;
    job->totalFeatureJobs = clips.size() * kImportAnalysisFeaturesPerClip;
    job->createdAt = juce::Time::getCurrentTime();
    job->updatedAt = job->createdAt;
    if (autoStart)
    {
        job->status = "running";
        job->intervalMs = kImportAnalysisAutoSubmitIntervalMs;
        job->startedAt = job->createdAt;
    }
    audioAnalysisJobs.add (job);
    publishAudioAnalysisProgress (*job, autoStart ? "started" : "queued");
    refreshAnalysisTimer();
    return job->jobId;
}

ImportService::AudioAnalysisJob* ImportService::findAnalysisJob (const juce::String& jobId) const
{
    const auto wanted = jobId.trim();
    if (wanted.isEmpty())
        return nullptr;

    for (auto& job : audioAnalysisJobs)
        if (job != nullptr && job->jobId == wanted)
            return job.get();

    return nullptr;
}

ImportService::AudioAnalysisJob* ImportService::findLatestAnalysisJob() const
{
    for (int i = audioAnalysisJobs.size() - 1; i >= 0; --i)
        if (auto job = audioAnalysisJobs.getReference (i))
            return job.get();

    return nullptr;
}

juce::var ImportService::buildAudioAnalysisJobStatus (const AudioAnalysisJob& job) const
{
    auto status = std::make_unique<juce::DynamicObject>();
    const auto totalClips = job.clips.size();
    const auto pendingClips = juce::jmax (0, totalClips - job.nextClipIndex);
    const auto totalFeatureJobs = juce::jmax (0, job.totalFeatureJobs);
    const auto progress = totalFeatureJobs > 0
        ? juce::jlimit (0.0, 1.0, (double) job.submittedFeatureJobs / (double) totalFeatureJobs)
        : 1.0;
    juce::Array<juce::var> waveformRows;
    int dadFactReadyCount = 0;
    int dadFactPartialCount = 0;
    int dadFactFailedCount = 0;
    for (const auto& clip : job.clips)
    {
        auto row = WaveformEnvelopeBaker::getLatestBakeStatus (clip.trackId, clip.clipId);
        if (auto* rowObject = row.getDynamicObject())
        {
            rowObject->setProperty ("analysis_job_id", job.jobId);
            rowObject->setProperty ("job_id", job.jobId);
            if (! rowObject->hasProperty ("duration_seconds") && clip.durationSeconds > 0.0)
                rowObject->setProperty ("duration_seconds", clip.durationSeconds);
            if (! rowObject->hasProperty ("file_path") && clip.filePath.isNotEmpty())
                rowObject->setProperty ("file_path", clip.filePath);
            if (! rowObject->hasProperty ("source_path") && clip.filePath.isNotEmpty())
                rowObject->setProperty ("source_path", clip.filePath);
            const auto rowStatus = rowObject->getProperty ("status").toString();
            if (rowStatus == "ready")
                ++dadFactReadyCount;
            else if (rowStatus == "partial" || rowStatus == "building" || rowStatus == "requested")
                ++dadFactPartialCount;
            else if (rowStatus == "failed" || rowStatus == "error")
                ++dadFactFailedCount;
        }
        waveformRows.add (row);
    }
    const auto dadFactTotalCount = totalClips;
    const auto dadFactPendingCount = juce::jmax (0, dadFactTotalCount - dadFactReadyCount - dadFactFailedCount);
    juce::String dadFactStatus = "missing";
    if (dadFactTotalCount > 0 && dadFactReadyCount >= dadFactTotalCount)
        dadFactStatus = "ready";
    else if (dadFactReadyCount > 0 || dadFactPartialCount > 0)
        dadFactStatus = "partial";
    else if (dadFactFailedCount > 0)
        dadFactStatus = "failed";

    status->setProperty ("analysis_job_id", job.jobId);
    status->setProperty ("job_id", job.jobId);
    status->setProperty ("status", job.status);
    status->setProperty ("analysis_queue_status", job.status);
    status->setProperty ("total_clips", totalClips);
    status->setProperty ("submitted_clips", job.submittedClips);
    status->setProperty ("pending_clips", pendingClips);
    status->setProperty ("total_feature_jobs", totalFeatureJobs);
    status->setProperty ("submitted_feature_jobs", job.submittedFeatureJobs);
    status->setProperty ("pending_feature_jobs", job.status == "cancelled" ? 0 : juce::jmax (0, totalFeatureJobs - job.submittedFeatureJobs));
    status->setProperty ("canceled_feature_jobs", job.canceledPendingClips * kImportAnalysisFeaturesPerClip);
    status->setProperty ("progress", progress);
    status->setProperty ("progress_percent", progress * 100.0);
    status->setProperty ("dad_fact_status", dadFactStatus);
    status->setProperty ("dad_fact_ready_count", dadFactReadyCount);
    status->setProperty ("dad_fact_total_count", dadFactTotalCount);
    status->setProperty ("dad_fact_pending_count", dadFactPendingCount);
    status->setProperty ("dad_fact_failed_count", dadFactFailedCount);
    status->setProperty ("dad_fact_completion_scope", "waveform_baker_latest_status");
    status->setProperty ("track_waveform_envelopes", juce::var (waveformRows));
    status->setProperty ("interval_ms", job.intervalMs);
    status->setProperty ("max_submit_clips", job.maxSubmitClips);
    status->setProperty ("max_submit_feature_jobs", job.maxSubmitFeatureJobs);
    status->setProperty ("canceled_pending_clips", job.canceledPendingClips);
    status->setProperty ("cancel_scope", "pending_only");
    status->setProperty ("completion_scope", "submitted_to_background_bakers");
    status->setProperty ("created_at_ms", (juce::int64) job.createdAt.toMilliseconds());
    if (job.startedAt.toMilliseconds() > 0)
        status->setProperty ("started_at_ms", (juce::int64) job.startedAt.toMilliseconds());
    if (job.updatedAt.toMilliseconds() > 0)
        status->setProperty ("updated_at_ms", (juce::int64) job.updatedAt.toMilliseconds());
    if (job.finishedAt.toMilliseconds() > 0)
        status->setProperty ("finished_at_ms", (juce::int64) job.finishedAt.toMilliseconds());
    return juce::var (status.release());
}

juce::String ImportService::makeAudioAnalysisJobReply (const AudioAnalysisJob& job,
                                                       const juce::String& command,
                                                       const juce::String& message) const
{
    auto response = std::make_unique<juce::DynamicObject>();
    response->setProperty ("status", "ok");
    response->setProperty ("command", command);
    response->setProperty ("message", message);
    response->setProperty ("analysis_job", buildAudioAnalysisJobStatus (job));
    response->setProperty ("analysis_job_id", job.jobId);
    response->setProperty ("job_id", job.jobId);
    response->setProperty ("analysis_queue_status", job.status);
    response->setProperty ("total_clips", job.clips.size());
    response->setProperty ("submitted_clips", job.submittedClips);
    response->setProperty ("total_feature_jobs", job.totalFeatureJobs);
    response->setProperty ("submitted_feature_jobs", job.submittedFeatureJobs);
    return juce::JSON::toString (juce::var (response.release()));
}

void ImportService::publishAudioAnalysisProgress (const AudioAnalysisJob& job, const juce::String& event) const
{
    if (! publishMessage)
        return;

    auto message = std::make_unique<juce::DynamicObject>();
    message->setProperty ("command", "project_audio_analysis_progress");
    message->setProperty ("feature_family", "audio_feature");
    message->setProperty ("event", event);
    message->setProperty ("analysis_job", buildAudioAnalysisJobStatus (job));
    message->setProperty ("analysis_job_id", job.jobId);
    message->setProperty ("job_id", job.jobId);
    message->setProperty ("analysis_queue_status", job.status);
    message->setProperty ("total_clips", job.clips.size());
    message->setProperty ("submitted_clips", job.submittedClips);
    message->setProperty ("total_feature_jobs", job.totalFeatureJobs);
    message->setProperty ("submitted_feature_jobs", job.submittedFeatureJobs);
    publishMessage (juce::JSON::toString (juce::var (message.release())));
}

void ImportService::refreshAnalysisTimer()
{
    int intervalMs = 0;
    for (auto& job : audioAnalysisJobs)
    {
        if (job != nullptr && job->status == "running")
        {
            const auto candidate = juce::jlimit (50, 10000, job->intervalMs);
            intervalMs = intervalMs == 0 ? candidate : juce::jmin (intervalMs, candidate);
        }
    }

    if (intervalMs > 0)
        startTimer (intervalMs);
    else
        stopTimer();
}

void ImportService::timerCallback()
{
    AudioAnalysisJob* job = nullptr;
    for (auto& candidate : audioAnalysisJobs)
    {
        if (candidate != nullptr && candidate->status == "running")
        {
            job = candidate.get();
            break;
        }
    }

    if (job == nullptr)
    {
        refreshAnalysisTimer();
        return;
    }

    if ((job->maxSubmitClips > 0 && job->submittedClips >= job->maxSubmitClips)
        || (job->maxSubmitFeatureJobs > 0 && job->submittedFeatureJobs >= job->maxSubmitFeatureJobs))
    {
        job->status = "paused";
        job->updatedAt = juce::Time::getCurrentTime();
        publishAudioAnalysisProgress (*job, "submit_limit_reached");
        refreshAnalysisTimer();
        return;
    }

    if (job->nextClipIndex >= job->clips.size())
    {
        job->status = "submitted";
        job->updatedAt = juce::Time::getCurrentTime();
        job->finishedAt = job->updatedAt;
        publishAudioAnalysisProgress (*job, "submitted");
        refreshAnalysisTimer();
        return;
    }

    const auto clip = job->clips[job->nextClipIndex++];
    requestImportAcousticPackages (juce::File (clip.filePath),
                                   clip.trackId,
                                   clip.clipId,
                                   clip.durationSeconds,
                                   publishMessage);
    job->submittedClips += 1;
    job->submittedFeatureJobs += kImportAnalysisFeaturesPerClip;
    job->updatedAt = juce::Time::getCurrentTime();

    if (job->nextClipIndex >= job->clips.size())
    {
        job->status = "submitted";
        job->finishedAt = job->updatedAt;
        publishAudioAnalysisProgress (*job, "submitted");
    }
    else
    {
        publishAudioAnalysisProgress (*job, "clip_submitted");
    }

    refreshAnalysisTimer();
}

ImportService::AudioImportInsertResult ImportService::insertWaveClipWithUndoAndStartBake (
    te::Edit& edit,
    te::AudioTrack& targetTrack,
    const juce::File& sourceFile,
    double startTimeSeconds,
    double audioLengthSeconds,
    bool deleteExistingClips,
    const juce::String& appliedMode) const
{
    ImportPerfTrace perf ("insert_wave_clip",
                          targetTrack.itemID.toString(),
                          sourceFile.getFullPathName());
    AudioImportInsertResult out;
    edit.getUndoManager().beginNewTransaction (
        "Import Media: " + sourceFile.getFileName());
    perf.mark ("begin_transaction");

    const auto clipStart = te::TimePosition::fromSeconds (startTimeSeconds);
    const auto clipEnd   = te::TimePosition::fromSeconds (startTimeSeconds + audioLengthSeconds);
    auto newClip = targetTrack.insertWaveClip (sourceFile.getFileNameWithoutExtension(),
                                               sourceFile,
                                               {{ clipStart, clipEnd }},
                                               deleteExistingClips);
    perf.mark ("tracktion_insert_wave_clip",
               "delete_existing=" + juce::String (deleteExistingClips ? "true" : "false")
               + " start=" + juce::String (startTimeSeconds, 4)
               + " length=" + juce::String (audioLengthSeconds, 4));

    if (newClip == nullptr)
    {
        perf.finish ("error", "reason=insert_failed");
        out.errorMessage = "Failed to insert audio clip";
        return out;
    }

    out.clipId = newClip->itemID.toString();
    out.clipName = newClip->getName();
    out.trackItemId = targetTrack.itemID.toString();
    out.startTimeSeconds = startTimeSeconds;
    out.audioLengthSeconds = audioLengthSeconds;

    targetTrack.flushStateToValueTree();
    edit.invalidateStoredLength();
    perf.mark ("flush_track_and_invalidate_length", "clip_id=" + out.clipId);
    edit.dispatchPendingUpdatesSynchronously();
    perf.mark ("dispatch_pending_updates");
    out.editLengthSeconds = edit.getLength().inSeconds();
    edit.getTransport().ensureContextAllocated (true);
    perf.mark ("ensure_context_allocated",
               "edit_length=" + juce::String (out.editLengthSeconds, 4));

    requestImportAcousticPackages (sourceFile, out.trackItemId, out.clipId, audioLengthSeconds, publishMessage);
    perf.mark ("request_import_waveform_envelope");

    if (saveProject && ! saveProject())
    {
        perf.finish ("error", "reason=save_project_failed");
        out.errorMessage = "Audio clip added in memory but failed to save project";
        return out;
    }
    perf.mark ("save_project", saveProject ? "called=true" : "called=false");

    out.ok = true;
    perf.finish ("ok", "clip_id=" + out.clipId);
    juce::ignoreUnused (appliedMode);
    return out;
}

juce::String ImportService::handleAddAudioClip (const juce::DynamicObject& object, const juce::String&) const
{
    auto* edit = getEdit != nullptr ? getEdit() : nullptr;

    if (edit == nullptr)
        return makeErrorReply ("No active edit loaded");

    const auto trackID = object.getProperty ("track_id").toString().trim();
    const auto filePath = object.getProperty ("file_path").toString().trim();
    const auto startTimeVar = object.getProperty ("start_time");

    if (trackID.isEmpty())
        return makeErrorReply ("add_audio_clip requires a non-empty track_id");

    if (filePath.isEmpty())
        return makeErrorReply ("add_audio_clip requires a non-empty file_path");

    if (! startTimeVar.isDouble() && ! startTimeVar.isInt() && ! startTimeVar.isInt64())
        return makeErrorReply ("add_audio_clip requires a numeric start_time field");

    ImportPerfTrace perf ("add_audio_clip", trackID, filePath);
    const auto startTimeSeconds = static_cast<double> (startTimeVar);

    if (startTimeSeconds < 0.0)
    {
        perf.finish ("error", "reason=negative_start_time");
        return makeErrorReply ("start_time must be greater than or equal to zero");
    }

    auto sourceFile = juce::File (filePath);

    if (! sourceFile.existsAsFile())
    {
        perf.finish ("error", "reason=file_missing");
        return makeErrorReply ("Audio file does not exist: " + filePath);
    }
    perf.mark ("file_exists");

    auto audioFile = te::AudioFile (edit->engine, sourceFile);

    if (! audioFile.isValid())
    {
        perf.finish ("error", "reason=audio_file_invalid");
        return makeErrorReply ("Unsupported or unreadable audio file: " + filePath);
    }
    perf.mark ("audio_file_open");

    const auto audioLengthSeconds = audioFile.getLength();
    perf.mark ("audio_length", "length=" + juce::String (audioLengthSeconds, 4));

    if (audioLengthSeconds <= 0.0)
    {
        perf.finish ("error", "reason=zero_length");
        return makeErrorReply ("Audio file has zero length: " + filePath);
    }

    auto* targetTrack = findAudioTrackByID (*edit, trackID);
    perf.mark ("find_track", "found=" + juce::String (targetTrack != nullptr ? "true" : "false"));

    if (targetTrack == nullptr)
    {
        perf.finish ("error", "reason=track_not_found");
        return makeErrorReply ("Audio track not found for track_id: " + trackID);
    }

    const bool monitoringChanged = ensureMonitoringPlugins (*targetTrack);
    perf.mark ("ensure_monitoring_plugins", "changed=" + juce::String (monitoringChanged ? "true" : "false"));

    const auto clipStart = te::TimePosition::fromSeconds (startTimeSeconds);
    const auto clipEnd = te::TimePosition::fromSeconds (startTimeSeconds + audioLengthSeconds);
    auto& undo = edit->getUndoManager();
    undo.beginNewTransaction ("Add audio clip");
    auto newClip = targetTrack->insertWaveClip (sourceFile.getFileNameWithoutExtension(),
                                                sourceFile,
                                                {{ clipStart, clipEnd }},
                                                false);

    if (newClip == nullptr)
        return makeErrorReply ("Failed to insert audio clip");

    edit->invalidateStoredLength();
    edit->dispatchPendingUpdatesSynchronously();
    perf.mark ("dispatch_pending_updates", "clip_id=" + newClip->itemID.toString());
    const auto editLengthSeconds = edit->getLength().inSeconds();
    edit->getTransport().ensureContextAllocated (true);
    perf.mark ("ensure_context_allocated", "edit_length=" + juce::String (editLengthSeconds, 4));

    requestImportAcousticPackages (sourceFile, trackID, newClip->itemID.toString(), audioLengthSeconds, publishMessage);
    perf.mark ("request_import_waveform_envelope");

    if (saveProject && ! saveProject())
    {
        perf.finish ("error", "reason=save_project_failed");
        return makeErrorReply ("Audio clip added in memory but failed to save project");
    }
    perf.mark ("save_project", saveProject ? "called=true" : "called=false");
    perf.finish ("ok", "clip_id=" + newClip->itemID.toString());

    auto response = std::make_unique<juce::DynamicObject>();
    response->setProperty ("status", "ok");
    response->setProperty ("message", "Audio clip added");
    response->setProperty ("track_id", trackID);
    response->setProperty ("clip_name", newClip->getName());
    response->setProperty ("file_path", sourceFile.getFullPathName());
    response->setProperty ("start_time", startTimeSeconds);
    response->setProperty ("clip_length_seconds", audioLengthSeconds);
    response->setProperty ("edit_length_seconds", editLengthSeconds);
    return juce::JSON::toString (juce::var (response.release()));
}

juce::String ImportService::handleImportMediaToTrack (const juce::DynamicObject& object, const juce::String&) const
{
    auto* edit = getEdit != nullptr ? getEdit() : nullptr;

    if (edit == nullptr)
        return makeErrorReply ("No active edit loaded");

    const auto filePath = object.getProperty ("file_path").toString().trim();
    const auto trackId = object.getProperty ("track_id").toString().trim();
    const auto mediaType = object.getProperty ("media_type").toString().trim().toLowerCase();
    const auto mode = object.getProperty ("mode").toString().trim().toLowerCase();
    const auto startTimeVar = object.getProperty ("start_time");

    if (filePath.isEmpty())
        return makeErrorReply ("import_media_to_track requires a non-empty file_path");

    if (trackId.isEmpty())
        return makeErrorReply ("import_media_to_track requires a non-empty track_id");

    if (mediaType.isEmpty() || mediaType != juce::String ("audio"))
        return makeErrorReply ("import_media_to_track: unsupported or missing media_type (expected \"audio\")");

    if (! startTimeVar.isDouble() && ! startTimeVar.isInt() && ! startTimeVar.isInt64())
        return makeErrorReply ("import_media_to_track requires a numeric start_time field");

    ImportPerfTrace perf ("import_media_to_track", trackId, filePath);
    const double startTimeSeconds = juce::jmax (0.0, static_cast<double> (startTimeVar));

    auto sourceFile = juce::File (filePath);

    if (! sourceFile.existsAsFile())
    {
        perf.finish ("error", "reason=file_missing");
        return makeErrorReply ("Audio file does not exist: " + filePath);
    }
    perf.mark ("file_exists");

    auto audioFile = te::AudioFile (edit->engine, sourceFile);

    if (! audioFile.isValid())
    {
        perf.finish ("error", "reason=audio_file_invalid");
        return makeErrorReply ("Unsupported or unreadable audio file: " + filePath);
    }
    perf.mark ("audio_file_open");

    const auto audioLengthSeconds = audioFile.getLength();
    perf.mark ("audio_length", "length=" + juce::String (audioLengthSeconds, 4));

    if (audioLengthSeconds <= 0.0)
    {
        perf.finish ("error", "reason=zero_length");
        return makeErrorReply ("Audio file has zero length: " + filePath);
    }

    auto* targetTrack = findAudioTrackByID (*edit, trackId);
    perf.mark ("find_track", "found=" + juce::String (targetTrack != nullptr ? "true" : "false"));

    if (targetTrack == nullptr)
    {
        perf.finish ("error", "reason=track_not_found");
        return makeErrorReply ("Audio track not found for track_id: " + trackId);
    }

    const bool monitoringChanged = ensureMonitoringPlugins (*targetTrack);
    perf.mark ("ensure_monitoring_plugins", "changed=" + juce::String (monitoringChanged ? "true" : "false"));
    const bool deleteExistingClips = (mode.isEmpty() || mode == juce::String ("destructive"));
    const auto appliedMode = deleteExistingClips ? juce::String ("destructive")
                                                  : juce::String ("non_destructive");

    const auto inserted = insertWaveClipWithUndoAndStartBake (*edit,
                                                              *targetTrack,
                                                              sourceFile,
                                                              startTimeSeconds,
                                                              audioLengthSeconds,
                                                               deleteExistingClips,
                                                               appliedMode);
    perf.mark ("insert_wave_clip_with_bake",
               "ok=" + juce::String (inserted.ok ? "true" : "false")
               + " clip_id=" + inserted.clipId);

    if (! inserted.ok)
    {
        perf.finish ("error", "reason=insert_failed");
        return makeErrorReply (inserted.errorMessage);
    }

    juce::Logger::writeToLog ("ImportService::handleImportMediaToTrack: clipId=\""
                            + inserted.clipId
                            + "\" clip=\""
                            + inserted.clipName
                            + "\" trackId=\""
                            + inserted.trackItemId
                            + "\" source=\""
                            + sourceFile.getFullPathName()
                            + "\" lengthSec="
                            + juce::String (inserted.audioLengthSeconds, 3)
                            + " "
                            + describeTransportState (*edit));

    auto response = std::make_unique<juce::DynamicObject>();
    response->setProperty ("status", "success");
    response->setProperty ("action", "import_media_to_track");
    response->setProperty ("clip_id", inserted.clipId);
    response->setProperty ("track_id", inserted.trackItemId);
    response->setProperty ("start_time", inserted.startTimeSeconds);
    response->setProperty ("resolved_start_time", inserted.startTimeSeconds);
    response->setProperty ("length", inserted.audioLengthSeconds);
    response->setProperty ("applied_mode", appliedMode);
    response->setProperty ("clip_name", inserted.clipName);
    response->setProperty ("file_path", sourceFile.getFullPathName());
    response->setProperty ("edit_length_seconds", inserted.editLengthSeconds);
    response->setProperty ("baking_status", "baking_started");
    perf.finish ("ok", "clip_id=" + inserted.clipId);
    return juce::JSON::toString (juce::var (response.release()));
}

juce::String ImportService::handleImportAudio (const juce::DynamicObject& object, const juce::String&) const
{
    auto* edit = getEdit != nullptr ? getEdit() : nullptr;

    if (edit == nullptr)
        return makeErrorReply ("No active edit loaded");

    const auto filePath = object.getProperty ("file_path").toString().trim();
    const auto trackId = object.getProperty ("track_id").toString().trim();

    if (filePath.isEmpty())
        return makeErrorReply ("import_audio requires a non-empty file_path");

    if (trackId.isEmpty())
        return makeErrorReply ("import_audio requires a non-empty track_id");

    ImportPerfTrace perf ("import_audio", trackId, filePath);
    auto sourceFile = juce::File (filePath);

    if (! sourceFile.existsAsFile())
    {
        perf.finish ("error", "reason=file_missing");
        return makeErrorReply ("Audio file does not exist: " + filePath);
    }
    perf.mark ("file_exists");

    auto audioFile = te::AudioFile (edit->engine, sourceFile);

    if (! audioFile.isValid())
    {
        perf.finish ("error", "reason=audio_file_invalid");
        return makeErrorReply ("Unsupported or unreadable audio file: " + filePath);
    }
    perf.mark ("audio_file_open");

    const auto audioLengthSeconds = audioFile.getLength();
    perf.mark ("audio_length", "length=" + juce::String (audioLengthSeconds, 4));

    if (audioLengthSeconds <= 0.0)
    {
        perf.finish ("error", "reason=zero_length");
        return makeErrorReply ("Audio file has zero length: " + filePath);
    }

    auto* targetTrack = findAudioTrackByID (*edit, trackId);
    perf.mark ("find_track", "found=" + juce::String (targetTrack != nullptr ? "true" : "false"));

    if (targetTrack == nullptr)
    {
        perf.finish ("error", "reason=track_not_found");
        return makeErrorReply ("Audio track not found for track_id: " + trackId);
    }

    const bool monitoringChanged = ensureMonitoringPlugins (*targetTrack);
    perf.mark ("ensure_monitoring_plugins", "changed=" + juce::String (monitoringChanged ? "true" : "false"));

    const auto offsetTimeVar = object.getProperty ("offset_time");
    double startTimeSeconds = 0.0;

    if (offsetTimeVar.isDouble() || offsetTimeVar.isInt() || offsetTimeVar.isInt64())
    {
        startTimeSeconds = juce::jmax (0.0, static_cast<double> (offsetTimeVar));
    }
    else
    {
        if (auto* clipTrack = dynamic_cast<te::ClipTrack*> (targetTrack))
        {
            for (auto* clip : clipTrack->getClips())
            {
                if (clip == nullptr)
                    continue;

                const auto clipEnd = clip->getEditTimeRange().getEnd().inSeconds();

                if (clipEnd > startTimeSeconds)
                    startTimeSeconds = clipEnd;
            }
        }
    }

    const auto inserted = insertWaveClipWithUndoAndStartBake (*edit,
                                                              *targetTrack,
                                                              sourceFile,
                                                              startTimeSeconds,
                                                               audioLengthSeconds,
                                                               false,
                                                               "append");
    perf.mark ("insert_wave_clip_with_bake",
               "ok=" + juce::String (inserted.ok ? "true" : "false")
               + " clip_id=" + inserted.clipId);

    if (! inserted.ok)
    {
        perf.finish ("error", "reason=insert_failed");
        return makeErrorReply (inserted.errorMessage);
    }

    juce::Logger::writeToLog ("ImportService::handleImportAudio: imported clip=\""
                              + inserted.clipName
                              + "\" clipId=\""
                              + inserted.clipId
                              + "\" trackId=\""
                              + inserted.trackItemId
                              + "\" source=\""
                              + sourceFile.getFullPathName()
                              + "\" clipLengthSeconds="
                              + juce::String (inserted.audioLengthSeconds, 3)
                              + " "
                              + describeTransportState (*edit));

    auto response = std::make_unique<juce::DynamicObject>();
    response->setProperty ("status", "ok");
    response->setProperty ("action", "import_audio");
    response->setProperty ("message", "Audio clip imported");
    response->setProperty ("track_id", inserted.trackItemId);
    response->setProperty ("clip_id", inserted.clipId);
    response->setProperty ("clip_name", inserted.clipName);
    response->setProperty ("file_path", sourceFile.getFullPathName());
    response->setProperty ("start_time", inserted.startTimeSeconds);
    response->setProperty ("clip_length_seconds", inserted.audioLengthSeconds);
    response->setProperty ("length", inserted.audioLengthSeconds);
    response->setProperty ("edit_length_seconds", inserted.editLengthSeconds);
    response->setProperty ("baking_status", "baking_started");
    perf.finish ("ok", "clip_id=" + inserted.clipId);
    return juce::JSON::toString (juce::var (response.release()));
}

juce::String ImportService::handleImportFolderAsStems (const juce::DynamicObject& object, const juce::String&)
{
    auto* edit = getEdit != nullptr ? getEdit() : nullptr;

    if (edit == nullptr)
        return makeErrorReply ("No active edit loaded");

    const auto targetPolicy = getStringFromObject (object, "target_policy", "create_tracks").toLowerCase();
    if (targetPolicy != "create_tracks")
        return makeErrorReply ("project.import_folder_as_stems currently supports target_policy=create_tracks only");

    auto settings = importSettingsFromEdit (*edit);
    settings = settingsWithCommandOverrides (object, settings);

    if (settings.channelImportPolicy.equalsIgnoreCase ("split_mono"))
        return makeErrorReply ("channel_import_policy=split_mono is not implemented for project.import_folder_as_stems; use preserve_interleaved");

    const bool confirmed = boolProperty (object, "confirmed", false)
                        || boolProperty (object, "confirmation", false)
                        || boolProperty (object, "allow_policy_warnings", false);
    const bool skipUnreadable = boolProperty (object, "skip_unreadable", false);
    const double startTimeSeconds = juce::jmax (0.0,
                                                doubleProperty (object,
                                                                "start_time_seconds",
                                                                doubleProperty (object, "start_time", 0.0)));
    const bool explicitStartAudioAnalysis = boolProperty (object, "start_background_analysis", false)
                                         || boolProperty (object, "start_audio_analysis", false)
                                         || boolProperty (object, "start_baking", false)
                                         || boolProperty (object, "immediate_bake", false);
    const bool deferAudioAnalysis = ! explicitStartAudioAnalysis
                                  && boolProperty (object, "defer_audio_analysis", false);
    const bool startBackgroundAnalysis = explicitStartAudioAnalysis;
    const bool queueBackgroundAnalysis = ! startBackgroundAnalysis && ! deferAudioAnalysis;
    const auto analysisStatus = juce::String (startBackgroundAnalysis ? "baking_started"
                                           : (queueBackgroundAnalysis ? "running" : "deferred"));
    const auto analysisQueueStatus = juce::String (startBackgroundAnalysis ? "not_queued"
                                                : (queueBackgroundAnalysis ? "running" : "queued"));

    const auto files = uniqueImportFilesFromCommand (object);
    if (files.isEmpty())
        return makeErrorReply ("project.import_folder_as_stems requires folder_path, file_path, or file_paths");

    juce::Array<ImportCandidate> candidates;
    juce::Array<juce::var> fileRows;
    juce::Array<juce::var> unreadableFiles;
    juce::Array<juce::var> sampleRateMismatches;
    juce::Array<juce::var> bitDepthMismatches;
    juce::Array<juce::var> warnings;

    for (auto file : files)
    {
        ImportCandidate candidate;
        const auto inspected = inspectImportCandidate (*edit, file, settings, candidate);

        if (inspected.failed())
        {
            auto unreadable = std::make_unique<juce::DynamicObject>();
            unreadable->setProperty ("file_path", file.getFullPathName());
            unreadable->setProperty ("error", inspected.getErrorMessage());
            unreadableFiles.add (juce::var (unreadable.release()));

            auto row = std::make_unique<juce::DynamicObject>();
            row->setProperty ("file_path", file.getFullPathName());
            row->setProperty ("file_name", file.getFileName());
            row->setProperty ("readable", false);
            row->setProperty ("error", inspected.getErrorMessage());
            fileRows.add (juce::var (row.release()));
            continue;
        }

        if (candidate.needsSampleRateConversion)
        {
            auto mismatch = std::make_unique<juce::DynamicObject>();
            mismatch->setProperty ("file_path", file.getFullPathName());
            mismatch->setProperty ("source_sample_rate_hz", candidate.sampleRateHz);
            mismatch->setProperty ("project_sample_rate_hz", settings.sampleRateHz);
            mismatch->setProperty ("policy", settings.importSampleRatePolicy);
            sampleRateMismatches.add (juce::var (mismatch.release()));
        }

        if (candidate.bitDepthOrFormatDiffers)
        {
            auto mismatch = std::make_unique<juce::DynamicObject>();
            mismatch->setProperty ("file_path", file.getFullPathName());
            mismatch->setProperty ("source_bit_depth", candidate.bitDepth > 0 ? juce::var (candidate.bitDepth) : juce::var());
            mismatch->setProperty ("source_pcm_format", candidate.pcmFormat);
            mismatch->setProperty ("project_record_bit_depth", settings.recordBitDepth);
            mismatch->setProperty ("policy", settings.importBitDepthPolicy);
            bitDepthMismatches.add (juce::var (mismatch.release()));
        }

        fileRows.add (candidateToFileRow (candidate));
        candidates.add (candidate);
    }

    if (! unreadableFiles.isEmpty() && ! skipUnreadable)
        return makeErrorReply ("project.import_folder_as_stems found unreadable audio files; rerun preflight or pass skip_unreadable=true to import only readable files");

    if (candidates.isEmpty())
        return makeErrorReply ("project.import_folder_as_stems found no readable audio files");

    if (settings.importSampleRatePolicy.equalsIgnoreCase ("reject_mismatch") && ! sampleRateMismatches.isEmpty())
        return makeErrorReply ("project.import_folder_as_stems rejected sample-rate mismatches by policy");

    if (settings.importSampleRatePolicy.equalsIgnoreCase ("ask") && ! sampleRateMismatches.isEmpty() && ! confirmed)
        return makeErrorReply ("project.import_folder_as_stems requires confirmation for sample-rate mismatches");

    if (settings.importBitDepthPolicy.equalsIgnoreCase ("ask") && ! bitDepthMismatches.isEmpty() && ! confirmed)
        return makeErrorReply ("project.import_folder_as_stems requires confirmation for bit-depth or PCM-format differences");

    if (settings.mediaCopyPolicy.equalsIgnoreCase ("ask") && ! confirmed)
        return makeErrorReply ("project.import_folder_as_stems requires confirmation for media_copy_policy=ask");

    if (settings.importSampleRatePolicy.equalsIgnoreCase ("convert_to_project") && ! sampleRateMismatches.isEmpty())
    {
        auto warning = std::make_unique<juce::DynamicObject>();
        warning->setProperty ("code", "sample_rate_conversion_deferred_to_engine");
        warning->setProperty ("message", "Imported clips reference source media; playback/render uses the engine's resampling path rather than rewriting source files during import.");
        warnings.add (juce::var (warning.release()));
    }

    if (settings.importBitDepthPolicy.equalsIgnoreCase ("convert_to_project_format") && ! bitDepthMismatches.isEmpty())
    {
        auto warning = std::make_unique<juce::DynamicObject>();
        warning->setProperty ("code", "bit_depth_conversion_not_rewritten_on_import");
        warning->setProperty ("message", "Imported clips reference source media; import does not rewrite source bit depth or PCM format.");
        warnings.add (juce::var (warning.release()));
    }

    juce::Array<juce::var> copiedMediaRows;
    if (const auto copyResult = applyMediaCopyPolicy (object, settings, getCurrentProjectPath, candidates, copiedMediaRows);
        copyResult.failed())
    {
        return makeErrorReply (copyResult.getErrorMessage());
    }

    juce::Array<juce::var> importedRows;
    juce::Array<juce::var> createdTrackIds;
    juce::Array<juce::var> createdClipIds;
    juce::Array<QueuedAnalysisClip> deferredAnalysisClips;
    juce::String lastTrackId;
    juce::String lastClipId;
    int analysisJobsCreated = 0;

    auto rollbackImport = [&]
    {
        edit->getUndoManager().undo();
        edit->invalidateStoredLength();
        edit->dispatchPendingUpdatesSynchronously();
        edit->getTransport().ensureContextAllocated (true);
    };

    auto& undo = edit->getUndoManager();
    undo.beginNewTransaction ("Import folder as stems");

    for (auto& candidate : candidates)
    {
        auto newTrack = edit->insertNewAudioTrack (te::TrackInsertPoint::getEndOfTracks (*edit), nullptr, true);

        if (newTrack == nullptr)
        {
            rollbackImport();
            return makeErrorReply ("Failed to create audio track for imported stem: " + candidate.sourceFile.getFileName());
        }

        ensureMonitoringPlugins (*newTrack);
        ensureSingleRackForTrack (*newTrack);
        newTrack->setName (candidate.trackName);
        newTrack->state.setProperty ("vit_import_role", "stem", &undo);
        newTrack->state.setProperty ("vit_import_source_file", candidate.sourceFile.getFullPathName(), &undo);
        newTrack->state.setProperty ("vit_import_media_copy_policy", settings.mediaCopyPolicy, &undo);

        const auto clipStart = te::TimePosition::fromSeconds (startTimeSeconds);
        const auto clipEnd = te::TimePosition::fromSeconds (startTimeSeconds + candidate.durationSeconds);
        auto newClip = newTrack->insertWaveClip (candidate.sourceFile.getFileNameWithoutExtension(),
                                                 candidate.insertFile,
                                                 {{ clipStart, clipEnd }},
                                                 false);

        if (newClip == nullptr)
        {
            rollbackImport();
            return makeErrorReply ("Failed to insert audio clip for imported stem: " + candidate.sourceFile.getFileName());
        }

        newClip->state.setProperty ("vit_import_source_file", candidate.sourceFile.getFullPathName(), &undo);
        newClip->state.setProperty ("vit_import_insert_file", candidate.insertFile.getFullPathName(), &undo);
        newClip->state.setProperty ("vit_import_media_copy_policy", settings.mediaCopyPolicy, &undo);

        const auto trackId = newTrack->itemID.toString();
        const auto clipId = newClip->itemID.toString();
        lastTrackId = trackId;
        lastClipId = clipId;
        createdTrackIds.add (trackId);
        createdClipIds.add (clipId);

        newTrack->flushStateToValueTree();

        if (startBackgroundAnalysis)
        {
            requestImportAcousticPackages (candidate.insertFile, trackId, clipId, candidate.durationSeconds, publishMessage);
            analysisJobsCreated += kImportAnalysisFeaturesPerClip;
        }
        else
        {
            QueuedAnalysisClip analysisClip;
            analysisClip.filePath = candidate.insertFile.getFullPathName();
            analysisClip.trackId = trackId;
            analysisClip.clipId = clipId;
            analysisClip.durationSeconds = candidate.durationSeconds;
            deferredAnalysisClips.add (analysisClip);
        }

        auto row = std::make_unique<juce::DynamicObject>();
        row->setProperty ("track_id", trackId);
        row->setProperty ("track_name", newTrack->getName());
        row->setProperty ("clip_id", clipId);
        row->setProperty ("clip_name", newClip->getName());
        row->setProperty ("source_file_path", candidate.sourceFile.getFullPathName());
        row->setProperty ("imported_file_path", candidate.insertFile.getFullPathName());
        row->setProperty ("start_time_seconds", startTimeSeconds);
        row->setProperty ("duration_seconds", candidate.durationSeconds);
        row->setProperty ("sample_rate_hz", candidate.sampleRateHz);
        row->setProperty ("bit_depth", candidate.bitDepth > 0 ? juce::var (candidate.bitDepth) : juce::var());
        row->setProperty ("pcm_format", candidate.pcmFormat);
        row->setProperty ("channel_count", candidate.channelCount);
        row->setProperty ("needs_sample_rate_conversion", candidate.needsSampleRateConversion);
        row->setProperty ("bit_depth_or_format_differs", candidate.bitDepthOrFormatDiffers);
        row->setProperty ("sample_rate_policy", settings.importSampleRatePolicy);
        row->setProperty ("bit_depth_policy", settings.importBitDepthPolicy);
        row->setProperty ("media_copy_policy", settings.mediaCopyPolicy);
        row->setProperty ("channel_import_policy", settings.channelImportPolicy);
        row->setProperty ("analysis_deferred", deferAudioAnalysis);
        row->setProperty ("analysis_jobs_created", startBackgroundAnalysis ? kImportAnalysisFeaturesPerClip : 0);
        row->setProperty ("background_analysis_status", analysisStatus);
        row->setProperty ("analysis_queue_status", analysisQueueStatus);
        row->setProperty ("baking_status", analysisStatus);
        importedRows.add (juce::var (row.release()));
    }

    edit->invalidateStoredLength();
    edit->dispatchPendingUpdatesSynchronously();
    const auto editLengthSeconds = edit->getLength().inSeconds();
    edit->getTransport().ensureContextAllocated (true);

    if (saveProject && ! saveProject())
        return makeErrorReply ("Stems imported in memory but failed to save project");

    const auto analysisJobId = startBackgroundAnalysis ? juce::String()
                                                       : registerDeferredAudioAnalysisJob (deferredAnalysisClips,
                                                                                           queueBackgroundAnalysis);
    const auto queuedFeatureJobs = startBackgroundAnalysis
        ? 0
        : deferredAnalysisClips.size() * kImportAnalysisFeaturesPerClip;

    auto summary = std::make_unique<juce::DynamicObject>();
    summary->setProperty ("discovered_audio_file_count", files.size());
    summary->setProperty ("readable_file_count", candidates.size());
    summary->setProperty ("unreadable_file_count", unreadableFiles.size());
    summary->setProperty ("tracks_created", createdTrackIds.size());
    summary->setProperty ("clips_created", createdClipIds.size());
    summary->setProperty ("sample_rate_mismatch_count", sampleRateMismatches.size());
    summary->setProperty ("bit_depth_or_format_mismatch_count", bitDepthMismatches.size());
    summary->setProperty ("requires_user_confirmation", false);
    summary->setProperty ("copy_policy", settings.mediaCopyPolicy);
    summary->setProperty ("start_time_seconds", startTimeSeconds);
    summary->setProperty ("edit_length_seconds", editLengthSeconds);
    summary->setProperty ("analysis_deferred", deferAudioAnalysis);
    summary->setProperty ("analysis_jobs_created", analysisJobsCreated);
    summary->setProperty ("analysis_jobs_queued", queuedFeatureJobs);
    summary->setProperty ("analysis_total_clips", deferredAnalysisClips.size());
    summary->setProperty ("analysis_total_feature_jobs", queuedFeatureJobs);
    summary->setProperty ("analysis_job_id", analysisJobId);
    summary->setProperty ("analysis_queue_status", analysisQueueStatus);
    summary->setProperty ("background_analysis_status", analysisStatus);

    auto response = std::make_unique<juce::DynamicObject>();
    response->setProperty ("status", "ok");
    response->setProperty ("command", getStringFromObject (object, "cmd", "project.import_folder_as_stems"));
    response->setProperty ("action", "import_folder_as_stems");
    response->setProperty ("message", startBackgroundAnalysis ? "Stems imported as separate tracks"
                                  : (queueBackgroundAnalysis ? "Stems imported as separate tracks; audio analysis queued"
                                                             : "Stems imported as separate tracks; audio analysis deferred"));
    response->setProperty ("summary", juce::var (summary.release()));
    response->setProperty ("audio_settings_snapshot", buildImportSettingsSnapshot (settings));
    response->setProperty ("files", juce::var (fileRows));
    response->setProperty ("imported_tracks", juce::var (importedRows));
    response->setProperty ("created_track_ids", juce::var (createdTrackIds));
    response->setProperty ("created_clip_ids", juce::var (createdClipIds));
    response->setProperty ("last_created_track_id", lastTrackId);
    response->setProperty ("last_created_clip_id", lastClipId);
    response->setProperty ("sample_rate_mismatches", juce::var (sampleRateMismatches));
    response->setProperty ("bit_depth_or_format_mismatches", juce::var (bitDepthMismatches));
    response->setProperty ("unreadable_files", juce::var (unreadableFiles));
    response->setProperty ("copied_media", juce::var (copiedMediaRows));
    response->setProperty ("warnings", juce::var (warnings));
    response->setProperty ("analysis_deferred", deferAudioAnalysis);
    response->setProperty ("analysis_jobs_created", analysisJobsCreated);
    response->setProperty ("analysis_jobs_queued", queuedFeatureJobs);
    response->setProperty ("analysis_total_clips", deferredAnalysisClips.size());
    response->setProperty ("analysis_total_feature_jobs", queuedFeatureJobs);
    response->setProperty ("analysis_job_id", analysisJobId);
    response->setProperty ("analysis_queue_status", analysisQueueStatus);
    if (auto* job = findAnalysisJob (analysisJobId))
        response->setProperty ("analysis_job", buildAudioAnalysisJobStatus (*job));
    response->setProperty ("background_analysis_status", analysisStatus);
    response->setProperty ("baking_status", analysisStatus);
    return juce::JSON::toString (juce::var (response.release()));
}

juce::String ImportService::handleAudioAnalysisStart (const juce::DynamicObject& object, const juce::String&)
{
    auto* job = findAnalysisJob (getStringFromObject (object, "analysis_job_id",
                                                      getStringFromObject (object, "job_id")));
    if (job == nullptr)
        job = findLatestAnalysisJob();

    const bool rebuildFromProject = boolProperty (object, "rebuild_from_project", false)
                                 || boolProperty (object, "create_from_project", false);

    if (job == nullptr && rebuildFromProject)
    {
        const auto clips = collectCurrentProjectAnalysisClips();
        const auto rebuiltJobId = registerDeferredAudioAnalysisJob (clips, false);
        job = findAnalysisJob (rebuiltJobId);
    }

    if (job == nullptr)
        return makeErrorReply (rebuildFromProject
                                   ? "project.audio_analysis_start could not find analyzable source clips in the current project"
                                   : "project.audio_analysis_start could not find a queued analysis job");

    const bool retryMissing = boolProperty (object, "retry_missing", false)
                           || boolProperty (object, "rebuild_missing", false);

    if (job->status == "submitted" && retryMissing)
    {
        int missingCount = 0;
        for (const auto& clip : job->clips)
        {
            const auto latest = WaveformEnvelopeBaker::getLatestBakeStatus (clip.trackId, clip.clipId);
            const auto* latestObject = latest.getDynamicObject();
            if (latestObject == nullptr || latestObject->getProperty ("status").toString() != "ready")
                ++missingCount;
        }

        if (missingCount == 0)
            return makeAudioAnalysisJobReply (*job,
                                              "project.audio_analysis_start",
                                              "Audio analysis job is already ready for every source clip");

        // Clip graph edits can invalidate clip-keyed bake rows after all source
        // requests were submitted. Replay the compact source queue so the job
        // can recover without re-importing or changing the project.
        job->nextClipIndex = 0;
        job->submittedClips = 0;
        job->submittedFeatureJobs = 0;
        job->canceledPendingClips = 0;
        job->startedAt = {};
        job->finishedAt = {};
        job->status = "queued";
    }

    if (job->status == "cancelled")
        return makeErrorReply ("Audio analysis job is cancelled: " + job->jobId);

    if (job->status == "submitted")
        return makeAudioAnalysisJobReply (*job,
                                          "project.audio_analysis_start",
                                          "Audio analysis job has already submitted all pending clips");

    job->intervalMs = juce::jlimit (50,
                                    10000,
                                    getIntFromObject (object,
                                                      "interval_ms",
                                                      getIntFromObject (object, "submit_interval_ms", job->intervalMs)));
    job->maxSubmitClips = juce::jmax (0,
                                      getIntFromObject (object,
                                                        "max_submit_clips",
                                                        getIntFromObject (object, "max_clips", job->maxSubmitClips)));
    job->maxSubmitFeatureJobs = juce::jmax (0,
                                            getIntFromObject (object,
                                                              "max_submit_feature_jobs",
                                                              getIntFromObject (object, "max_feature_jobs", job->maxSubmitFeatureJobs)));

    job->status = "running";
    const auto now = juce::Time::getCurrentTime();
    if (job->startedAt.toMilliseconds() <= 0)
        job->startedAt = now;
    job->updatedAt = now;
    publishAudioAnalysisProgress (*job, "started");
    refreshAnalysisTimer();

    return makeAudioAnalysisJobReply (*job,
                                      "project.audio_analysis_start",
                                      "Audio analysis job started");
}

juce::String ImportService::handleAudioAnalysisStatus (const juce::DynamicObject& object, const juce::String&) const
{
    auto* job = findAnalysisJob (getStringFromObject (object, "analysis_job_id",
                                                      getStringFromObject (object, "job_id")));
    if (job == nullptr && boolProperty (object, "latest", true))
        job = findLatestAnalysisJob();

    if (job == nullptr)
        return makeErrorReply ("project.audio_analysis_status could not find an analysis job");

    return makeAudioAnalysisJobReply (*job,
                                      "project.audio_analysis_status",
                                      "Audio analysis job status");
}

juce::String ImportService::handleAudioAnalysisCancel (const juce::DynamicObject& object, const juce::String&)
{
    auto* job = findAnalysisJob (getStringFromObject (object, "analysis_job_id",
                                                      getStringFromObject (object, "job_id")));
    if (job == nullptr)
        job = findLatestAnalysisJob();

    if (job == nullptr)
        return makeErrorReply ("project.audio_analysis_cancel could not find an analysis job");

    job->canceledPendingClips = juce::jmax (0, job->clips.size() - job->nextClipIndex);
    job->nextClipIndex = job->clips.size();
    job->status = "cancelled";
    job->updatedAt = juce::Time::getCurrentTime();
    job->finishedAt = job->updatedAt;
    publishAudioAnalysisProgress (*job, "cancelled");
    refreshAnalysisTimer();

    return makeAudioAnalysisJobReply (*job,
                                      "project.audio_analysis_cancel",
                                      "Audio analysis job cancelled; already submitted background bakes may continue");
}

juce::String ImportService::handleWarmWaveformBake (const juce::DynamicObject& object, const juce::String&) const
{
    auto* edit = getEdit != nullptr ? getEdit() : nullptr;

    if (edit == nullptr)
        return makeErrorReply ("No active edit loaded");

    const auto filePath = object.getProperty ("file_path").toString().trim();
    const auto trackId  = object.getProperty ("track_id").toString().trim();
    const auto clipId   = object.getProperty ("clip_id").toString().trim();
    const auto sourceKind = object.getProperty ("source_kind").toString().trim().toLowerCase();
    const bool useExplicitFileSource = filePath.isNotEmpty()
                                     && (trackId == "master_output"
                                         || sourceKind == "master_render"
                                         || static_cast<bool> (object.getProperty ("allow_file_source")));
    if (trackId.isEmpty())
        return makeErrorReply ("warm_waveform_bake requires a non-empty track_id");
    if (! useExplicitFileSource && findAudioTrackByID (*edit, trackId) == nullptr)
        return makeErrorReply ("Audio track not found for track_id: " + trackId);

    juce::File sourceFile;
    double sourceOffsetSeconds = 0.0;
    double bakeLengthSeconds = -1.0;

    if (useExplicitFileSource)
    {
        sourceFile = juce::File (filePath);
        if (! sourceFile.existsAsFile())
            return makeErrorReply ("Audio file does not exist: " + filePath);
    }
    else if (clipId.isNotEmpty())
    {
        auto* clip = findClipByID (*edit, clipId);
        auto* audioClip = dynamic_cast<te::AudioClipBase*> (clip);
        if (audioClip == nullptr)
            return makeErrorReply ("warm_waveform_bake clip_id not found or not audio: " + clipId);
        sourceFile = audioClip->getCurrentSourceFile();
        if (! sourceFile.existsAsFile())
            sourceFile = audioClip->getOriginalFile();
        if (! sourceFile.existsAsFile())
            return makeErrorReply ("warm_waveform_bake clip source file missing for clip_id: " + clipId);
        sourceOffsetSeconds = clip->getPosition().getOffset().inSeconds();
        bakeLengthSeconds = juce::jmax (0.0, clip->getPosition().getLength().inSeconds());
    }
    else
    {
        if (filePath.isEmpty())
            return makeErrorReply ("warm_waveform_bake requires clip_id or file_path");
        sourceFile = juce::File (filePath);
        if (! sourceFile.existsAsFile())
            return makeErrorReply ("Audio file does not exist: " + filePath);
    }

    const auto requestedFeature = audioFeatureTypeFromString (
        object.getProperty ("feature_type").toString(),
        AudioFeatureType::WaveformEnvelope);
    const auto priorityText = object.getProperty ("priority").toString().trim().toLowerCase();
    auto sourceRevision = object.getProperty ("source_revision").toString().trim();
    if (sourceRevision.isEmpty())
        sourceRevision = sourceRevisionForImport (sourceFile, bakeLengthSeconds > 0.0 ? bakeLengthSeconds : 0.0);

    AudioFeatureBakeRequest featureRequest;
    featureRequest.filePath = sourceFile.getFullPathName();
    featureRequest.trackId = trackId;
    featureRequest.clipId = clipId;
    featureRequest.sourceId = object.getProperty ("source_id").toString().trim();
    if (featureRequest.sourceId.isEmpty())
        featureRequest.sourceId = sourceFile.getFullPathName();
    featureRequest.sourceRevision = sourceRevision;
    featureRequest.clipRevision = object.getProperty ("clip_revision").toString().trim();
    if (featureRequest.clipRevision.isEmpty())
        featureRequest.clipRevision = clipRevisionForFeatureRequest (trackId,
                                                                     clipId,
                                                                     sourceRevision,
                                                                     sourceOffsetSeconds,
                                                                     bakeLengthSeconds);
    featureRequest.renderRevision = object.getProperty ("render_revision").toString().trim();
    featureRequest.requestId = object.getProperty ("request_id").toString().trim();
    if (featureRequest.requestId.isEmpty())
        featureRequest.requestId = object.getProperty ("mixboard_request_id").toString().trim();
    featureRequest.featureType = requestedFeature;
    featureRequest.priority = priorityText == "background_warm"
        ? AudioFeaturePriority::BackgroundWarm
        : AudioFeaturePriority::OnDemand;
    featureRequest.range.sourceOffsetSeconds = sourceOffsetSeconds;
    featureRequest.range.lengthSeconds = bakeLengthSeconds;
    AudioFeatureService::requestBake (std::move (featureRequest), publishMessage);

    auto response = std::make_unique<juce::DynamicObject>();
    response->setProperty ("status", "ok");
    const auto command = object.getProperty ("cmd").toString().trim();
    response->setProperty ("cmd", command.isNotEmpty() ? command : "warm_waveform_bake");
    response->setProperty ("track_id", trackId);
    response->setProperty ("clip_id", clipId);
    response->setProperty ("file_path", sourceFile.getFullPathName());
    response->setProperty ("feature_family", "audio_feature");
    response->setProperty ("feature_type", audioFeatureTypeToString (requestedFeature));
    response->setProperty ("feature_version", audioFeatureProductVersion (requestedFeature));
    response->setProperty ("analysis_version", audioFeatureAnalysisVersion());
    response->setProperty ("priority", priorityText == "background_warm" ? "background_warm" : "on_demand");
    response->setProperty ("message", "Audio feature bake requested (no new clip inserted)");
    return juce::JSON::toString (juce::var (response.release()));
}

juce::String ImportService::makeStatusReply (const juce::String& status, const juce::String& message)
{
    auto response = std::make_unique<juce::DynamicObject>();
    response->setProperty ("status", status);
    response->setProperty ("message", message);
    return juce::JSON::toString (juce::var (response.release()));
}

juce::String ImportService::makeErrorReply (const juce::String& message)
{
    return makeStatusReply ("error", message);
}

} // namespace vit
