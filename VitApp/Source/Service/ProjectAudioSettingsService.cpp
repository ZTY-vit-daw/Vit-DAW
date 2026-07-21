#include "ProjectAudioSettingsService.h"

#include <algorithm>
#include <cmath>
#include <cstdlib>
#include <map>
#include <set>

namespace vit
{

namespace
{

const juce::Identifier kSettingsTree ("VIT_AUDIO_SETTINGS");
const juce::Identifier kSchemaVersion ("schema_version");
const juce::Identifier kSampleRateHz ("sample_rate_hz");
const juce::Identifier kRecordFileType ("record_file_type");
const juce::Identifier kRecordBitDepth ("record_bit_depth");
const juce::Identifier kPcmFormat ("pcm_format");
const juce::Identifier kInternalProcessingFormat ("internal_processing_format");
const juce::Identifier kImportSampleRatePolicy ("import_sample_rate_policy");
const juce::Identifier kImportBitDepthPolicy ("import_bit_depth_policy");
const juce::Identifier kMediaCopyPolicy ("media_copy_policy");
const juce::Identifier kChannelImportPolicy ("channel_import_policy");
const juce::Identifier kRenderDefaultSampleRateHz ("render_default_sample_rate_hz");
const juce::Identifier kRenderDefaultBitDepth ("render_default_bit_depth");
const juce::Identifier kRenderDefaultFileType ("render_default_file_type");
const juce::Identifier kDitherPolicy ("dither_policy");
const juce::Identifier kMigrationState ("migration_state");
const juce::Identifier kDefaultedOrigin ("defaulted_origin");

constexpr int kDefaultSampleRateHz = 48000;
constexpr int kDefaultRecordBitDepth = 24;

struct AudioSettingsDefaults
{
    const char* schemaVersion = "vit_project_audio_settings.v1";
    int sampleRateHz = kDefaultSampleRateHz;
    const char* recordFileType = "WAV/BWF";
    int recordBitDepth = kDefaultRecordBitDepth;
    const char* pcmFormat = "int24";
    const char* internalProcessingFormat = "unknown";
    const char* importSampleRatePolicy = "ask";
    const char* importBitDepthPolicy = "keep_source";
    const char* mediaCopyPolicy = "reference_original";
    const char* channelImportPolicy = "preserve_interleaved";
    int renderDefaultSampleRateHz = kDefaultSampleRateHz;
    int renderDefaultBitDepth = kDefaultRecordBitDepth;
    const char* renderDefaultFileType = "WAV";
    const char* ditherPolicy = "unknown";
};

const AudioSettingsDefaults& defaults()
{
    static AudioSettingsDefaults d;
    return d;
}

juce::UndoManager* undoFor (te::Edit& edit, bool useUndo)
{
    return useUndo ? &edit.getUndoManager() : nullptr;
}

bool hasUsableProperty (const juce::ValueTree& tree, const juce::Identifier& id)
{
    if (! tree.hasProperty (id))
        return false;

    const auto value = tree.getProperty (id);
    return ! value.isVoid() && value.toString().trim().isNotEmpty();
}

bool setIfMissing (juce::ValueTree& tree,
                   const juce::Identifier& id,
                   const juce::var& value,
                   juce::UndoManager* undo)
{
    if (hasUsableProperty (tree, id))
        return false;

    tree.setProperty (id, value, undo);
    return true;
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

int intProperty (const juce::ValueTree& tree, const juce::Identifier& id, int fallback)
{
    double parsed = 0.0;
    return numericVarToDouble (tree.getProperty (id), parsed) ? juce::roundToInt (parsed) : fallback;
}

juce::String stringProperty (const juce::ValueTree& tree,
                             const juce::Identifier& id,
                             const juce::String& fallback)
{
    const auto s = tree.getProperty (id).toString().trim();
    return s.isNotEmpty() ? s : fallback;
}

bool readNumericVar (const juce::var& value, double& out)
{
    return numericVarToDouble (value, out);
}

bool readIntVar (const juce::var& value, int& out)
{
    double numeric = 0.0;
    if (! readNumericVar (value, numeric))
        return false;

    out = juce::roundToInt (numeric);
    return true;
}

bool enumContains (std::initializer_list<const char*> allowed, const juce::String& value)
{
    for (auto* item : allowed)
        if (value.equalsIgnoreCase (item))
            return true;

    return false;
}

juce::String canonicalEnum (std::initializer_list<const char*> allowed,
                            const juce::String& value,
                            const juce::String& fallback)
{
    for (auto* item : allowed)
        if (value.equalsIgnoreCase (item))
            return item;

    return fallback;
}

juce::ValueTree settingsTree (te::Edit& edit)
{
    return edit.state.getChildWithName (kSettingsTree);
}

juce::ValueTree ensureSettingsTree (te::Edit& edit,
                                    const juce::String& origin,
                                    juce::UndoManager* undo,
                                    bool& created)
{
    auto tree = settingsTree (edit);
    if (tree.isValid())
        return tree;

    tree = juce::ValueTree (kSettingsTree);
    tree.setProperty (kMigrationState,
                      origin.equalsIgnoreCase ("new_project") ? "defaulted_new_project"
                                                               : "defaulted_from_legacy",
                      nullptr);
    tree.setProperty (kDefaultedOrigin, origin, nullptr);
    edit.state.addChild (tree, -1, undo);
    created = true;
    return tree;
}

juce::var settingsToVar (te::Edit& edit,
                         const juce::String& metadataState,
                         bool defaultedThisCall)
{
    const auto tree = settingsTree (edit);
    const auto& d = defaults();
    auto out = std::make_unique<juce::DynamicObject>();

    out->setProperty ("schema_version", stringProperty (tree, kSchemaVersion, d.schemaVersion));
    out->setProperty ("sample_rate_hz", intProperty (tree, kSampleRateHz, d.sampleRateHz));
    out->setProperty ("record_file_type", stringProperty (tree, kRecordFileType, d.recordFileType));
    out->setProperty ("record_bit_depth", intProperty (tree, kRecordBitDepth, d.recordBitDepth));
    out->setProperty ("pcm_format", stringProperty (tree, kPcmFormat, d.pcmFormat));
    out->setProperty ("internal_processing_format", stringProperty (tree, kInternalProcessingFormat, d.internalProcessingFormat));
    out->setProperty ("import_sample_rate_policy", stringProperty (tree, kImportSampleRatePolicy, d.importSampleRatePolicy));
    out->setProperty ("import_bit_depth_policy", stringProperty (tree, kImportBitDepthPolicy, d.importBitDepthPolicy));
    out->setProperty ("media_copy_policy", stringProperty (tree, kMediaCopyPolicy, d.mediaCopyPolicy));
    out->setProperty ("channel_import_policy", stringProperty (tree, kChannelImportPolicy, d.channelImportPolicy));
    out->setProperty ("render_default_sample_rate_hz", intProperty (tree, kRenderDefaultSampleRateHz, d.renderDefaultSampleRateHz));
    out->setProperty ("render_default_bit_depth", intProperty (tree, kRenderDefaultBitDepth, d.renderDefaultBitDepth));
    out->setProperty ("render_default_file_type", stringProperty (tree, kRenderDefaultFileType, d.renderDefaultFileType));
    out->setProperty ("dither_policy", stringProperty (tree, kDitherPolicy, d.ditherPolicy));

    juce::Array<juce::var> recommendedPresets;
    auto addPreset = [&recommendedPresets] (const char* id,
                                            const char* name,
                                            const char* role,
                                            int sampleRateHz,
                                            int bitDepth,
                                            const char* fileType,
                                            bool defaultWorkingSpec)
    {
        auto preset = std::make_unique<juce::DynamicObject>();
        preset->setProperty ("preset_id", id);
        preset->setProperty ("name", name);
        preset->setProperty ("role", role);
        preset->setProperty ("sample_rate_hz", sampleRateHz);
        preset->setProperty ("bit_depth", bitDepth);
        preset->setProperty ("file_type", fileType);
        preset->setProperty ("default_project_working_spec", defaultWorkingSpec);
        recommendedPresets.add (juce::var (preset.release()));
    };
    addPreset ("default_production", "Default Production", "project_working", 48000, 24, "WAV/BWF", true);
    addPreset ("music_production", "Music Production", "project_working", 44100, 24, "WAV/BWF", false);
    addPreset ("video_game_production", "Video/Game Production", "project_working", 48000, 24, "WAV/BWF", false);
    addPreset ("cd_export", "CD Export", "delivery_export", 44100, 16, "WAV", false);
    addPreset ("high_resolution", "High Resolution", "project_working", 96000, 24, "WAV/BWF", false);
    out->setProperty ("recommended_presets", juce::var (recommendedPresets));

    out->setProperty ("metadata_state", metadataState);
    out->setProperty ("defaulted_this_call", defaultedThisCall);
    out->setProperty ("migration_state", stringProperty (tree, kMigrationState, "stored"));
    out->setProperty ("defaulted_origin", stringProperty (tree, kDefaultedOrigin, ""));

    auto fieldStatus = std::make_unique<juce::DynamicObject>();
    fieldStatus->setProperty ("sample_rate_hz", "metadata_active");
    fieldStatus->setProperty ("record_file_type", "declared_metadata_only");
    fieldStatus->setProperty ("record_bit_depth", "declared_metadata_only");
    fieldStatus->setProperty ("pcm_format", "declared_metadata_only");
    fieldStatus->setProperty ("internal_processing_format", "read_only_unknown");
    fieldStatus->setProperty ("import_sample_rate_policy", "metadata_active_for_preflight");
    fieldStatus->setProperty ("import_bit_depth_policy", "metadata_active_for_preflight");
    fieldStatus->setProperty ("media_copy_policy", "metadata_active_for_preflight");
    fieldStatus->setProperty ("channel_import_policy", "metadata_active_for_preflight");
    fieldStatus->setProperty ("render_defaults", "declared_metadata_only");
    fieldStatus->setProperty ("dither_policy", "declared_metadata_only");
    out->setProperty ("field_status", juce::var (fieldStatus.release()));

    auto caps = std::make_unique<juce::DynamicObject>();
    caps->setProperty ("project_sample_rate_is_audio_device_sample_rate", false);
    caps->setProperty ("changes_audio_device_sample_rate", false);
    caps->setProperty ("engine_project_sample_rate_binding", "not_bound");
    caps->setProperty ("record_format_binding", "metadata_only");
    caps->setProperty ("preflight_uses_settings", true);
    out->setProperty ("capabilities", juce::var (caps.release()));

    return juce::var (out.release());
}

double currentDeviceSampleRate (te::Edit& edit)
{
    auto setup = edit.engine.getDeviceManager().deviceManager.getAudioDeviceSetup();
    return setup.sampleRate;
}

juce::var buildSettingsWarnings (te::Edit& edit)
{
    juce::Array<juce::var> warnings;
    const auto tree = settingsTree (edit);
    const auto projectSr = intProperty (tree, kSampleRateHz, kDefaultSampleRateHz);
    const auto deviceSr = currentDeviceSampleRate (edit);

    if (deviceSr > 0.0 && std::abs (deviceSr - (double) projectSr) > 1.0)
    {
        auto warning = std::make_unique<juce::DynamicObject>();
        warning->setProperty ("code", "project_sample_rate_differs_from_audio_device");
        warning->setProperty ("message", "Project sample rate differs from the current audio device sample rate; this setting does not change the audio device.");
        warning->setProperty ("project_sample_rate_hz", projectSr);
        warning->setProperty ("audio_device_sample_rate_hz", deviceSr);
        warnings.add (juce::var (warning.release()));
    }

    return juce::var (warnings);
}

int countAudioClips (te::Edit& edit)
{
    int count = 0;
    for (auto* track : te::getAllTracks (edit))
    {
        if (track == nullptr)
            continue;

        const int n = track->getNumTrackItems();
        for (int i = 0; i < n; ++i)
            if (dynamic_cast<te::AudioClipBase*> (track->getTrackItem (i)) != nullptr)
                ++count;
    }

    return count;
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

juce::var inspectAudioFile (te::Edit& edit, const juce::File& file)
{
    auto row = std::make_unique<juce::DynamicObject>();
    juce::Array<juce::var> warnings;
    juce::Array<juce::var> errors;

    const auto fileType = lowerExtensionWithoutDot (file);
    row->setProperty ("file_path", file.getFullPathName());
    row->setProperty ("file_name", file.getFileName());
    row->setProperty ("file_type", fileType);
    row->setProperty ("readable", false);
    row->setProperty ("duration_seconds", juce::var());
    row->setProperty ("sample_rate_hz", juce::var());
    row->setProperty ("bit_depth", juce::var());
    row->setProperty ("pcm_format", "unknown");
    row->setProperty ("channel_count", juce::var());
    row->setProperty ("compressed_bitrate_kbps", juce::var());
    row->setProperty ("suggested_track_name", suggestedTrackName (file));

    if (! file.existsAsFile())
    {
        errors.add ("file_not_found");
        row->setProperty ("errors", juce::var (errors));
        row->setProperty ("warnings", juce::var (warnings));
        return juce::var (row.release());
    }

    if (! isSupportedAudioExtension (file))
    {
        errors.add ("unsupported_file_type");
        row->setProperty ("errors", juce::var (errors));
        row->setProperty ("warnings", juce::var (warnings));
        return juce::var (row.release());
    }

    std::unique_ptr<juce::AudioFormatReader> reader (
        edit.engine.getAudioFileFormatManager().readFormatManager.createReaderFor (file));

    if (reader == nullptr || reader->sampleRate <= 0.0 || reader->lengthInSamples <= 0)
    {
        errors.add ("reader_failed");
        row->setProperty ("errors", juce::var (errors));
        row->setProperty ("warnings", juce::var (warnings));
        return juce::var (row.release());
    }

    const auto durationSeconds = (double) reader->lengthInSamples / reader->sampleRate;
    row->setProperty ("readable", true);
    row->setProperty ("duration_seconds", durationSeconds);
    row->setProperty ("sample_rate_hz", juce::roundToInt (reader->sampleRate));
    row->setProperty ("channel_count", (int) reader->numChannels);
    row->setProperty ("format_name", reader->getFormatName());

    if (fileType == "mp3")
    {
        warnings.add ("limited_mp3_metadata");
        row->setProperty ("pcm_format", "compressed_mp3_decoded_float32");

        if (durationSeconds > 0.0)
        {
            const auto kbps = ((double) file.getSize() * 8.0) / durationSeconds / 1000.0;
            row->setProperty ("compressed_bitrate_kbps", juce::roundToInt (kbps));
        }
    }
    else
    {
        const auto bitDepth = (int) reader->bitsPerSample;
        if (bitDepth > 0)
            row->setProperty ("bit_depth", bitDepth);
        else
            warnings.add ("bit_depth_unknown");

        row->setProperty ("pcm_format",
                          reader->usesFloatingPointData ? "float" + juce::String (juce::jmax (bitDepth, 32))
                                                        : "int" + juce::String (bitDepth));
    }

    row->setProperty ("warnings", juce::var (warnings));
    row->setProperty ("errors", juce::var (errors));
    return juce::var (row.release());
}

int getIntFromObject (const juce::DynamicObject& object, const juce::String& key, int fallback)
{
    int out = fallback;
    if (readIntVar (object.getProperty (key), out))
        return out;
    return fallback;
}

juce::String getStringFromObject (const juce::DynamicObject& object,
                                  const juce::String& key,
                                  const juce::String& fallback)
{
    const auto value = object.getProperty (key).toString().trim();
    return value.isNotEmpty() ? value : fallback;
}

juce::DynamicObject* objectFromVar (const juce::var& value)
{
    return value.getDynamicObject();
}

struct PreflightSettings
{
    int sampleRateHz = kDefaultSampleRateHz;
    int recordBitDepth = kDefaultRecordBitDepth;
    juce::String importSampleRatePolicy = "ask";
    juce::String importBitDepthPolicy = "keep_source";
    juce::String mediaCopyPolicy = "reference_original";
    juce::String channelImportPolicy = "preserve_interleaved";
};

PreflightSettings settingsFromEdit (te::Edit& edit)
{
    const auto tree = settingsTree (edit);
    PreflightSettings s;
    s.sampleRateHz = intProperty (tree, kSampleRateHz, kDefaultSampleRateHz);
    s.recordBitDepth = intProperty (tree, kRecordBitDepth, kDefaultRecordBitDepth);
    s.importSampleRatePolicy = stringProperty (tree, kImportSampleRatePolicy, "ask");
    s.importBitDepthPolicy = stringProperty (tree, kImportBitDepthPolicy, "keep_source");
    s.mediaCopyPolicy = stringProperty (tree, kMediaCopyPolicy, "reference_original");
    s.channelImportPolicy = stringProperty (tree, kChannelImportPolicy, "preserve_interleaved");
    return s;
}

PreflightSettings settingsFromSnapshot (const juce::var& snapshot, PreflightSettings fallback)
{
    if (auto* object = objectFromVar (snapshot))
    {
        fallback.sampleRateHz = getIntFromObject (*object, "sample_rate_hz", fallback.sampleRateHz);
        fallback.recordBitDepth = getIntFromObject (*object, "record_bit_depth", fallback.recordBitDepth);
        fallback.importSampleRatePolicy = getStringFromObject (*object, "import_sample_rate_policy", fallback.importSampleRatePolicy);
        fallback.importBitDepthPolicy = getStringFromObject (*object, "import_bit_depth_policy", fallback.importBitDepthPolicy);
        fallback.mediaCopyPolicy = getStringFromObject (*object, "media_copy_policy", fallback.mediaCopyPolicy);
        fallback.channelImportPolicy = getStringFromObject (*object, "channel_import_policy", fallback.channelImportPolicy);
    }

    return fallback;
}

juce::var sampleRateDistributionToVar (const std::map<int, int>& sourceSampleRateCounts)
{
    juce::Array<juce::var> rows;

    for (const auto& [sampleRateHz, count] : sourceSampleRateCounts)
    {
        auto row = std::make_unique<juce::DynamicObject>();
        row->setProperty ("sample_rate_hz", sampleRateHz);
        row->setProperty ("file_count", count);
        rows.add (juce::var (row.release()));
    }

    return juce::var (rows);
}

juce::var projectAudioSettingsPatchForSampleRate (int sampleRateHz)
{
    auto patch = std::make_unique<juce::DynamicObject>();
    patch->setProperty ("sample_rate_hz", sampleRateHz);
    patch->setProperty ("render_default_sample_rate_hz", sampleRateHz);
    return juce::var (patch.release());
}

juce::var buildSampleRateDecision (const PreflightSettings& settings,
                                   int projectAudioClipCount,
                                   int readableCount,
                                   const std::map<int, int>& sourceSampleRateCounts,
                                   int sampleRateMismatchCount)
{
    const bool hasMismatch = sampleRateMismatchCount > 0;
    const bool projectIsEmpty = projectAudioClipCount <= 0;
    const bool hasUniqueReadableSourceRate = sourceSampleRateCounts.size() == 1;
    const int uniqueSourceSampleRate = hasUniqueReadableSourceRate ? sourceSampleRateCounts.begin()->first : 0;
    const bool canSwitchProjectSampleRate = hasMismatch
                                         && projectIsEmpty
                                         && readableCount > 0
                                         && uniqueSourceSampleRate > 0
                                         && uniqueSourceSampleRate != settings.sampleRateHz;

    auto decision = std::make_unique<juce::DynamicObject>();
    decision->setProperty ("status", hasMismatch ? "mismatch" : "match");
    decision->setProperty ("project_sample_rate_hz", settings.sampleRateHz);
    decision->setProperty ("readable_file_count", readableCount);
    decision->setProperty ("sample_rate_mismatch_count", sampleRateMismatchCount);
    decision->setProperty ("source_sample_rate_distribution", sampleRateDistributionToVar (sourceSampleRateCounts));
    decision->setProperty ("source_sample_rate_is_unique", hasUniqueReadableSourceRate);
    decision->setProperty ("project_audio_clip_count", projectAudioClipCount);
    decision->setProperty ("project_is_empty", projectIsEmpty);
    decision->setProperty ("can_switch_project_sample_rate", canSwitchProjectSampleRate);
    decision->setProperty ("requires_user_choice", hasMismatch);
    decision->setProperty ("import_sample_rate_policy", settings.importSampleRatePolicy);

    if (uniqueSourceSampleRate > 0)
        decision->setProperty ("source_sample_rate_hz", uniqueSourceSampleRate);

    juce::Array<juce::var> availableActions;
    juce::String recommendedAction;

    if (! hasMismatch)
    {
        recommendedAction = "import";
        availableActions.add (juce::var ("import"));
    }
    else if (canSwitchProjectSampleRate)
    {
        recommendedAction = "ask_user_switch_or_keep_project_rate";
        availableActions.add (juce::var ("switch_project_sample_rate_then_import"));
        availableActions.add (juce::var ("keep_project_sample_rate_and_import"));
        availableActions.add (juce::var ("reject_import"));
        decision->setProperty ("project_audio_settings_patch",
                               projectAudioSettingsPatchForSampleRate (uniqueSourceSampleRate));
    }
    else
    {
        recommendedAction = "keep_project_sample_rate_confirm_import";
        availableActions.add (juce::var ("keep_project_sample_rate_and_import"));
        availableActions.add (juce::var ("reject_import"));
        if (projectIsEmpty)
            availableActions.add (juce::var ("set_project_sample_rate_manually"));
    }

    decision->setProperty ("recommended_action", recommendedAction);
    decision->setProperty ("available_actions", juce::var (availableActions));
    return juce::var (decision.release());
}

bool boolProperty (const juce::DynamicObject& object, const juce::String& key, bool fallback)
{
    const auto value = object.getProperty (key);
    return value.isBool() ? static_cast<bool> (value) : fallback;
}

double doubleProperty (const juce::DynamicObject& object, const juce::String& key, double fallback)
{
    double value = fallback;
    return readNumericVar (object.getProperty (key), value) ? value : fallback;
}

juce::var buildPreflight (te::Edit& edit, const juce::DynamicObject& object, bool planMode)
{
    ProjectAudioSettingsService::ensureDefaultAudioSettings (edit, "import_preflight");

    auto files = filesFromVar (object.getProperty ("file_paths"));
    const auto folderPath = object.getProperty ("folder_path").toString().trim();
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

    auto settings = settingsFromEdit (edit);
    settings = settingsFromSnapshot (object.getProperty ("audio_settings_snapshot"), settings);

    const auto intendedMode = getStringFromObject (object, "intended_mode", planMode ? "stems_folder" : "inspect_files");
    const auto targetPolicy = getStringFromObject (object, "target_policy", "create_tracks");
    const double startTimeSeconds = juce::jmax (0.0, doubleProperty (object, "start_time_seconds", 0.0));

    juce::Array<juce::var> fileRows;
    juce::Array<juce::var> planTracks;
    juce::Array<juce::var> sampleRateMismatches;
    juce::Array<juce::var> bitDepthMismatches;
    juce::Array<juce::var> unreadableFiles;
    std::map<int, int> sourceSampleRateCounts;

    int readableCount = 0;

    for (auto file : uniqueFiles)
    {
        auto inspected = inspectAudioFile (edit, file);
        auto* row = inspected.getDynamicObject();

        if (row != nullptr)
        {
            const bool readable = static_cast<bool> (row->getProperty ("readable"));
            if (readable)
            {
                ++readableCount;

                const int fileSr = getIntFromObject (*row, "sample_rate_hz", 0);
                const int bitDepth = getIntFromObject (*row, "bit_depth", 0);
                const auto pcmFormat = row->getProperty ("pcm_format").toString();
                if (fileSr > 0)
                    ++sourceSampleRateCounts[fileSr];

                auto track = std::make_unique<juce::DynamicObject>();
                track->setProperty ("file_path", file.getFullPathName());
                track->setProperty ("suggested_track_name", row->getProperty ("suggested_track_name"));
                track->setProperty ("start_time_seconds", startTimeSeconds);
                track->setProperty ("channel_import_policy", settings.channelImportPolicy);
                track->setProperty ("media_copy_policy", settings.mediaCopyPolicy);

                const bool srMismatch = fileSr > 0 && fileSr != settings.sampleRateHz;
                const bool bitMismatch = bitDepth > 0 && bitDepth != settings.recordBitDepth;
                const bool floatMismatch = pcmFormat.startsWithIgnoreCase ("float")
                                        && settings.importBitDepthPolicy == "convert_to_project_format";

                track->setProperty ("needs_sample_rate_conversion", srMismatch);
                track->setProperty ("sample_rate_policy", settings.importSampleRatePolicy);
                track->setProperty ("bit_depth_or_format_differs", bitMismatch || floatMismatch);
                track->setProperty ("bit_depth_policy", settings.importBitDepthPolicy);

                if (srMismatch)
                {
                    auto mismatch = std::make_unique<juce::DynamicObject>();
                    mismatch->setProperty ("file_path", file.getFullPathName());
                    mismatch->setProperty ("source_sample_rate_hz", fileSr);
                    mismatch->setProperty ("project_sample_rate_hz", settings.sampleRateHz);
                    mismatch->setProperty ("policy", settings.importSampleRatePolicy);
                    sampleRateMismatches.add (juce::var (mismatch.release()));
                }

                if (bitMismatch || floatMismatch)
                {
                    auto mismatch = std::make_unique<juce::DynamicObject>();
                    mismatch->setProperty ("file_path", file.getFullPathName());
                    mismatch->setProperty ("source_bit_depth", bitDepth > 0 ? juce::var (bitDepth) : juce::var());
                    mismatch->setProperty ("source_pcm_format", pcmFormat);
                    mismatch->setProperty ("project_record_bit_depth", settings.recordBitDepth);
                    mismatch->setProperty ("policy", settings.importBitDepthPolicy);
                    bitDepthMismatches.add (juce::var (mismatch.release()));
                }

                planTracks.add (juce::var (track.release()));
            }
            else
            {
                unreadableFiles.add (file.getFullPathName());
            }
        }

        fileRows.add (inspected);
    }

    const auto sampleRateDecision = buildSampleRateDecision (settings,
                                                             countAudioClips (edit),
                                                             readableCount,
                                                             sourceSampleRateCounts,
                                                             sampleRateMismatches.size());
    auto* sampleRateDecisionObject = sampleRateDecision.getDynamicObject();
    const bool sampleRateDecisionRequiresChoice = sampleRateDecisionObject != nullptr
                                               && static_cast<bool> (sampleRateDecisionObject->getProperty ("requires_user_choice"));
    const auto projectAudioSettingsPatch = sampleRateDecisionObject != nullptr
        ? sampleRateDecisionObject->getProperty ("project_audio_settings_patch")
        : juce::var();

    const bool needsConfirmation = sampleRateDecisionRequiresChoice
                                || ((settings.importSampleRatePolicy == "ask" || settings.importSampleRatePolicy == "reject_mismatch")
                                        && sampleRateMismatches.size() > 0)
                                || (settings.importBitDepthPolicy == "ask" && bitDepthMismatches.size() > 0)
                                || settings.mediaCopyPolicy == "ask"
                                || unreadableFiles.size() > 0;

    auto summary = std::make_unique<juce::DynamicObject>();
    summary->setProperty ("discovered_audio_file_count", uniqueFiles.size());
    summary->setProperty ("readable_file_count", readableCount);
    summary->setProperty ("unreadable_file_count", unreadableFiles.size());
    summary->setProperty ("tracks_to_create", planMode ? readableCount : 0);
    summary->setProperty ("sample_rate_mismatch_count", sampleRateMismatches.size());
    summary->setProperty ("bit_depth_or_format_mismatch_count", bitDepthMismatches.size());
    summary->setProperty ("requires_user_confirmation", needsConfirmation);
    summary->setProperty ("copy_policy", settings.mediaCopyPolicy);
    summary->setProperty ("recommended_copy_policy", "reference_original");
    summary->setProperty ("sample_rate_decision", sampleRateDecision);
    if (! projectAudioSettingsPatch.isVoid())
        summary->setProperty ("project_audio_settings_patch", projectAudioSettingsPatch);

    auto settingsSnap = std::make_unique<juce::DynamicObject>();
    settingsSnap->setProperty ("sample_rate_hz", settings.sampleRateHz);
    settingsSnap->setProperty ("record_bit_depth", settings.recordBitDepth);
    settingsSnap->setProperty ("import_sample_rate_policy", settings.importSampleRatePolicy);
    settingsSnap->setProperty ("import_bit_depth_policy", settings.importBitDepthPolicy);
    settingsSnap->setProperty ("media_copy_policy", settings.mediaCopyPolicy);
    settingsSnap->setProperty ("channel_import_policy", settings.channelImportPolicy);

    auto plan = std::make_unique<juce::DynamicObject>();
    plan->setProperty ("intended_mode", intendedMode);
    plan->setProperty ("target_policy", targetPolicy);
    plan->setProperty ("start_time_seconds", startTimeSeconds);
    plan->setProperty ("tracks_to_create", planMode ? readableCount : 0);
    plan->setProperty ("track_plan", juce::var (planTracks));
    plan->setProperty ("sample_rate_mismatches", juce::var (sampleRateMismatches));
    plan->setProperty ("bit_depth_or_format_mismatches", juce::var (bitDepthMismatches));
    plan->setProperty ("unreadable_files", juce::var (unreadableFiles));
    plan->setProperty ("copy_reference_strategy", settings.mediaCopyPolicy);
    plan->setProperty ("requires_user_confirmation", needsConfirmation);
    plan->setProperty ("sample_rate_decision", sampleRateDecision);
    if (! projectAudioSettingsPatch.isVoid())
        plan->setProperty ("project_audio_settings_patch", projectAudioSettingsPatch);

    auto response = std::make_unique<juce::DynamicObject>();
    response->setProperty ("status", "ok");
    response->setProperty ("command", planMode ? "project.import_preflight" : "media.inspect_files");
    response->setProperty ("files", juce::var (fileRows));
    response->setProperty ("summary", juce::var (summary.release()));
    response->setProperty ("audio_settings_snapshot", juce::var (settingsSnap.release()));
    response->setProperty ("sample_rate_decision", sampleRateDecision);
    if (! projectAudioSettingsPatch.isVoid())
        response->setProperty ("project_audio_settings_patch", projectAudioSettingsPatch);
    if (planMode)
        response->setProperty ("import_plan", juce::var (plan.release()));

    return juce::var (response.release());
}

} // namespace

ProjectAudioSettingsService::ProjectAudioSettingsService (EditGetter editGetter,
                                                          SaveProjectAction saveProjectAction)
    : getEdit (std::move (editGetter)),
      saveProject (std::move (saveProjectAction))
{
}

ProjectAudioSettingsService::EnsureResult ProjectAudioSettingsService::ensureDefaultAudioSettings (te::Edit& edit,
                                                                                                    const juce::String& origin,
                                                                                                    bool useUndo)
{
    EnsureResult result;
    auto* undo = undoFor (edit, useUndo);
    bool created = false;
    auto tree = ensureSettingsTree (edit, origin, undo, created);
    result.created = created;
    result.changed = created;

    const auto& d = defaults();
    result.patched = setIfMissing (tree, kSchemaVersion, d.schemaVersion, undo) || result.patched;
    result.patched = setIfMissing (tree, kSampleRateHz, d.sampleRateHz, undo) || result.patched;
    result.patched = setIfMissing (tree, kRecordFileType, d.recordFileType, undo) || result.patched;
    result.patched = setIfMissing (tree, kRecordBitDepth, d.recordBitDepth, undo) || result.patched;
    result.patched = setIfMissing (tree, kPcmFormat, d.pcmFormat, undo) || result.patched;
    result.patched = setIfMissing (tree, kInternalProcessingFormat, d.internalProcessingFormat, undo) || result.patched;
    result.patched = setIfMissing (tree, kImportSampleRatePolicy, d.importSampleRatePolicy, undo) || result.patched;
    result.patched = setIfMissing (tree, kImportBitDepthPolicy, d.importBitDepthPolicy, undo) || result.patched;
    result.patched = setIfMissing (tree, kMediaCopyPolicy, d.mediaCopyPolicy, undo) || result.patched;
    result.patched = setIfMissing (tree, kChannelImportPolicy, d.channelImportPolicy, undo) || result.patched;
    result.patched = setIfMissing (tree, kRenderDefaultSampleRateHz, d.renderDefaultSampleRateHz, undo) || result.patched;
    result.patched = setIfMissing (tree, kRenderDefaultBitDepth, d.renderDefaultBitDepth, undo) || result.patched;
    result.patched = setIfMissing (tree, kRenderDefaultFileType, d.renderDefaultFileType, undo) || result.patched;
    result.patched = setIfMissing (tree, kDitherPolicy, d.ditherPolicy, undo) || result.patched;
    result.patched = setIfMissing (tree, kMigrationState, "stored", undo) || result.patched;

    result.changed = result.changed || result.patched;
    return result;
}

bool ProjectAudioSettingsService::applyAudioSettingsToEdit (te::Edit& edit,
                                                            const juce::DynamicObject& source,
                                                            const juce::String& origin,
                                                            bool useUndo,
                                                            juce::Array<juce::var>* warningsOut)
{
    juce::Array<juce::var> localWarnings;
    auto& warnings = warningsOut != nullptr ? *warningsOut : localWarnings;

    ensureDefaultAudioSettings (edit, origin, useUndo);
    auto tree = settingsTree (edit);
    auto* undo = undoFor (edit, useUndo);
    bool accepted = true;

    auto setInt = [&] (const char* key, const juce::Identifier& id, int minValue, int maxValue) -> bool
    {
        const auto value = source.getProperty (key);
        if (value.isVoid())
            return true;

        int parsed = 0;
        if (! readIntVar (value, parsed))
        {
            warnings.add (juce::String ("ignored_non_numeric_") + key);
            return false;
        }

        if (parsed < minValue || parsed > maxValue)
        {
            warnings.add (juce::String ("ignored_out_of_range_") + key);
            return false;
        }

        tree.setProperty (id, parsed, undo);
        return true;
    };

    auto setEnum = [&] (const char* key, const juce::Identifier& id, std::initializer_list<const char*> allowed) -> bool
    {
        const auto value = source.getProperty (key);
        if (value.isVoid())
            return true;

        const auto parsed = value.toString().trim();
        if (! enumContains (allowed, parsed))
        {
            warnings.add (juce::String ("ignored_invalid_") + key);
            return false;
        }

        tree.setProperty (id, canonicalEnum (allowed, parsed, parsed), undo);
        return true;
    };

    accepted = setInt ("sample_rate_hz", kSampleRateHz, 8000, 384000) && accepted;
    accepted = setInt ("record_bit_depth", kRecordBitDepth, 16, 64) && accepted;
    accepted = setInt ("render_default_sample_rate_hz", kRenderDefaultSampleRateHz, 8000, 384000) && accepted;
    accepted = setInt ("render_default_bit_depth", kRenderDefaultBitDepth, 16, 64) && accepted;

    accepted = setEnum ("record_file_type", kRecordFileType, { "WAV/BWF", "WAV", "BWF", "AIFF", "FLAC" }) && accepted;
    accepted = setEnum ("pcm_format", kPcmFormat, { "int16", "int24", "int32", "float32" }) && accepted;
    accepted = setEnum ("import_sample_rate_policy", kImportSampleRatePolicy, { "ask", "convert_to_project", "keep_source", "reject_mismatch" }) && accepted;
    accepted = setEnum ("import_bit_depth_policy", kImportBitDepthPolicy, { "keep_source", "convert_to_project_format", "ask" }) && accepted;
    accepted = setEnum ("media_copy_policy", kMediaCopyPolicy, { "reference_original", "copy_to_project_media", "ask" }) && accepted;
    accepted = setEnum ("channel_import_policy", kChannelImportPolicy, { "preserve_interleaved", "split_mono", "ask" }) && accepted;
    accepted = setEnum ("render_default_file_type", kRenderDefaultFileType, { "WAV", "WAV/BWF", "BWF", "AIFF", "FLAC" }) && accepted;
    accepted = setEnum ("dither_policy", kDitherPolicy, { "none", "unknown", "ask" }) && accepted;

    tree.setProperty (kSchemaVersion, defaults().schemaVersion, undo);
    tree.setProperty (kMigrationState, "stored", undo);

    edit.dispatchPendingUpdatesSynchronously();
    return accepted;
}

juce::String ProjectAudioSettingsService::handleGetAudioSettings (const juce::DynamicObject&, const juce::String&) const
{
    auto* edit = getEdit != nullptr ? getEdit() : nullptr;
    if (edit == nullptr)
        return makeErrorReply ("No active edit loaded");

    const auto ensure = ensureDefaultAudioSettings (*edit, "get_audio_settings");
    auto response = std::make_unique<juce::DynamicObject>();
    response->setProperty ("status", "ok");
    response->setProperty ("command", "project.get_audio_settings");
    response->setProperty ("audio_settings", settingsToVar (*edit, ensure.changed ? "defaulted_or_patched" : "stored", ensure.changed));
    response->setProperty ("warnings", buildSettingsWarnings (*edit));
    return juce::JSON::toString (juce::var (response.release()));
}

juce::String ProjectAudioSettingsService::handleSetAudioSettings (const juce::DynamicObject& object, const juce::String&) const
{
    auto* edit = getEdit != nullptr ? getEdit() : nullptr;
    if (edit == nullptr)
        return makeErrorReply ("No active edit loaded");

    ensureDefaultAudioSettings (*edit, "set_audio_settings", true);
    auto tree = settingsTree (*edit);
    auto& undo = edit->getUndoManager();
    undo.beginNewTransaction ("Set project audio settings");

    juce::Array<juce::var> warnings;

    auto maybeSettingsObject = object.getProperty ("audio_settings").getDynamicObject();
    const auto& source = maybeSettingsObject != nullptr ? *maybeSettingsObject : object;
    const auto changeOrigin = getStringFromObject (object, "change_origin",
                                                   getStringFromObject (object, "origin", ""));
    const bool importSampleRatePreflightOrigin = changeOrigin.equalsIgnoreCase ("stems_import_sample_rate_preflight");
    if (importSampleRatePreflightOrigin && ! boolProperty (object, "allow_import_sample_rate_switch", false))
        return makeErrorReply ("Import preflight sample-rate changes require explicit allow_import_sample_rate_switch=true");

    if (importSampleRatePreflightOrigin
        && (! source.getProperty ("record_bit_depth").isVoid()
            || ! source.getProperty ("render_default_bit_depth").isVoid()))
        return makeErrorReply ("Import preflight sample-rate changes must not change project bit-depth metadata");

    auto setInt = [&] (const char* key, const juce::Identifier& id, int minValue, int maxValue) -> bool
    {
        const auto value = source.getProperty (key);
        if (value.isVoid())
            return true;

        int parsed = 0;
        if (! readIntVar (value, parsed))
        {
            warnings.add (juce::String ("ignored_non_numeric_") + key);
            return false;
        }

        if (parsed < minValue || parsed > maxValue)
        {
            warnings.add (juce::String ("ignored_out_of_range_") + key);
            return false;
        }

        tree.setProperty (id, parsed, &undo);
        return true;
    };

    auto setEnum = [&] (const char* key, const juce::Identifier& id, std::initializer_list<const char*> allowed) -> bool
    {
        const auto value = source.getProperty (key);
        if (value.isVoid())
            return true;

        const auto parsed = value.toString().trim();
        if (! enumContains (allowed, parsed))
        {
            warnings.add (juce::String ("ignored_invalid_") + key);
            return false;
        }

        tree.setProperty (id, canonicalEnum (allowed, parsed, parsed), &undo);
        return true;
    };

    setInt ("sample_rate_hz", kSampleRateHz, 8000, 384000);
    setInt ("record_bit_depth", kRecordBitDepth, 16, 64);
    setInt ("render_default_sample_rate_hz", kRenderDefaultSampleRateHz, 8000, 384000);
    setInt ("render_default_bit_depth", kRenderDefaultBitDepth, 16, 64);

    setEnum ("record_file_type", kRecordFileType, { "WAV/BWF", "WAV", "BWF", "AIFF", "FLAC" });
    setEnum ("pcm_format", kPcmFormat, { "int16", "int24", "int32", "float32" });
    setEnum ("import_sample_rate_policy", kImportSampleRatePolicy, { "ask", "convert_to_project", "keep_source", "reject_mismatch" });
    setEnum ("import_bit_depth_policy", kImportBitDepthPolicy, { "keep_source", "convert_to_project_format", "ask" });
    setEnum ("media_copy_policy", kMediaCopyPolicy, { "reference_original", "copy_to_project_media", "ask" });
    setEnum ("channel_import_policy", kChannelImportPolicy, { "preserve_interleaved", "split_mono", "ask" });
    setEnum ("render_default_file_type", kRenderDefaultFileType, { "WAV", "WAV/BWF", "BWF", "AIFF", "FLAC" });
    setEnum ("dither_policy", kDitherPolicy, { "none", "unknown", "ask" });

    tree.setProperty (kSchemaVersion, defaults().schemaVersion, &undo);
    tree.setProperty (kMigrationState, "stored", &undo);

    edit->dispatchPendingUpdatesSynchronously();

    if (saveProject && ! saveProject())
        return makeErrorReply ("Audio settings updated in memory but failed to save project");

    auto deviceWarnings = buildSettingsWarnings (*edit);
    if (auto* arr = deviceWarnings.getArray())
        warnings.addArray (*arr);

    auto response = std::make_unique<juce::DynamicObject>();
    response->setProperty ("status", "ok");
    response->setProperty ("command", "project.set_audio_settings");
    response->setProperty ("message", "Project audio settings updated");
    response->setProperty ("audio_settings", settingsToVar (*edit, "stored", false));
    response->setProperty ("warnings", juce::var (warnings));
    return juce::JSON::toString (juce::var (response.release()));
}

juce::String ProjectAudioSettingsService::handleValidateAudioSettingsChange (const juce::DynamicObject& object, const juce::String&) const
{
    auto* edit = getEdit != nullptr ? getEdit() : nullptr;
    if (edit == nullptr)
        return makeErrorReply ("No active edit loaded");

    ensureDefaultAudioSettings (*edit, "validate_audio_settings_change");
    const auto tree = settingsTree (*edit);

    const int oldSr = intProperty (tree, kSampleRateHz, kDefaultSampleRateHz);
    const int oldBitDepth = intProperty (tree, kRecordBitDepth, kDefaultRecordBitDepth);
    int newSr = oldSr;
    int newBitDepth = oldBitDepth;

    auto* settingsObject = object.getProperty ("audio_settings").getDynamicObject();
    const auto& source = settingsObject != nullptr ? *settingsObject : object;
    readIntVar (source.getProperty ("sample_rate_hz"), newSr);
    readIntVar (source.getProperty ("record_bit_depth"), newBitDepth);

    const int clipCount = countAudioClips (*edit);
    juce::Array<juce::var> warnings;

    if (clipCount > 0 && newSr != oldSr)
    {
        auto warning = std::make_unique<juce::DynamicObject>();
        warning->setProperty ("code", "sample_rate_change_with_existing_audio_clips");
        warning->setProperty ("message", "Changing the project sample-rate metadata while audio clips already exist may require import conversion policy review.");
        warning->setProperty ("audio_clip_count", clipCount);
        warning->setProperty ("old_sample_rate_hz", oldSr);
        warning->setProperty ("new_sample_rate_hz", newSr);
        warnings.add (juce::var (warning.release()));
    }

    if (clipCount > 0 && newBitDepth != oldBitDepth)
    {
        auto warning = std::make_unique<juce::DynamicObject>();
        warning->setProperty ("code", "record_bit_depth_change_with_existing_audio_clips");
        warning->setProperty ("message", "Changing record bit-depth metadata does not rewrite existing media and may affect future recording/import decisions.");
        warning->setProperty ("audio_clip_count", clipCount);
        warning->setProperty ("old_record_bit_depth", oldBitDepth);
        warning->setProperty ("new_record_bit_depth", newBitDepth);
        warnings.add (juce::var (warning.release()));
    }

    auto deviceWarnings = buildSettingsWarnings (*edit);
    if (auto* arr = deviceWarnings.getArray())
        warnings.addArray (*arr);

    auto response = std::make_unique<juce::DynamicObject>();
    response->setProperty ("status", "ok");
    response->setProperty ("command", "project.validate_audio_settings_change");
    response->setProperty ("safe_to_apply", warnings.isEmpty());
    response->setProperty ("audio_clip_count", clipCount);
    response->setProperty ("warnings", juce::var (warnings));
    response->setProperty ("changes_audio_device_sample_rate", false);
    return juce::JSON::toString (juce::var (response.release()));
}

juce::String ProjectAudioSettingsService::handleImportPreflight (const juce::DynamicObject& object, const juce::String&) const
{
    auto* edit = getEdit != nullptr ? getEdit() : nullptr;
    if (edit == nullptr)
        return makeErrorReply ("No active edit loaded");

    return juce::JSON::toString (buildPreflight (*edit, object, true));
}

juce::String ProjectAudioSettingsService::handleInspectFiles (const juce::DynamicObject& object, const juce::String&) const
{
    auto* edit = getEdit != nullptr ? getEdit() : nullptr;
    if (edit == nullptr)
        return makeErrorReply ("No active edit loaded");

    return juce::JSON::toString (buildPreflight (*edit, object, false));
}

juce::String ProjectAudioSettingsService::makeStatusReply (const juce::String& status, const juce::String& message)
{
    auto response = std::make_unique<juce::DynamicObject>();
    response->setProperty ("status", status);
    response->setProperty ("message", message);
    return juce::JSON::toString (juce::var (response.release()));
}

juce::String ProjectAudioSettingsService::makeErrorReply (const juce::String& message)
{
    return makeStatusReply ("error", message);
}

} // namespace vit
