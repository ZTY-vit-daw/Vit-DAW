#include "ClipService.h"

#include "AudioFeatureService.h"

#include <algorithm>
#include <cmath>
#include <unordered_set>
#include <vector>

namespace vit
{

namespace
{

enum class CommandTimeUnit
{
    seconds,
    beats
};

enum class OverlapPolicy
{
    trim,
    layer,
    crossfade,
};

constexpr double kMinimumSurvivingClipLengthSeconds = 0.01;
constexpr double kClipEditEpsilonSeconds = 0.0005;
constexpr double kDefaultStripThresholdDbfs = -45.0;
constexpr double kDefaultStripMinimumSilenceSeconds = 0.12;
constexpr double kDefaultStripStartPadSeconds = 0.015;
constexpr double kDefaultStripEndPadSeconds = 0.050;
constexpr double kDefaultStripFrameSeconds = 0.010;
constexpr double kMinimumStripRegionSeconds = 0.010;

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

te::Track* findTrackByID (te::Edit& edit, const juce::String& trackID)
{
    for (auto* track : te::getAllTracks (edit))
        if (track != nullptr && track->itemID.toString() == trackID)
            return track;

    return nullptr;
}

bool parseTimeUnit (const juce::DynamicObject& object, CommandTimeUnit& out, juce::String& error)
{
    const auto raw = object.getProperty ("time_unit").toString().trim().toLowerCase();

    if (raw.isEmpty() || raw == "seconds" || raw == "second" || raw == "sec" || raw == "s")
    {
        out = CommandTimeUnit::seconds;
        return true;
    }

    if (raw == "beat" || raw == "beats")
    {
        out = CommandTimeUnit::beats;
        return true;
    }

    error = "time_unit must be one of: seconds, beats";
    return false;
}

bool readNumericProperty (const juce::DynamicObject& object, const juce::StringArray& keys, double& out)
{
    for (const auto& k : keys)
    {
        const auto v = object.getProperty (k);

        if (v.isDouble() || v.isInt() || v.isInt64())
        {
            out = static_cast<double> (v);
            return true;
        }
    }

    return false;
}

bool readBoolProperty (const juce::DynamicObject& object, const juce::StringArray& keys, bool& out)
{
    for (const auto& k : keys)
    {
        const auto v = object.getProperty (k);

        if (v.isBool())
        {
            out = static_cast<bool> (v);
            return true;
        }

        const auto raw = v.toString().trim().toLowerCase();

        if (raw == "true" || raw == "1" || raw == "yes" || raw == "on")
        {
            out = true;
            return true;
        }

        if (raw == "false" || raw == "0" || raw == "no" || raw == "off")
        {
            out = false;
            return true;
        }
    }

    return false;
}

void regenerateValueTreeIDs (juce::ValueTree& vt, te::Edit& edit, juce::UndoManager* undoManager)
{
    if (! vt.isValid())
        return;

    te::EditItemID::remapIDs (vt, undoManager, edit, nullptr);
}

bool convertTimeValueToSeconds (te::Edit& edit, CommandTimeUnit unit, double value, double& outSeconds, juce::String& error)
{
    if (value < 0.0)
    {
        error = "time value must be greater than or equal to zero";
        return false;
    }

    if (unit == CommandTimeUnit::seconds)
    {
        outSeconds = value;
        return true;
    }

    outSeconds = edit.tempoSequence.toTime (te::BeatPosition::fromBeats (value)).inSeconds();
    return true;
}

bool convertDurationValueToSeconds (te::Edit& edit,
                                    CommandTimeUnit unit,
                                    double atStartSeconds,
                                    double durationValue,
                                    double& outSeconds,
                                    juce::String& error)
{
    if (durationValue <= 0.0)
    {
        error = "duration must be greater than zero";
        return false;
    }

    if (unit == CommandTimeUnit::seconds)
    {
        outSeconds = durationValue;
        return true;
    }

    const auto startTime = te::TimePosition::fromSeconds (atStartSeconds);
    const auto startBeat = edit.tempoSequence.toBeats (startTime);
    const auto endBeat = startBeat + te::BeatDuration::fromBeats (durationValue);
    const auto endTime = edit.tempoSequence.toTime (endBeat);
    outSeconds = (endTime - startTime).inSeconds();

    if (outSeconds <= 0.0)
    {
        error = "duration conversion produced non-positive length";
        return false;
    }

    return true;
}

juce::String fadeCurveToString (te::AudioFadeCurve::Type type)
{
    switch (type)
    {
        case te::AudioFadeCurve::convex:  return "convex";
        case te::AudioFadeCurve::concave: return "concave";
        case te::AudioFadeCurve::sCurve:  return "s_curve";
        case te::AudioFadeCurve::linear:  break;
    }

    return "linear";
}

bool parseFadeCurve (const juce::String& rawValue, te::AudioFadeCurve::Type& out)
{
    const auto raw = rawValue.trim().toLowerCase().replaceCharacter ('-', '_');

    if (raw.isEmpty())
        return false;

    if (raw == "linear" || raw == "lin")
    {
        out = te::AudioFadeCurve::linear;
        return true;
    }

    if (raw == "convex" || raw == "equal_power" || raw == "eq_power" || raw == "power")
    {
        out = te::AudioFadeCurve::convex;
        return true;
    }

    if (raw == "concave")
    {
        out = te::AudioFadeCurve::concave;
        return true;
    }

    if (raw == "s_curve" || raw == "scurve" || raw == "s")
    {
        out = te::AudioFadeCurve::sCurve;
        return true;
    }

    return false;
}

juce::String fadeBehaviourToString (te::AudioClipBase::FadeBehaviour behaviour)
{
    switch (behaviour)
    {
        case te::AudioClipBase::speedRamp: return "speed";
        case te::AudioClipBase::gainFade:  break;
    }

    return "gain";
}

bool parseFadeBehaviour (const juce::String& rawValue, te::AudioClipBase::FadeBehaviour& out)
{
    const auto raw = rawValue.trim().toLowerCase().replaceCharacter ('-', '_');

    if (raw.isEmpty())
        return false;

    if (raw == "gain" || raw == "gain_fade" || raw == "volume" || raw == "volume_fade")
    {
        out = te::AudioClipBase::gainFade;
        return true;
    }

    if (raw == "speed" || raw == "speed_ramp" || raw == "tape" || raw == "tape_stop" || raw == "tape_start")
    {
        out = te::AudioClipBase::speedRamp;
        return true;
    }

    return false;
}

juce::String clipTrackIdOrEmpty (const te::Clip& clip)
{
    if (auto* track = clip.getClipTrack())
        return track->itemID.toString();

    return {};
}

juce::var createClipFadeState (const te::Clip& clip, const te::AudioClipBase& audioClip)
{
    auto row = std::make_unique<juce::DynamicObject>();
    row->setProperty ("clip_id", clip.itemID.toString());
    row->setProperty ("track_id", clipTrackIdOrEmpty (clip));
    row->setProperty ("fade_in_seconds", audioClip.getFadeIn().inSeconds());
    row->setProperty ("fade_out_seconds", audioClip.getFadeOut().inSeconds());
    row->setProperty ("fade_in_curve", fadeCurveToString (audioClip.getFadeInType()));
    row->setProperty ("fade_out_curve", fadeCurveToString (audioClip.getFadeOutType()));
    row->setProperty ("fade_in_curve_type", static_cast<int> (audioClip.getFadeInType()));
    row->setProperty ("fade_out_curve_type", static_cast<int> (audioClip.getFadeOutType()));
    row->setProperty ("fade_in_behaviour", fadeBehaviourToString (audioClip.getFadeInBehaviour()));
    row->setProperty ("fade_out_behaviour", fadeBehaviourToString (audioClip.getFadeOutBehaviour()));
    row->setProperty ("fade_in_behaviour_type", static_cast<int> (audioClip.getFadeInBehaviour()));
    row->setProperty ("fade_out_behaviour_type", static_cast<int> (audioClip.getFadeOutBehaviour()));
    row->setProperty ("auto_crossfade", audioClip.getAutoCrossfade());
    return juce::var (row.release());
}

juce::var createClipGainState (const te::Clip& clip, const te::AudioClipBase& audioClip)
{
    auto row = std::make_unique<juce::DynamicObject>();
    row->setProperty ("clip_id", clip.itemID.toString());
    row->setProperty ("track_id", clipTrackIdOrEmpty (clip));
    row->setProperty ("gain_db", audioClip.getGainDB());
    row->setProperty ("clip_gain_db", audioClip.getGainDB());
    row->setProperty ("pan", audioClip.getPan());
    row->setProperty ("clip_pan", audioClip.getPan());
    row->setProperty ("mute", audioClip.isMuted());
    row->setProperty ("clip_mute", audioClip.isMuted());
    return juce::var (row.release());
}

void appendClipFadeState (juce::DynamicObject& target, const te::AudioClipBase& audioClip)
{
    target.setProperty ("fade_in_seconds", audioClip.getFadeIn().inSeconds());
    target.setProperty ("fade_out_seconds", audioClip.getFadeOut().inSeconds());
    target.setProperty ("fade_in_curve", fadeCurveToString (audioClip.getFadeInType()));
    target.setProperty ("fade_out_curve", fadeCurveToString (audioClip.getFadeOutType()));
    target.setProperty ("fade_in_curve_type", static_cast<int> (audioClip.getFadeInType()));
    target.setProperty ("fade_out_curve_type", static_cast<int> (audioClip.getFadeOutType()));
    target.setProperty ("fade_in_behaviour", fadeBehaviourToString (audioClip.getFadeInBehaviour()));
    target.setProperty ("fade_out_behaviour", fadeBehaviourToString (audioClip.getFadeOutBehaviour()));
    target.setProperty ("fade_in_behaviour_type", static_cast<int> (audioClip.getFadeInBehaviour()));
    target.setProperty ("fade_out_behaviour_type", static_cast<int> (audioClip.getFadeOutBehaviour()));
    target.setProperty ("auto_crossfade", audioClip.getAutoCrossfade());
}

void appendClipGainState (juce::DynamicObject& target, const te::AudioClipBase& audioClip)
{
    target.setProperty ("gain_db", audioClip.getGainDB());
    target.setProperty ("clip_gain_db", audioClip.getGainDB());
    target.setProperty ("pan", audioClip.getPan());
    target.setProperty ("clip_pan", audioClip.getPan());
    target.setProperty ("mute", audioClip.isMuted());
    target.setProperty ("clip_mute", audioClip.isMuted());
}

struct StripSilenceParams
{
    double thresholdDbfs = kDefaultStripThresholdDbfs;
    double minimumSilenceSeconds = kDefaultStripMinimumSilenceSeconds;
    double clipStartPadSeconds = kDefaultStripStartPadSeconds;
    double clipEndPadSeconds = kDefaultStripEndPadSeconds;
    double frameSeconds = kDefaultStripFrameSeconds;
    juce::String scope = "selected_ranges_or_clip";
};

struct StripSilenceRange
{
    juce::String rangeId;
    double startSeconds = 0.0;
    double endSeconds = 0.0;
};

struct StripSilenceRegion
{
    juce::String rangeId;
    juce::String clipId;
    juce::String trackId;
    double startSeconds = 0.0;
    double endSeconds = 0.0;
    double sourceStartSeconds = 0.0;
    double sourceEndSeconds = 0.0;
};

struct StripSilenceAnalysisResult
{
    juce::String analysisId;
    juce::String clipId;
    juce::String trackId;
    juce::String sourcePath;
    double clipStartSeconds = 0.0;
    double clipEndSeconds = 0.0;
    double sourceOffsetSeconds = 0.0;
    double sourceSampleRate = 0.0;
    int sourceChannels = 0;
    StripSilenceParams params;
    std::vector<StripSilenceRange> analysisRanges;
    std::vector<StripSilenceRegion> keepSegments;
    std::vector<StripSilenceRegion> stripRegions;
    bool wouldRemoveEntireClip = false;
    juce::String reason;
};

bool readSecondsOrMsProperty (const juce::DynamicObject& object,
                              const juce::StringArray& secondsKeys,
                              const juce::StringArray& msKeys,
                              double& outSeconds)
{
    double value = 0.0;
    if (readNumericProperty (object, secondsKeys, value))
    {
        outSeconds = value;
        return true;
    }

    if (readNumericProperty (object, msKeys, value))
    {
        outSeconds = value / 1000.0;
        return true;
    }

    return false;
}

StripSilenceParams parseStripSilenceParams (const juce::DynamicObject& object)
{
    StripSilenceParams params;

    readNumericProperty (object, { "threshold_dbfs", "threshold_db", "strip_threshold_dbfs", "strip_threshold_db" }, params.thresholdDbfs);
    readSecondsOrMsProperty (object,
                             { "min_silence_seconds", "minimum_silence_seconds", "min_strip_duration_seconds", "minimum_strip_duration_seconds" },
                             { "min_silence_ms", "minimum_silence_ms", "min_strip_duration_ms", "minimum_strip_duration_ms" },
                             params.minimumSilenceSeconds);
    readSecondsOrMsProperty (object,
                             { "clip_start_pad_seconds", "start_pad_seconds", "clip_start_pad" },
                             { "clip_start_pad_ms", "start_pad_ms" },
                             params.clipStartPadSeconds);
    readSecondsOrMsProperty (object,
                             { "clip_end_pad_seconds", "end_pad_seconds", "clip_end_pad" },
                             { "clip_end_pad_ms", "end_pad_ms" },
                             params.clipEndPadSeconds);
    readSecondsOrMsProperty (object,
                             { "frame_seconds", "analysis_frame_seconds" },
                             { "frame_ms", "analysis_frame_ms" },
                             params.frameSeconds);

    const auto scope = object.getProperty ("scope").toString().trim();
    if (scope.isNotEmpty())
        params.scope = scope;

    params.thresholdDbfs = juce::jlimit (-120.0, 0.0, params.thresholdDbfs);
    params.minimumSilenceSeconds = juce::jlimit (0.01, 10.0, params.minimumSilenceSeconds);
    params.clipStartPadSeconds = juce::jlimit (0.0, 5.0, params.clipStartPadSeconds);
    params.clipEndPadSeconds = juce::jlimit (0.0, 5.0, params.clipEndPadSeconds);
    params.frameSeconds = juce::jlimit (0.0025, 0.100, params.frameSeconds);

    return params;
}

bool readRangeSecondsFromObject (const juce::DynamicObject& rangeObject,
                                 const te::Clip& clip,
                                 StripSilenceRange& out)
{
    const auto clipStartSeconds = clip.getPosition().getStart().inSeconds();
    const auto clipEndSeconds = clip.getPosition().getEnd().inSeconds();
    double startSeconds = clipStartSeconds;
    double endSeconds = clipEndSeconds;
    const auto rangeClipId = rangeObject.getProperty ("clip_id").toString().trim();

    if (rangeClipId.isNotEmpty() && rangeClipId != clip.itemID.toString())
        return false;

    if (readNumericProperty (rangeObject, { "start_seconds", "range_start_seconds", "timeline_start_seconds" }, startSeconds)
        && readNumericProperty (rangeObject, { "end_seconds", "range_end_seconds", "timeline_end_seconds" }, endSeconds))
    {
    }
    else
    {
        double localStartSeconds = 0.0;
        double localEndSeconds = 0.0;
        if (! readNumericProperty (rangeObject, { "clip_local_start_seconds", "local_start_seconds", "start_local_seconds" }, localStartSeconds)
            || ! readNumericProperty (rangeObject, { "clip_local_end_seconds", "local_end_seconds", "end_local_seconds" }, localEndSeconds))
        {
            return false;
        }

        startSeconds = clipStartSeconds + localStartSeconds;
        endSeconds = clipStartSeconds + localEndSeconds;
    }

    if (endSeconds < startSeconds)
        std::swap (startSeconds, endSeconds);

    startSeconds = juce::jlimit (clipStartSeconds, clipEndSeconds, startSeconds);
    endSeconds = juce::jlimit (clipStartSeconds, clipEndSeconds, endSeconds);

    if (endSeconds - startSeconds < kMinimumSurvivingClipLengthSeconds)
        return false;

    out.rangeId = rangeObject.getProperty ("range_id").toString().trim();
    out.startSeconds = startSeconds;
    out.endSeconds = endSeconds;
    return true;
}

void appendRangesFromVar (const juce::var& rangesVar,
                          const te::Clip& clip,
                          std::vector<StripSilenceRange>& ranges)
{
    if (auto* array = rangesVar.getArray())
    {
        for (const auto& item : *array)
        {
            if (auto* rangeObject = item.getDynamicObject())
            {
                StripSilenceRange range;
                if (readRangeSecondsFromObject (*rangeObject, clip, range))
                    ranges.push_back (range);
            }
        }
    }
}

std::vector<StripSilenceRange> parseStripSilenceRanges (const juce::DynamicObject& object, const te::Clip& clip)
{
    std::vector<StripSilenceRange> ranges;
    appendRangesFromVar (object.getProperty ("ranges"), clip, ranges);
    appendRangesFromVar (object.getProperty ("selected_clip_ranges"), clip, ranges);

    if (auto* rangeObject = object.getProperty ("range").getDynamicObject())
    {
        StripSilenceRange range;
        if (readRangeSecondsFromObject (*rangeObject, clip, range))
            ranges.push_back (range);
    }

    StripSilenceRange explicitRange;
    if (readRangeSecondsFromObject (object, clip, explicitRange))
        ranges.push_back (explicitRange);

    if (ranges.empty())
    {
        StripSilenceRange wholeClip;
        wholeClip.rangeId = "whole_clip";
        wholeClip.startSeconds = clip.getPosition().getStart().inSeconds();
        wholeClip.endSeconds = clip.getPosition().getEnd().inSeconds();
        ranges.push_back (wholeClip);
    }

    std::sort (ranges.begin(), ranges.end(), [] (const auto& a, const auto& b)
    {
        if (a.startSeconds == b.startSeconds)
            return a.endSeconds < b.endSeconds;
        return a.startSeconds < b.startSeconds;
    });

    std::vector<StripSilenceRange> merged;
    for (const auto& range : ranges)
    {
        if (merged.empty() || range.startSeconds > merged.back().endSeconds + kClipEditEpsilonSeconds)
        {
            merged.push_back (range);
            continue;
        }

        merged.back().endSeconds = juce::jmax (merged.back().endSeconds, range.endSeconds);
        if (merged.back().rangeId.isEmpty())
            merged.back().rangeId = range.rangeId;
    }

    return merged;
}

juce::File sourceFileForAudioClip (const te::AudioClipBase& audioClip)
{
    auto sourceFile = audioClip.getCurrentSourceFile();
    if (! sourceFile.existsAsFile())
        sourceFile = audioClip.getOriginalFile();
    return sourceFile;
}

StripSilenceRegion makeStripRegion (const juce::String& clipId,
                                    const juce::String& trackId,
                                    const juce::String& rangeId,
                                    double timelineStartSeconds,
                                    double timelineEndSeconds,
                                    double clipStartSeconds,
                                    double sourceOffsetSeconds)
{
    StripSilenceRegion region;
    region.clipId = clipId;
    region.trackId = trackId;
    region.rangeId = rangeId;
    region.startSeconds = timelineStartSeconds;
    region.endSeconds = timelineEndSeconds;
    region.sourceStartSeconds = sourceOffsetSeconds + (timelineStartSeconds - clipStartSeconds);
    region.sourceEndSeconds = sourceOffsetSeconds + (timelineEndSeconds - clipStartSeconds);
    return region;
}

juce::var stripRangeToVar (const StripSilenceRange& range)
{
    auto row = std::make_unique<juce::DynamicObject>();
    row->setProperty ("range_id", range.rangeId);
    row->setProperty ("start_seconds", range.startSeconds);
    row->setProperty ("end_seconds", range.endSeconds);
    row->setProperty ("duration_seconds", range.endSeconds - range.startSeconds);
    return juce::var (row.release());
}

juce::var stripRegionToVar (const StripSilenceRegion& region)
{
    auto row = std::make_unique<juce::DynamicObject>();
    row->setProperty ("range_id", region.rangeId);
    row->setProperty ("clip_id", region.clipId);
    row->setProperty ("track_id", region.trackId);
    row->setProperty ("start_seconds", region.startSeconds);
    row->setProperty ("end_seconds", region.endSeconds);
    row->setProperty ("duration_seconds", region.endSeconds - region.startSeconds);
    row->setProperty ("source_start_seconds", region.sourceStartSeconds);
    row->setProperty ("source_end_seconds", region.sourceEndSeconds);
    return juce::var (row.release());
}

juce::var stripParamsToVar (const StripSilenceParams& params)
{
    auto row = std::make_unique<juce::DynamicObject>();
    row->setProperty ("threshold_dbfs", params.thresholdDbfs);
    row->setProperty ("min_silence_seconds", params.minimumSilenceSeconds);
    row->setProperty ("min_silence_ms", params.minimumSilenceSeconds * 1000.0);
    row->setProperty ("clip_start_pad_seconds", params.clipStartPadSeconds);
    row->setProperty ("clip_start_pad_ms", params.clipStartPadSeconds * 1000.0);
    row->setProperty ("clip_end_pad_seconds", params.clipEndPadSeconds);
    row->setProperty ("clip_end_pad_ms", params.clipEndPadSeconds * 1000.0);
    row->setProperty ("frame_seconds", params.frameSeconds);
    row->setProperty ("scope", params.scope);
    return juce::var (row.release());
}

juce::var stripRegionsToVar (const std::vector<StripSilenceRegion>& regions)
{
    juce::Array<juce::var> out;
    for (const auto& region : regions)
        out.add (stripRegionToVar (region));
    return juce::var (out);
}

juce::var stripRangesToVar (const std::vector<StripSilenceRange>& ranges)
{
    juce::Array<juce::var> out;
    for (const auto& range : ranges)
        out.add (stripRangeToVar (range));
    return juce::var (out);
}

juce::var stripActionsToVar (const StripSilenceAnalysisResult& analysis)
{
    juce::Array<juce::var> out;

    for (const auto& region : analysis.stripRegions)
    {
        auto action = std::make_unique<juce::DynamicObject>();
        action->setProperty ("action", "delete_region");
        action->setProperty ("cmd", "clip.strip_silence.apply");
        action->setProperty ("command", "clip.strip_silence.apply");
        action->setProperty ("analysis_id", analysis.analysisId);
        action->setProperty ("clip_id", region.clipId);
        action->setProperty ("track_id", region.trackId);
        action->setProperty ("range_id", region.rangeId);
        action->setProperty ("start_seconds", region.startSeconds);
        action->setProperty ("end_seconds", region.endSeconds);
        action->setProperty ("duration_seconds", region.endSeconds - region.startSeconds);
        out.add (juce::var (action.release()));
    }

    return juce::var (out);
}

juce::var stripAnalysisToVar (const StripSilenceAnalysisResult& analysis)
{
    auto response = std::make_unique<juce::DynamicObject>();
    response->setProperty ("status", "ok");
    response->setProperty ("message", "Strip silence analysis ready");
    response->setProperty ("analysis_id", analysis.analysisId);
    response->setProperty ("clip_id", analysis.clipId);
    response->setProperty ("track_id", analysis.trackId);
    response->setProperty ("source_path", analysis.sourcePath);
    response->setProperty ("clip_start_seconds", analysis.clipStartSeconds);
    response->setProperty ("clip_end_seconds", analysis.clipEndSeconds);
    response->setProperty ("clip_length_seconds", analysis.clipEndSeconds - analysis.clipStartSeconds);
    response->setProperty ("offset_in_source_seconds", analysis.sourceOffsetSeconds);
    response->setProperty ("source_sample_rate", analysis.sourceSampleRate);
    response->setProperty ("source_channels", analysis.sourceChannels);
    response->setProperty ("threshold_dbfs", analysis.params.thresholdDbfs);
    response->setProperty ("min_silence_seconds", analysis.params.minimumSilenceSeconds);
    response->setProperty ("clip_start_pad_seconds", analysis.params.clipStartPadSeconds);
    response->setProperty ("clip_end_pad_seconds", analysis.params.clipEndPadSeconds);
    response->setProperty ("params", stripParamsToVar (analysis.params));
    response->setProperty ("analysis_ranges", stripRangesToVar (analysis.analysisRanges));
    response->setProperty ("keep_segments", stripRegionsToVar (analysis.keepSegments));
    response->setProperty ("strip_regions", stripRegionsToVar (analysis.stripRegions));
    response->setProperty ("actions", stripActionsToVar (analysis));
    response->setProperty ("strip_region_count", (int) analysis.stripRegions.size());
    response->setProperty ("keep_segment_count", (int) analysis.keepSegments.size());
    response->setProperty ("would_remove_entire_clip", analysis.wouldRemoveEntireClip);
    response->setProperty ("requires_confirmation", ! analysis.stripRegions.empty());
    if (analysis.reason.isNotEmpty())
        response->setProperty ("reason", analysis.reason);
    return juce::var (response.release());
}

bool appendStripRegionFromObject (const juce::DynamicObject& object,
                                  const te::Clip& clip,
                                  const juce::String& defaultTrackId,
                                  std::vector<StripSilenceRegion>& regions)
{
    const auto clipId = clip.itemID.toString();
    const auto regionClipId = object.getProperty ("clip_id").toString().trim();
    if (regionClipId.isNotEmpty() && regionClipId != clipId)
        return false;

    double startSeconds = 0.0;
    double endSeconds = 0.0;
    if (! readNumericProperty (object, { "start_seconds", "range_start_seconds", "timeline_start_seconds" }, startSeconds)
        || ! readNumericProperty (object, { "end_seconds", "range_end_seconds", "timeline_end_seconds" }, endSeconds))
    {
        return false;
    }

    if (endSeconds < startSeconds)
        std::swap (startSeconds, endSeconds);

    const auto clipStartSeconds = clip.getPosition().getStart().inSeconds();
    const auto clipEndSeconds = clip.getPosition().getEnd().inSeconds();
    startSeconds = juce::jlimit (clipStartSeconds, clipEndSeconds, startSeconds);
    endSeconds = juce::jlimit (clipStartSeconds, clipEndSeconds, endSeconds);

    if (endSeconds - startSeconds < kMinimumStripRegionSeconds)
        return false;

    StripSilenceRegion region = makeStripRegion (clipId,
                                                 defaultTrackId,
                                                 object.getProperty ("range_id").toString().trim(),
                                                 startSeconds,
                                                 endSeconds,
                                                 clipStartSeconds,
                                                 clip.getPosition().getOffset().inSeconds());
    regions.push_back (region);
    return true;
}

bool appendStripRegionFromVar (const juce::var& value,
                               const te::Clip& clip,
                               const juce::String& defaultTrackId,
                               std::vector<StripSilenceRegion>& regions)
{
    if (auto* object = value.getDynamicObject())
        return appendStripRegionFromObject (*object, clip, defaultTrackId, regions);

    return false;
}

std::vector<StripSilenceRegion> parseStripRegionsForApply (const juce::DynamicObject& object,
                                                           const te::Clip& clip,
                                                           const juce::String& trackId)
{
    std::vector<StripSilenceRegion> regions;

    auto appendArray = [&] (const juce::var& value)
    {
        if (auto* array = value.getArray())
            for (const auto& item : *array)
                appendStripRegionFromVar (item, clip, trackId, regions);
    };

    appendArray (object.getProperty ("strip_regions"));

    if (auto* analysisObject = object.getProperty ("analysis").getDynamicObject())
        appendArray (analysisObject->getProperty ("strip_regions"));

    if (regions.empty())
        appendStripRegionFromObject (object, clip, trackId, regions);

    std::sort (regions.begin(), regions.end(), [] (const auto& a, const auto& b)
    {
        if (a.startSeconds == b.startSeconds)
            return a.endSeconds > b.endSeconds;
        return a.startSeconds > b.startSeconds;
    });

    return regions;
}

bool analyseStripSilenceForClip (te::Edit& edit,
                                 te::Clip& clip,
                                 te::AudioClipBase& audioClip,
                                 const juce::DynamicObject& object,
                                 StripSilenceAnalysisResult& out,
                                 juce::String& error)
{
    const auto clipId = clip.itemID.toString();
    const auto trackId = clipTrackIdOrEmpty (clip);
    const auto sourceFile = sourceFileForAudioClip (audioClip);

    if (! sourceFile.existsAsFile())
    {
        error = "clip.strip_silence.analyze source file not found for clip_id: " + clipId;
        return false;
    }

    std::unique_ptr<juce::AudioFormatReader> reader (
        edit.engine.getAudioFileFormatManager().readFormatManager.createReaderFor (sourceFile));

    if (reader == nullptr || reader->sampleRate <= 0.0 || reader->lengthInSamples <= 0)
    {
        error = "clip.strip_silence.analyze failed to read source file: " + sourceFile.getFullPathName();
        return false;
    }

    const auto params = parseStripSilenceParams (object);
    const auto ranges = parseStripSilenceRanges (object, clip);
    const auto clipStartSeconds = clip.getPosition().getStart().inSeconds();
    const auto clipEndSeconds = clip.getPosition().getEnd().inSeconds();
    const auto sourceOffsetSeconds = clip.getPosition().getOffset().inSeconds();
    const double thresholdLinear = std::pow (10.0, params.thresholdDbfs / 20.0);
    const int channelCount = juce::jlimit (1, 2, (int) reader->numChannels);
    const int frameSamples = juce::jmax (64, (int) std::llround (params.frameSeconds * reader->sampleRate));

    out.analysisId = "strip_silence_" + clipId + "_" + juce::String (juce::Time::getMillisecondCounterHiRes(), 0);
    out.clipId = clipId;
    out.trackId = trackId;
    out.sourcePath = sourceFile.getFullPathName();
    out.clipStartSeconds = clipStartSeconds;
    out.clipEndSeconds = clipEndSeconds;
    out.sourceOffsetSeconds = sourceOffsetSeconds;
    out.sourceSampleRate = reader->sampleRate;
    out.sourceChannels = (int) reader->numChannels;
    out.params = params;
    out.analysisRanges = ranges;

    juce::AudioBuffer<float> buffer (channelCount, frameSamples);

    for (const auto& analysisRange : ranges)
    {
        const double sourceStartSeconds = sourceOffsetSeconds + (analysisRange.startSeconds - clipStartSeconds);
        const double sourceEndSeconds = sourceOffsetSeconds + (analysisRange.endSeconds - clipStartSeconds);
        const int64_t sourceStartSample = juce::jlimit<int64_t> (0, reader->lengthInSamples, (int64_t) std::floor (sourceStartSeconds * reader->sampleRate));
        const int64_t sourceEndSample = juce::jlimit<int64_t> (0, reader->lengthInSamples, (int64_t) std::ceil (sourceEndSeconds * reader->sampleRate));

        if (sourceEndSample <= sourceStartSample)
            continue;

        std::vector<StripSilenceRegion> activeSegments;
        bool activeOpen = false;
        double activeStartSeconds = analysisRange.startSeconds;

        for (int64_t pos = sourceStartSample; pos < sourceEndSample;)
        {
            const int samplesThisFrame = (int) juce::jmin<int64_t> (frameSamples, sourceEndSample - pos);
            buffer.clear();
            reader->read (&buffer, 0, samplesThisFrame, pos, true, true);

            double peak = 0.0;
            double sumSquares = 0.0;
            int sampleCount = 0;

            for (int ch = 0; ch < channelCount; ++ch)
            {
                const auto* data = buffer.getReadPointer (ch);
                for (int i = 0; i < samplesThisFrame; ++i)
                {
                    const double value = std::abs ((double) data[i]);
                    peak = juce::jmax (peak, value);
                    sumSquares += value * value;
                    ++sampleCount;
                }
            }

            const double rms = sampleCount > 0 ? std::sqrt (sumSquares / (double) sampleCount) : 0.0;
            const bool isActive = peak >= thresholdLinear || rms >= thresholdLinear * 0.5;
            const double frameStartSeconds = analysisRange.startSeconds + ((double) (pos - sourceStartSample) / reader->sampleRate);
            const double frameEndSeconds = juce::jmin (analysisRange.endSeconds,
                                                       analysisRange.startSeconds + ((double) (pos + samplesThisFrame - sourceStartSample) / reader->sampleRate));

            if (isActive && ! activeOpen)
            {
                activeOpen = true;
                activeStartSeconds = frameStartSeconds;
            }
            else if (! isActive && activeOpen)
            {
                activeOpen = false;
                activeSegments.push_back (makeStripRegion (clipId,
                                                           trackId,
                                                           analysisRange.rangeId,
                                                           activeStartSeconds,
                                                           frameStartSeconds,
                                                           clipStartSeconds,
                                                           sourceOffsetSeconds));
            }

            pos += samplesThisFrame;

            if (pos >= sourceEndSample && activeOpen)
            {
                activeOpen = false;
                activeSegments.push_back (makeStripRegion (clipId,
                                                           trackId,
                                                           analysisRange.rangeId,
                                                           activeStartSeconds,
                                                           frameEndSeconds,
                                                           clipStartSeconds,
                                                           sourceOffsetSeconds));
            }
        }

        std::vector<StripSilenceRegion> paddedKeeps;
        for (const auto& active : activeSegments)
        {
            auto keep = active;
            keep.startSeconds = juce::jlimit (analysisRange.startSeconds,
                                              analysisRange.endSeconds,
                                              active.startSeconds - params.clipStartPadSeconds);
            keep.endSeconds = juce::jlimit (analysisRange.startSeconds,
                                            analysisRange.endSeconds,
                                            active.endSeconds + params.clipEndPadSeconds);
            keep.sourceStartSeconds = sourceOffsetSeconds + (keep.startSeconds - clipStartSeconds);
            keep.sourceEndSeconds = sourceOffsetSeconds + (keep.endSeconds - clipStartSeconds);

            if (keep.endSeconds - keep.startSeconds >= kMinimumSurvivingClipLengthSeconds)
                paddedKeeps.push_back (keep);
        }

        std::sort (paddedKeeps.begin(), paddedKeeps.end(), [] (const auto& a, const auto& b)
        {
            return a.startSeconds < b.startSeconds;
        });

        std::vector<StripSilenceRegion> mergedKeeps;
        for (const auto& keep : paddedKeeps)
        {
            if (mergedKeeps.empty() || keep.startSeconds > mergedKeeps.back().endSeconds + kClipEditEpsilonSeconds)
            {
                mergedKeeps.push_back (keep);
                continue;
            }

            mergedKeeps.back().endSeconds = juce::jmax (mergedKeeps.back().endSeconds, keep.endSeconds);
            mergedKeeps.back().sourceEndSeconds = sourceOffsetSeconds + (mergedKeeps.back().endSeconds - clipStartSeconds);
        }

        for (const auto& keep : mergedKeeps)
            out.keepSegments.push_back (keep);

        double cursor = analysisRange.startSeconds;
        for (const auto& keep : mergedKeeps)
        {
            if (keep.startSeconds - cursor >= params.minimumSilenceSeconds)
            {
                out.stripRegions.push_back (makeStripRegion (clipId,
                                                             trackId,
                                                             analysisRange.rangeId,
                                                             cursor,
                                                             keep.startSeconds,
                                                             clipStartSeconds,
                                                             sourceOffsetSeconds));
            }

            cursor = juce::jmax (cursor, keep.endSeconds);
        }

        if (analysisRange.endSeconds - cursor >= params.minimumSilenceSeconds)
        {
            out.stripRegions.push_back (makeStripRegion (clipId,
                                                         trackId,
                                                         analysisRange.rangeId,
                                                         cursor,
                                                         analysisRange.endSeconds,
                                                         clipStartSeconds,
                                                         sourceOffsetSeconds));
        }
    }

    double totalStripSeconds = 0.0;
    for (const auto& region : out.stripRegions)
        totalStripSeconds += juce::jmax (0.0, region.endSeconds - region.startSeconds);

    const auto clipLengthSeconds = clipEndSeconds - clipStartSeconds;
    out.wouldRemoveEntireClip = clipLengthSeconds > 0.0
                                && totalStripSeconds >= clipLengthSeconds - kClipEditEpsilonSeconds
                                && out.keepSegments.empty();

    if (out.stripRegions.empty())
        out.reason = out.keepSegments.empty() ? "no_audio_above_threshold" : "no_silence_regions_above_minimum_duration";

    return true;
}

struct OverlapEditResult
{
    juce::StringArray touchedOriginalClipIds;
    juce::StringArray createdClipIds;
    juce::StringArray removedClipIds;
};

juce::var stringArrayToVar (const juce::StringArray& values)
{
    juce::Array<juce::var> out;
    for (const auto& value : values)
        out.add (value);
    return juce::var (out);
}

bool trackContainsClipId (te::ClipTrack& track, const juce::String& clipId)
{
    for (auto* clip : track.getClips())
        if (clip != nullptr && clip->itemID.toString() == clipId)
            return true;

    return false;
}

void applyMicroFadeIfNeeded (te::Clip& clip)
{
    if (auto* audioClip = dynamic_cast<te::AudioClipBase*> (&clip))
    {
        const auto clipLengthSeconds = clip.getPosition().getLength().inSeconds();
        const auto fadeSeconds = juce::jlimit (0.0, 0.005, juce::jmin (0.003, clipLengthSeconds * 0.5));

        if (fadeSeconds > 0.0)
        {
            const auto fadeDuration = te::TimeDuration::fromSeconds (fadeSeconds);
            audioClip->setFadeIn (fadeDuration);
            audioClip->setFadeOut (fadeDuration);
        }
    }
}

void restoreSplitBoundaryFades (te::Clip& leftClip,
                                te::Clip& rightClip,
                                double originalFadeInSeconds,
                                double originalFadeOutSeconds)
{
    auto* leftAudio = dynamic_cast<te::AudioClipBase*> (&leftClip);
    auto* rightAudio = dynamic_cast<te::AudioClipBase*> (&rightClip);

    if (leftAudio == nullptr || rightAudio == nullptr)
        return;

    const auto leftLengthSeconds = leftClip.getPosition().getLength().inSeconds();
    const auto rightLengthSeconds = rightClip.getPosition().getLength().inSeconds();

    leftAudio->setFadeIn (te::TimeDuration::fromSeconds (juce::jlimit (0.0, leftLengthSeconds, originalFadeInSeconds)));
    leftAudio->setFadeOut (te::TimeDuration::fromSeconds (0.0));
    rightAudio->setFadeIn (te::TimeDuration::fromSeconds (0.0));
    rightAudio->setFadeOut (te::TimeDuration::fromSeconds (juce::jlimit (0.0, rightLengthSeconds, originalFadeOutSeconds)));
}

bool isBelowMinimumSurvivingClipLength (const te::Clip& clip)
{
    return clip.getPosition().getLength().inSeconds() < (kMinimumSurvivingClipLengthSeconds - kClipEditEpsilonSeconds);
}

void pruneTinyEditedClips (te::ClipTrack& track, OverlapEditResult* result)
{
    te::Clip::Array clipsToRemove;

    for (auto* clip : track.getClips())
    {
        if (clip == nullptr)
            continue;

        const auto clipId = clip->itemID.toString();
        const auto wasTouched = result != nullptr && result->touchedOriginalClipIds.contains (clipId);
        const auto wasCreated = result != nullptr && result->createdClipIds.contains (clipId);

        if (! wasTouched && ! wasCreated)
            continue;

        if (isBelowMinimumSurvivingClipLength (*clip))
            clipsToRemove.add (clip);
    }

    for (int i = clipsToRemove.size(); --i >= 0;)
    {
        auto clip = clipsToRemove.getUnchecked (i);
        if (clip != nullptr)
        {
            const auto clipId = clip->itemID.toString();
            const auto wasCreated = result != nullptr && result->createdClipIds.contains (clipId);

            clip->removeFromParent();

            if (result != nullptr)
            {
                if (wasCreated)
                    result->createdClipIds.removeString (clipId);
                else
                    result->removedClipIds.addIfNotAlreadyThere (clipId);
            }
        }
    }
}

bool applyCutOverlapPolicy (te::ClipTrack& track, te::Clip& movedOrResizedClip, bool allowSplit, OverlapEditResult* result = nullptr)
{
    const auto targetRange = movedOrResizedClip.getPosition().time;
    te::Clip::Array victims;

    for (auto* clip : track.getClips())
    {
        if (clip == nullptr || clip == &movedOrResizedClip)
            continue;

        if (clip->getPosition().time.overlaps (targetRange))
            victims.add (clip);
    }

    for (int i = victims.size(); --i >= 0;)
    {
        auto victim = victims.getUnchecked (i);

        if (victim == nullptr)
            continue;

        const auto victimId = victim->itemID.toString();

        if (result != nullptr)
            result->touchedOriginalClipIds.addIfNotAlreadyThere (victimId);

        if (allowSplit)
        {
            const auto newClips = te::deleteRegion (*victim, targetRange);

            if (result != nullptr)
            {
                for (int newClipIndex = 0; newClipIndex < newClips.size(); ++newClipIndex)
                    if (auto* newClip = newClips.getUnchecked (newClipIndex))
                        result->createdClipIds.addIfNotAlreadyThere (newClip->itemID.toString());
            }
        }
        else
        {
            victim->trimAwayOverlap (targetRange);
        }
    }

    pruneTinyEditedClips (track, result);

    if (result != nullptr)
    {
        for (const auto& victimId : result->touchedOriginalClipIds)
            if (! trackContainsClipId (track, victimId))
                result->removedClipIds.addIfNotAlreadyThere (victimId);

        for (auto* clip : track.getClips())
        {
            if (clip == nullptr)
                continue;

            const auto clipId = clip->itemID.toString();

            if (result->touchedOriginalClipIds.contains (clipId)
                || result->createdClipIds.contains (clipId))
            {
                applyMicroFadeIfNeeded (*clip);
            }
        }
    }

    return true;
}

OverlapPolicy parseOverlapPolicy (const juce::DynamicObject& object)
{
    auto raw = object.getProperty ("overlap_policy").toString().trim().toLowerCase();

    if (raw.isEmpty())
        raw = object.getProperty ("overlap_mode").toString().trim().toLowerCase();

    if (raw.isEmpty() || raw == "cut" || raw == "trim")
        return OverlapPolicy::trim;

    if (raw == "layer" || raw == "stack")
        return OverlapPolicy::layer;

    if (raw == "crossfade" || raw == "xfade")
        return OverlapPolicy::crossfade;

    return OverlapPolicy::trim;
}

juce::String overlapPolicyToString (OverlapPolicy policy)
{
    switch (policy)
    {
        case OverlapPolicy::layer:     return "layer";
        case OverlapPolicy::crossfade: return "crossfade";
        case OverlapPolicy::trim:      break;
    }

    return "trim";
}

bool isTrimPolicy (OverlapPolicy policy)
{
    return policy == OverlapPolicy::trim;
}

} // namespace

ClipService::ClipService (EditGetter editGetter)
    : getEdit (std::move (editGetter))
{
}

juce::String ClipService::handleMoveClip (const juce::DynamicObject& object, const juce::String&) const
{
    auto* edit = getEdit != nullptr ? getEdit() : nullptr;

    if (edit == nullptr)
        return makeErrorReply ("No active edit loaded");

    const auto sourceTrackId = object.getProperty ("source_track_id").toString().trim();
    const auto targetTrackId = object.getProperty ("target_track_id").toString().trim();
    const auto clipId = object.getProperty ("clip_id").toString().trim();

    if (sourceTrackId.isEmpty())
        return makeErrorReply ("move_clip requires a non-empty source_track_id");

    if (targetTrackId.isEmpty())
        return makeErrorReply ("move_clip requires a non-empty target_track_id");

    if (clipId.isEmpty())
        return makeErrorReply ("move_clip requires a non-empty clip_id");

    CommandTimeUnit timeUnit = CommandTimeUnit::seconds;
    juce::String parseError;

    if (! parseTimeUnit (object, timeUnit, parseError))
        return makeErrorReply ("move_clip: " + parseError);

    double startValue = 0.0;
    const juce::StringArray startKeys = (timeUnit == CommandTimeUnit::beats)
                                            ? juce::StringArray { "new_start_beat", "new_start_beats", "new_start" }
                                            : juce::StringArray { "new_start_seconds", "new_start" };

    if (! readNumericProperty (object, startKeys, startValue))
        return makeErrorReply ("move_clip requires numeric new_start (or unit-specific alias)");

    double newStartSeconds = 0.0;
    if (! convertTimeValueToSeconds (*edit, timeUnit, startValue, newStartSeconds, parseError))
        return makeErrorReply ("move_clip: " + parseError);

    auto* sourceTrack = dynamic_cast<te::ClipTrack*> (findTrackByID (*edit, sourceTrackId));
    auto* targetTrack = dynamic_cast<te::ClipTrack*> (findTrackByID (*edit, targetTrackId));

    if (sourceTrack == nullptr)
        return makeErrorReply ("move_clip source track not found or is not a clip track: " + sourceTrackId);
    if (targetTrack == nullptr)
        return makeErrorReply ("move_clip target track not found or is not a clip track: " + targetTrackId);

    auto* clip = findClipByID (*edit, clipId);
    if (clip == nullptr)
        return makeErrorReply ("move_clip clip not found for clip_id: " + clipId);

    auto* clipTrack = clip->getClipTrack();
    if (clipTrack == nullptr)
        return makeErrorReply ("move_clip clip has no parent clip track");
    if (clipTrack->itemID.toString() != sourceTrackId)
        return makeErrorReply ("move_clip source_track_id does not match clip's current parent track");

    const auto overlapPolicy = parseOverlapPolicy (object);
    auto& undo = edit->getUndoManager();
    undo.beginNewTransaction ("Move clip");
    OverlapEditResult overlapResult;

    if (sourceTrack != targetTrack)
    {
        if (! clip->moveTo (*targetTrack))
            return makeErrorReply ("move_clip failed to move clip to target track");
    }

    clip->setStart (te::TimePosition::fromSeconds (newStartSeconds), false, true);

    if (isTrimPolicy (overlapPolicy))
    {
        const auto allowSplit = true;
        if (! applyCutOverlapPolicy (*targetTrack, *clip, allowSplit, &overlapResult))
            return makeErrorReply ("move_clip failed to apply trim overlap policy");
    }

    edit->invalidateStoredLength();
    edit->dispatchPendingUpdatesSynchronously();
    edit->getTransport().ensureContextAllocated (true);

    auto response = std::make_unique<juce::DynamicObject>();
    response->setProperty ("status", "ok");
    response->setProperty ("message", "Clip moved");
    response->setProperty ("clip_id", clipId);
    response->setProperty ("source_track_id", sourceTrackId);
    response->setProperty ("target_track_id", targetTrackId);
    response->setProperty ("affected_track_id", targetTrackId);
    response->setProperty ("new_start_seconds", clip->getPosition().getStart().inSeconds());
    response->setProperty ("overlap_mode", overlapPolicyToString (overlapPolicy));
    response->setProperty ("created_clip_ids", stringArrayToVar (overlapResult.createdClipIds));
    response->setProperty ("removed_clip_ids", stringArrayToVar (overlapResult.removedClipIds));
    response->setProperty ("affected_clip_ids", stringArrayToVar (overlapResult.touchedOriginalClipIds));
    return juce::JSON::toString (juce::var (response.release()));
}

juce::String ClipService::handleCloneClip (const juce::DynamicObject& object, const juce::String&) const
{
    auto* edit = getEdit != nullptr ? getEdit() : nullptr;

    if (edit == nullptr)
        return makeErrorReply ("No active edit loaded");

    const auto timeUnitRaw = object.getProperty ("time_unit").toString().trim();

    if (timeUnitRaw.isEmpty())
        return makeErrorReply ("clone_clip requires time_unit (e.g. beats or seconds)");

    const auto sourceClipId = object.getProperty ("source_clip_id").toString().trim();

    if (sourceClipId.isEmpty())
        return makeErrorReply ("clone_clip requires a non-empty source_clip_id");

    const auto targetTrackId = object.getProperty ("target_track_id").toString().trim();

    if (targetTrackId.isEmpty())
        return makeErrorReply ("clone_clip requires a non-empty target_track_id");

    CommandTimeUnit timeUnit = CommandTimeUnit::seconds;
    juce::String parseError;

    if (! parseTimeUnit (object, timeUnit, parseError))
        return makeErrorReply ("clone_clip: " + parseError);

    double startValue = 0.0;

    if (! readNumericProperty (object, { "new_start" }, startValue))
        return makeErrorReply ("clone_clip requires numeric new_start");

    double newStartSeconds = 0.0;

    if (! convertTimeValueToSeconds (*edit, timeUnit, startValue, newStartSeconds, parseError))
        return makeErrorReply ("clone_clip: " + parseError);

    auto* targetTrack = dynamic_cast<te::ClipTrack*> (findTrackByID (*edit, targetTrackId));

    if (targetTrack == nullptr)
        return makeErrorReply ("clone_clip target track not found or is not a clip track: " + targetTrackId);

    auto* sourceClip = findClipByID (*edit, sourceClipId);

    if (sourceClip == nullptr)
        return makeErrorReply ("clone_clip source clip not found for source_clip_id: " + sourceClipId);

    if (! sourceClip->canBeAddedTo (*targetTrack))
        return makeErrorReply ("clone_clip: clip type cannot be added to target track");

    sourceClip->flushStateToValueTree();

    juce::ValueTree clipState = sourceClip->state.createCopy();

    if (! clipState.isValid())
        return makeErrorReply ("clone_clip: failed to copy clip state");

    jassert (! clipState.getParent().isValid());

    auto& undo = edit->getUndoManager();
    undo.beginNewTransaction ("Clone clip");

    regenerateValueTreeIDs (clipState, *edit, &undo);
    clipState.setProperty (te::IDs::start, newStartSeconds, &undo);

    auto* newClip = targetTrack->insertClipWithState (clipState);

    if (newClip == nullptr)
        return makeErrorReply ("clone_clip: engine refused to insert cloned clip");

    edit->invalidateStoredLength();
    edit->dispatchPendingUpdatesSynchronously();
    edit->getTransport().ensureContextAllocated (true);

    auto response = std::make_unique<juce::DynamicObject>();
    response->setProperty ("status", "ok");
    response->setProperty ("new_clip_id", newClip->itemID.toString());
    return juce::JSON::toString (juce::var (response.release()));
}

juce::String ClipService::handleResizeClip (const juce::DynamicObject& object, const juce::String&) const
{
    auto* edit = getEdit != nullptr ? getEdit() : nullptr;

    if (edit == nullptr)
        return makeErrorReply ("No active edit loaded");

    const auto clipId = object.getProperty ("clip_id").toString().trim();

    if (clipId.isEmpty())
        return makeErrorReply ("resize_clip requires a non-empty clip_id");

    auto* clip = findClipByID (*edit, clipId);
    if (clip == nullptr)
        return makeErrorReply ("resize_clip clip not found for clip_id: " + clipId);

    auto* clipTrack = clip->getClipTrack();
    if (clipTrack == nullptr)
        return makeErrorReply ("resize_clip clip has no parent clip track");

    juce::String trackId = object.getProperty ("track_id").toString().trim();
    if (trackId.isEmpty())
        trackId = clipTrack->itemID.toString();
    else if (clipTrack->itemID.toString() != trackId)
        return makeErrorReply ("resize_clip track_id does not match clip's current parent track");

    auto* targetTrack = dynamic_cast<te::ClipTrack*> (findTrackByID (*edit, trackId));
    if (targetTrack == nullptr)
        return makeErrorReply ("resize_clip track not found or is not a clip track: " + trackId);

    CommandTimeUnit timeUnit = CommandTimeUnit::seconds;
    juce::String parseError;

    if (! parseTimeUnit (object, timeUnit, parseError))
        return makeErrorReply ("resize_clip: " + parseError);

    const juce::StringArray startKeys = (timeUnit == CommandTimeUnit::beats)
                                            ? juce::StringArray { "new_start_beat", "new_start_beats", "new_start" }
                                            : juce::StringArray { "new_start_seconds", "new_start" };
    const juce::StringArray lengthKeys = (timeUnit == CommandTimeUnit::beats)
                                            ? juce::StringArray { "new_length_beats", "new_length_beat", "new_length" }
                                            : juce::StringArray { "new_length_seconds", "new_length" };
    const juce::StringArray offsetKeys = (timeUnit == CommandTimeUnit::beats)
                                            ? juce::StringArray { "offset_in_source_beats", "offset_in_source_beat", "offset_in_source" }
                                            : juce::StringArray { "offset_in_source_seconds", "offset_in_source" };

    double startValue = 0.0;
    double lengthValue = 0.0;
    double offsetValue = 0.0;

    const bool haveStart = readNumericProperty (object, startKeys, startValue);
    const bool haveLength = readNumericProperty (object, lengthKeys, lengthValue);
    const bool haveOffset = readNumericProperty (object, offsetKeys, offsetValue);

    if (! haveLength)
        return makeErrorReply ("resize_clip requires numeric new_length (or unit-specific alias)");

    double newStartSeconds = clip->getPosition().getStart().inSeconds();
    if (haveStart)
    {
        if (! convertTimeValueToSeconds (*edit, timeUnit, startValue, newStartSeconds, parseError))
            return makeErrorReply ("resize_clip: " + parseError);
    }

    double newLengthSeconds = 0.0;
    if (! convertDurationValueToSeconds (*edit, timeUnit, newStartSeconds, lengthValue, newLengthSeconds, parseError))
        return makeErrorReply ("resize_clip: " + parseError);
    newLengthSeconds = juce::jmax (newLengthSeconds, kMinimumSurvivingClipLengthSeconds);

    double newOffsetSeconds = clip->getPosition().getOffset().inSeconds();
    if (haveOffset)
    {
        if (! convertTimeValueToSeconds (*edit, timeUnit, offsetValue, newOffsetSeconds, parseError))
            return makeErrorReply ("resize_clip: " + parseError);
    }

    const auto overlapPolicy = parseOverlapPolicy (object);
    auto& undo = edit->getUndoManager();
    undo.beginNewTransaction ("Resize clip");
    OverlapEditResult overlapResult;

    clip->setStart (te::TimePosition::fromSeconds (newStartSeconds), false, true);
    clip->setLength (te::TimeDuration::fromSeconds (newLengthSeconds), false);
    clip->setOffset (te::TimeDuration::fromSeconds (newOffsetSeconds));

    if (isTrimPolicy (overlapPolicy))
    {
        if (! applyCutOverlapPolicy (*targetTrack, *clip, true, &overlapResult))
            return makeErrorReply ("resize_clip failed to apply trim overlap policy");
    }

    edit->invalidateStoredLength();
    edit->dispatchPendingUpdatesSynchronously();
    edit->getTransport().ensureContextAllocated (true);

    auto response = std::make_unique<juce::DynamicObject>();
    response->setProperty ("status", "ok");
    response->setProperty ("message", "Clip resized");
    response->setProperty ("clip_id", clipId);
    response->setProperty ("track_id", trackId);
    response->setProperty ("affected_track_id", trackId);
    response->setProperty ("new_start_seconds", clip->getPosition().getStart().inSeconds());
    response->setProperty ("new_length_seconds", clip->getPosition().getLength().inSeconds());
    response->setProperty ("offset_in_source_seconds", clip->getPosition().getOffset().inSeconds());
    response->setProperty ("overlap_mode", overlapPolicyToString (overlapPolicy));
    response->setProperty ("created_clip_ids", stringArrayToVar (overlapResult.createdClipIds));
    response->setProperty ("removed_clip_ids", stringArrayToVar (overlapResult.removedClipIds));
    response->setProperty ("affected_clip_ids", stringArrayToVar (overlapResult.touchedOriginalClipIds));
    return juce::JSON::toString (juce::var (response.release()));
}

juce::String ClipService::handleSplitClip (const juce::DynamicObject& object, const juce::String&) const
{
    auto* edit = getEdit != nullptr ? getEdit() : nullptr;

    if (edit == nullptr)
        return makeErrorReply ("No active edit loaded");

    const auto clipId = object.getProperty ("clip_id").toString().trim();

    if (clipId.isEmpty())
        return makeErrorReply ("split_clip requires a non-empty clip_id");

    auto* clip = findClipByID (*edit, clipId);
    if (clip == nullptr)
        return makeErrorReply ("split_clip clip not found for clip_id: " + clipId);

    auto* clipTrack = clip->getClipTrack();
    if (clipTrack == nullptr)
        return makeErrorReply ("split_clip clip has no parent clip track");

    juce::String trackId = object.getProperty ("track_id").toString().trim();
    if (trackId.isEmpty())
        trackId = clipTrack->itemID.toString();
    else if (clipTrack->itemID.toString() != trackId)
        return makeErrorReply ("split_clip track_id does not match clip's current parent track");

    CommandTimeUnit timeUnit = CommandTimeUnit::seconds;
    juce::String parseError;

    if (! parseTimeUnit (object, timeUnit, parseError))
        return makeErrorReply ("split_clip: " + parseError);

    const juce::StringArray splitKeys = (timeUnit == CommandTimeUnit::beats)
                                            ? juce::StringArray { "split_time_beats", "split_time_beat", "split_time", "time" }
                                            : juce::StringArray { "split_time_seconds", "split_time", "position_seconds", "time" };
    double splitValue = 0.0;

    if (! readNumericProperty (object, splitKeys, splitValue))
        return makeErrorReply ("split_clip requires numeric split_time (or unit-specific alias)");

    double splitTimeSeconds = 0.0;
    if (! convertTimeValueToSeconds (*edit, timeUnit, splitValue, splitTimeSeconds, parseError))
        return makeErrorReply ("split_clip: " + parseError);

    const auto beforeRange = clip->getPosition().time;
    const auto clipStartSeconds = beforeRange.getStart().inSeconds();
    const auto clipEndSeconds = beforeRange.getEnd().inSeconds();
    auto originalFadeInSeconds = 0.0;
    auto originalFadeOutSeconds = 0.0;

    if (auto* audioClip = dynamic_cast<te::AudioClipBase*> (clip))
    {
        originalFadeInSeconds = audioClip->getFadeIn().inSeconds();
        originalFadeOutSeconds = audioClip->getFadeOut().inSeconds();
    }

    if (splitTimeSeconds <= clipStartSeconds + kMinimumSurvivingClipLengthSeconds
        || splitTimeSeconds >= clipEndSeconds - kMinimumSurvivingClipLengthSeconds)
    {
        return makeErrorReply ("split_clip split_time must be inside the clip, leaving audio on both sides");
    }

    auto& undo = edit->getUndoManager();
    undo.beginNewTransaction ("Split clip");

    auto* newClip = clipTrack->splitClip (*clip, te::TimePosition::fromSeconds (splitTimeSeconds));

    if (newClip == nullptr)
        return makeErrorReply ("split_clip failed to split clip");

    restoreSplitBoundaryFades (*clip, *newClip, originalFadeInSeconds, originalFadeOutSeconds);

    edit->invalidateStoredLength();
    edit->dispatchPendingUpdatesSynchronously();
    edit->getTransport().ensureContextAllocated (true);

    auto response = std::make_unique<juce::DynamicObject>();
    response->setProperty ("status", "ok");
    response->setProperty ("message", "Clip split");
    response->setProperty ("clip_id", clipId);
    response->setProperty ("left_clip_id", clip->itemID.toString());
    response->setProperty ("right_clip_id", newClip->itemID.toString());
    response->setProperty ("new_clip_id", newClip->itemID.toString());
    response->setProperty ("track_id", trackId);
    response->setProperty ("affected_track_id", trackId);
    response->setProperty ("split_time_seconds", splitTimeSeconds);
    response->setProperty ("left_start_seconds", clip->getPosition().getStart().inSeconds());
    response->setProperty ("left_length_seconds", clip->getPosition().getLength().inSeconds());
    response->setProperty ("right_start_seconds", newClip->getPosition().getStart().inSeconds());
    response->setProperty ("right_length_seconds", newClip->getPosition().getLength().inSeconds());
    return juce::JSON::toString (juce::var (response.release()));
}

juce::String ClipService::handleRemoveClips (const juce::DynamicObject& object, const juce::String&) const
{
    auto* edit = getEdit != nullptr ? getEdit() : nullptr;

    if (edit == nullptr)
        return makeErrorReply ("No active edit loaded");

    const auto idsVar = object.getProperty ("clip_ids");

    if (! idsVar.isArray())
        return makeErrorReply ("remove_clips requires clip_ids array");

    auto* arr = idsVar.getArray();

    if (arr == nullptr)
        return makeErrorReply ("remove_clips requires clip_ids array");

    std::vector<juce::String> uniqueOrdered;
    std::unordered_set<std::string> seen;

    for (const auto& item : *arr)
    {
        const auto id = item.toString().trim();

        if (id.isEmpty())
            continue;

        const auto key = id.toStdString();

        if (seen.insert (key).second)
            uniqueOrdered.push_back (id);
    }

    if (uniqueOrdered.empty())
    {
        auto response = std::make_unique<juce::DynamicObject>();
        response->setProperty ("status", "ok");
        response->setProperty ("message", "No clip ids to remove");
        response->setProperty ("removed_count", 0);
        response->setProperty ("missing_ids", stringArrayToVar (juce::StringArray()));
        return juce::JSON::toString (juce::var (response.release()));
    }

    juce::StringArray missingIds;
    auto& undo = edit->getUndoManager();
    undo.beginNewTransaction ("Remove clips");
    int removedCount = 0;

    for (const auto& clipId : uniqueOrdered)
    {
        auto* clip = findClipByID (*edit, clipId);

        if (clip == nullptr)
        {
            missingIds.add (clipId);
            continue;
        }

        if (dynamic_cast<te::AudioClipBase*> (clip) != nullptr)
            AudioFeatureService::invalidateClipBake (clip->itemID.toString());

        clip->deselect();
        clip->removeFromParent();
        ++removedCount;
    }

    edit->invalidateStoredLength();
    edit->dispatchPendingUpdatesSynchronously();
    edit->getTransport().ensureContextAllocated (true);

    if (removedCount == 0)
    {
        auto response = std::make_unique<juce::DynamicObject>();
        response->setProperty ("status", "error");
        response->setProperty ("message", "remove_clips: no matching clips for supplied clip_ids");
        response->setProperty ("removed_count", 0);
        response->setProperty ("missing_ids", stringArrayToVar (missingIds));
        return juce::JSON::toString (juce::var (response.release()));
    }

    auto response = std::make_unique<juce::DynamicObject>();
    response->setProperty ("status", "ok");
    response->setProperty ("message", "Clips removed");
    response->setProperty ("removed_count", removedCount);
    response->setProperty ("missing_ids", stringArrayToVar (missingIds));
    return juce::JSON::toString (juce::var (response.release()));
}

juce::String ClipService::handleAnalyzeStripSilence (const juce::DynamicObject& object, const juce::String&) const
{
    auto* edit = getEdit != nullptr ? getEdit() : nullptr;

    if (edit == nullptr)
        return makeErrorReply ("No active edit loaded");

    const auto clipId = object.getProperty ("clip_id").toString().trim();

    if (clipId.isEmpty())
        return makeErrorReply ("clip.strip_silence.analyze requires a non-empty clip_id");

    auto* clip = findClipByID (*edit, clipId);
    if (clip == nullptr)
        return makeErrorReply ("clip.strip_silence.analyze clip not found for clip_id: " + clipId);

    auto* audioClip = dynamic_cast<te::AudioClipBase*> (clip);
    if (audioClip == nullptr)
        return makeErrorReply ("clip.strip_silence.analyze requires an audio clip: " + clipId);

    const auto trackId = object.getProperty ("track_id").toString().trim();
    if (trackId.isNotEmpty() && clipTrackIdOrEmpty (*clip) != trackId)
        return makeErrorReply ("clip.strip_silence.analyze track_id does not match clip's current parent track");

    StripSilenceAnalysisResult analysis;
    juce::String error;
    if (! analyseStripSilenceForClip (*edit, *clip, *audioClip, object, analysis, error))
        return makeErrorReply (error);

    return juce::JSON::toString (stripAnalysisToVar (analysis));
}

juce::String ClipService::handleApplyStripSilence (const juce::DynamicObject& object, const juce::String&) const
{
    auto* edit = getEdit != nullptr ? getEdit() : nullptr;

    if (edit == nullptr)
        return makeErrorReply ("No active edit loaded");

    auto clipId = object.getProperty ("clip_id").toString().trim();
    if (clipId.isEmpty())
        if (auto* analysisObject = object.getProperty ("analysis").getDynamicObject())
            clipId = analysisObject->getProperty ("clip_id").toString().trim();

    if (clipId.isEmpty())
        return makeErrorReply ("clip.strip_silence.apply requires a non-empty clip_id");

    auto* clip = findClipByID (*edit, clipId);
    if (clip == nullptr)
        return makeErrorReply ("clip.strip_silence.apply clip not found for clip_id: " + clipId);

    auto* audioClip = dynamic_cast<te::AudioClipBase*> (clip);
    if (audioClip == nullptr)
        return makeErrorReply ("clip.strip_silence.apply requires an audio clip: " + clipId);

    juce::String trackId = object.getProperty ("track_id").toString().trim();
    if (trackId.isEmpty())
        if (auto* analysisObject = object.getProperty ("analysis").getDynamicObject())
            trackId = analysisObject->getProperty ("track_id").toString().trim();
    if (trackId.isEmpty())
        trackId = clipTrackIdOrEmpty (*clip);
    else if (clipTrackIdOrEmpty (*clip) != trackId)
        return makeErrorReply ("clip.strip_silence.apply track_id does not match clip's current parent track");

    auto regions = parseStripRegionsForApply (object, *clip, trackId);

    if (regions.empty())
        return makeErrorReply ("clip.strip_silence.apply requires strip_regions from a confirmed clip.strip_silence.analyze result");

    bool allowRemoveEntireClip = false;
    readBoolProperty (object, { "allow_remove_entire_clip", "allow_full_clip_removal", "allow_delete_entire_clip" }, allowRemoveEntireClip);

    const auto originalClipStartSeconds = clip->getPosition().getStart().inSeconds();
    const auto originalClipEndSeconds = clip->getPosition().getEnd().inSeconds();
    bool wouldRemoveEntireClip = false;
    double totalStripSeconds = 0.0;

    for (const auto& region : regions)
    {
        totalStripSeconds += juce::jmax (0.0, region.endSeconds - region.startSeconds);
        if (region.startSeconds <= originalClipStartSeconds + kClipEditEpsilonSeconds
            && region.endSeconds >= originalClipEndSeconds - kClipEditEpsilonSeconds)
        {
            wouldRemoveEntireClip = true;
        }
    }

    if (totalStripSeconds >= (originalClipEndSeconds - originalClipStartSeconds) - kClipEditEpsilonSeconds)
        wouldRemoveEntireClip = true;

    if (wouldRemoveEntireClip && ! allowRemoveEntireClip)
        return makeErrorReply ("clip.strip_silence.apply would remove the entire clip; set allow_remove_entire_clip=true to confirm");

    juce::StringArray createdClipIds;
    juce::StringArray removedClipIds;
    juce::StringArray affectedClipIds;
    affectedClipIds.addIfNotAlreadyThere (clipId);

    auto& undo = edit->getUndoManager();
    undo.beginNewTransaction ("Strip silence");

    int appliedCount = 0;
    for (const auto& region : regions)
    {
        auto* currentClip = findClipByID (*edit, clipId);
        if (currentClip == nullptr)
            break;

        const auto regionStartSeconds = juce::jmin (region.startSeconds, region.endSeconds);
        const auto regionEndSeconds = juce::jmax (region.startSeconds, region.endSeconds);

        if (regionEndSeconds - regionStartSeconds < kMinimumStripRegionSeconds)
            continue;

        const auto deleteRange = te::TimeRange (te::TimePosition::fromSeconds (regionStartSeconds),
                                                te::TimePosition::fromSeconds (regionEndSeconds));
        const auto newClips = te::deleteRegion (*currentClip, deleteRange);
        ++appliedCount;

        for (int i = 0; i < newClips.size(); ++i)
        {
            if (auto* newClip = newClips.getUnchecked (i))
            {
                const auto newClipId = newClip->itemID.toString();
                createdClipIds.addIfNotAlreadyThere (newClipId);
                affectedClipIds.addIfNotAlreadyThere (newClipId);
            }
        }
    }

    if (findClipByID (*edit, clipId) == nullptr)
        removedClipIds.addIfNotAlreadyThere (clipId);

    for (const auto& id : affectedClipIds)
        AudioFeatureService::invalidateClipBake (id);
    for (const auto& id : removedClipIds)
        AudioFeatureService::invalidateClipBake (id);

    edit->invalidateStoredLength();
    edit->dispatchPendingUpdatesSynchronously();
    edit->getTransport().ensureContextAllocated (true);

    auto response = std::make_unique<juce::DynamicObject>();
    response->setProperty ("status", appliedCount > 0 ? "ok" : "error");
    response->setProperty ("message", appliedCount > 0 ? "Strip silence applied" : "No strip silence regions overlapped the clip");
    response->setProperty ("clip_id", clipId);
    response->setProperty ("track_id", trackId);
    response->setProperty ("applied_region_count", appliedCount);
    response->setProperty ("strip_regions", stripRegionsToVar (regions));
    response->setProperty ("created_clip_ids", stringArrayToVar (createdClipIds));
    response->setProperty ("removed_clip_ids", stringArrayToVar (removedClipIds));
    response->setProperty ("affected_clip_ids", stringArrayToVar (affectedClipIds));
    response->setProperty ("would_remove_entire_clip", wouldRemoveEntireClip);
    return juce::JSON::toString (juce::var (response.release()));
}

juce::String ClipService::handleReadClipFade (const juce::DynamicObject& object, const juce::String&) const
{
    auto* edit = getEdit != nullptr ? getEdit() : nullptr;

    if (edit == nullptr)
        return makeErrorReply ("No active edit loaded");

    const auto clipId = object.getProperty ("clip_id").toString().trim();

    if (clipId.isEmpty())
        return makeErrorReply ("clip.fade.read requires a non-empty clip_id");

    auto* clip = findClipByID (*edit, clipId);
    if (clip == nullptr)
        return makeErrorReply ("clip.fade.read clip not found for clip_id: " + clipId);

    auto* audioClip = dynamic_cast<te::AudioClipBase*> (clip);
    if (audioClip == nullptr)
        return makeErrorReply ("clip.fade.read requires an audio clip: " + clipId);

    auto response = std::make_unique<juce::DynamicObject>();
    response->setProperty ("status", "ok");
    response->setProperty ("message", "Clip fade read");
    response->setProperty ("clip_id", clipId);
    response->setProperty ("track_id", clipTrackIdOrEmpty (*clip));
    appendClipFadeState (*response, *audioClip);
    return juce::JSON::toString (juce::var (response.release()));
}

juce::String ClipService::handleSetClipFade (const juce::DynamicObject& object, const juce::String&) const
{
    auto* edit = getEdit != nullptr ? getEdit() : nullptr;

    if (edit == nullptr)
        return makeErrorReply ("No active edit loaded");

    const auto clipId = object.getProperty ("clip_id").toString().trim();

    if (clipId.isEmpty())
        return makeErrorReply ("clip.fade.set requires a non-empty clip_id");

    auto* clip = findClipByID (*edit, clipId);
    if (clip == nullptr)
        return makeErrorReply ("clip.fade.set clip not found for clip_id: " + clipId);

    auto* audioClip = dynamic_cast<te::AudioClipBase*> (clip);
    if (audioClip == nullptr)
        return makeErrorReply ("clip.fade.set requires an audio clip: " + clipId);

    const auto before = createClipFadeState (*clip, *audioClip);
    const auto clipLengthSeconds = clip->getPosition().getLength().inSeconds();

    double fadeInSeconds = 0.0;
    double fadeOutSeconds = 0.0;
    const bool haveFadeIn = readNumericProperty (object, { "fade_in_seconds", "fade_in", "fadeInSeconds", "fadeIn" }, fadeInSeconds);
    const bool haveFadeOut = readNumericProperty (object, { "fade_out_seconds", "fade_out", "fadeOutSeconds", "fadeOut" }, fadeOutSeconds);

    te::AudioFadeCurve::Type fadeInCurve = audioClip->getFadeInType();
    te::AudioFadeCurve::Type fadeOutCurve = audioClip->getFadeOutType();
    bool haveFadeInCurve = false;
    bool haveFadeOutCurve = false;
    const auto sharedCurveRaw = object.getProperty ("curve").toString();

    if (! sharedCurveRaw.trim().isEmpty())
    {
        if (! parseFadeCurve (sharedCurveRaw, fadeInCurve))
            return makeErrorReply ("clip.fade.set unsupported curve: " + sharedCurveRaw);
        fadeOutCurve = fadeInCurve;
        haveFadeInCurve = true;
        haveFadeOutCurve = true;
    }

    const auto fadeInCurveRaw = object.getProperty ("fade_in_curve").toString();
    if (! fadeInCurveRaw.trim().isEmpty())
    {
        if (! parseFadeCurve (fadeInCurveRaw, fadeInCurve))
            return makeErrorReply ("clip.fade.set unsupported fade_in_curve: " + fadeInCurveRaw);
        haveFadeInCurve = true;
    }

    const auto fadeOutCurveRaw = object.getProperty ("fade_out_curve").toString();
    if (! fadeOutCurveRaw.trim().isEmpty())
    {
        if (! parseFadeCurve (fadeOutCurveRaw, fadeOutCurve))
            return makeErrorReply ("clip.fade.set unsupported fade_out_curve: " + fadeOutCurveRaw);
        haveFadeOutCurve = true;
    }

    te::AudioClipBase::FadeBehaviour fadeInBehaviour = audioClip->getFadeInBehaviour();
    te::AudioClipBase::FadeBehaviour fadeOutBehaviour = audioClip->getFadeOutBehaviour();
    bool haveFadeInBehaviour = false;
    bool haveFadeOutBehaviour = false;
    const auto sharedBehaviourRaw = object.getProperty ("behaviour").toString();

    if (! sharedBehaviourRaw.trim().isEmpty())
    {
        if (! parseFadeBehaviour (sharedBehaviourRaw, fadeInBehaviour))
            return makeErrorReply ("clip.fade.set unsupported behaviour: " + sharedBehaviourRaw);
        fadeOutBehaviour = fadeInBehaviour;
        haveFadeInBehaviour = true;
        haveFadeOutBehaviour = true;
    }

    const auto fadeInBehaviourRaw = object.getProperty ("fade_in_behaviour").toString();
    if (! fadeInBehaviourRaw.trim().isEmpty())
    {
        if (! parseFadeBehaviour (fadeInBehaviourRaw, fadeInBehaviour))
            return makeErrorReply ("clip.fade.set unsupported fade_in_behaviour: " + fadeInBehaviourRaw);
        haveFadeInBehaviour = true;
    }

    const auto fadeOutBehaviourRaw = object.getProperty ("fade_out_behaviour").toString();
    if (! fadeOutBehaviourRaw.trim().isEmpty())
    {
        if (! parseFadeBehaviour (fadeOutBehaviourRaw, fadeOutBehaviour))
            return makeErrorReply ("clip.fade.set unsupported fade_out_behaviour: " + fadeOutBehaviourRaw);
        haveFadeOutBehaviour = true;
    }

    bool autoCrossfade = audioClip->getAutoCrossfade();
    const bool haveAutoCrossfade = readBoolProperty (object, { "auto_crossfade", "autoCrossfade" }, autoCrossfade);

    if (! haveFadeIn && ! haveFadeOut && ! haveFadeInCurve && ! haveFadeOutCurve
        && ! haveFadeInBehaviour && ! haveFadeOutBehaviour && ! haveAutoCrossfade)
        return makeErrorReply ("clip.fade.set requires fade_in_seconds and/or fade_out_seconds or fade metadata");

    auto& undo = edit->getUndoManager();
    undo.beginNewTransaction ("Set clip fade");

    if (haveAutoCrossfade)
        audioClip->setAutoCrossfade (autoCrossfade);
    if (haveFadeInCurve)
        audioClip->setFadeInType (fadeInCurve);
    if (haveFadeOutCurve)
        audioClip->setFadeOutType (fadeOutCurve);
    if (haveFadeInBehaviour)
        audioClip->setFadeInBehaviour (fadeInBehaviour);
    if (haveFadeOutBehaviour)
        audioClip->setFadeOutBehaviour (fadeOutBehaviour);
    if (haveFadeIn)
        audioClip->setFadeIn (te::TimeDuration::fromSeconds (juce::jlimit (0.0, clipLengthSeconds, fadeInSeconds)));
    if (haveFadeOut)
        audioClip->setFadeOut (te::TimeDuration::fromSeconds (juce::jlimit (0.0, clipLengthSeconds, fadeOutSeconds)));

    clip->flushStateToValueTree();
    edit->invalidateStoredLength();
    edit->dispatchPendingUpdatesSynchronously();
    edit->getTransport().ensureContextAllocated (true);

    auto response = std::make_unique<juce::DynamicObject>();
    response->setProperty ("status", "ok");
    response->setProperty ("message", "Clip fade set");
    response->setProperty ("clip_id", clipId);
    response->setProperty ("track_id", clipTrackIdOrEmpty (*clip));
    response->setProperty ("before", before);
    response->setProperty ("after", createClipFadeState (*clip, *audioClip));
    appendClipFadeState (*response, *audioClip);
    return juce::JSON::toString (juce::var (response.release()));
}

juce::String ClipService::handleReadClipGain (const juce::DynamicObject& object, const juce::String&) const
{
    auto* edit = getEdit != nullptr ? getEdit() : nullptr;

    if (edit == nullptr)
        return makeErrorReply ("No active edit loaded");

    const auto clipId = object.getProperty ("clip_id").toString().trim();

    if (clipId.isEmpty())
        return makeErrorReply ("clip.gain.read requires a non-empty clip_id");

    auto* clip = findClipByID (*edit, clipId);
    if (clip == nullptr)
        return makeErrorReply ("clip.gain.read clip not found for clip_id: " + clipId);

    auto* audioClip = dynamic_cast<te::AudioClipBase*> (clip);
    if (audioClip == nullptr)
        return makeErrorReply ("clip.gain.read requires an audio clip: " + clipId);

    auto response = std::make_unique<juce::DynamicObject>();
    response->setProperty ("status", "ok");
    response->setProperty ("message", "Clip gain read");
    response->setProperty ("clip_id", clipId);
    response->setProperty ("track_id", clipTrackIdOrEmpty (*clip));
    appendClipGainState (*response, *audioClip);
    return juce::JSON::toString (juce::var (response.release()));
}

juce::String ClipService::handleSetClipGain (const juce::DynamicObject& object, const juce::String&) const
{
    auto* edit = getEdit != nullptr ? getEdit() : nullptr;

    if (edit == nullptr)
        return makeErrorReply ("No active edit loaded");

    const auto clipId = object.getProperty ("clip_id").toString().trim();

    if (clipId.isEmpty())
        return makeErrorReply ("clip.gain.set requires a non-empty clip_id");

    auto* clip = findClipByID (*edit, clipId);
    if (clip == nullptr)
        return makeErrorReply ("clip.gain.set clip not found for clip_id: " + clipId);

    auto* audioClip = dynamic_cast<te::AudioClipBase*> (clip);
    if (audioClip == nullptr)
        return makeErrorReply ("clip.gain.set requires an audio clip: " + clipId);

    double gainDb = 0.0;
    if (! readNumericProperty (object, { "gain_db", "clip_gain_db", "db" }, gainDb))
        return makeErrorReply ("clip.gain.set requires numeric gain_db");

    const auto before = createClipGainState (*clip, *audioClip);
    auto& undo = edit->getUndoManager();
    undo.beginNewTransaction ("Set clip gain");

    audioClip->setGainDB (static_cast<float> (gainDb));

    clip->flushStateToValueTree();
    edit->dispatchPendingUpdatesSynchronously();
    edit->getTransport().ensureContextAllocated (true);

    auto response = std::make_unique<juce::DynamicObject>();
    response->setProperty ("status", "ok");
    response->setProperty ("message", "Clip gain set");
    response->setProperty ("clip_id", clipId);
    response->setProperty ("track_id", clipTrackIdOrEmpty (*clip));
    response->setProperty ("before", before);
    response->setProperty ("after", createClipGainState (*clip, *audioClip));
    appendClipGainState (*response, *audioClip);
    return juce::JSON::toString (juce::var (response.release()));
}

juce::String ClipService::handleSetClipGainBatch (const juce::DynamicObject& object, const juce::String&) const
{
    auto* edit = getEdit != nullptr ? getEdit() : nullptr;

    if (edit == nullptr)
        return makeErrorReply ("No active edit loaded");

    auto actionsVar = object.getProperty ("pending_actions");
    if (! actionsVar.isArray())
        actionsVar = object.getProperty ("actions");

    auto* actions = actionsVar.getArray();
    if (actions == nullptr || actions->isEmpty())
        return makeErrorReply ("clip.gain.set_batch requires pending_actions");

    struct Target
    {
        te::Clip* clip = nullptr;
        te::AudioClipBase* audioClip = nullptr;
        juce::String clipId;
        juce::String trackId;
        double gainDb = 0.0;
    };

    std::vector<Target> targets;
    targets.reserve (static_cast<size_t> (actions->size()));
    juce::StringArray seenClipIds;

    for (const auto& actionVar : *actions)
    {
        auto* action = actionVar.getDynamicObject();
        if (action == nullptr)
            return makeErrorReply ("clip.gain.set_batch contains a non-object action");

        auto* args = action;
        if (auto* nestedArgs = action->getProperty ("args").getDynamicObject())
            args = nestedArgs;

        const auto clipId = args->getProperty ("clip_id").toString().trim().isNotEmpty()
                                ? args->getProperty ("clip_id").toString().trim()
                                : action->getProperty ("clip_id").toString().trim();
        if (clipId.isEmpty())
            return makeErrorReply ("clip.gain.set_batch action requires clip_id");
        if (seenClipIds.contains (clipId))
            return makeErrorReply ("clip.gain.set_batch contains duplicate clip_id: " + clipId);

        double gainDb = 0.0;
        if (! readNumericProperty (*args, { "gain_db", "clip_gain_db", "db" }, gainDb)
            && ! readNumericProperty (*action, { "gain_db", "clip_gain_db", "target_gain_db" }, gainDb))
            return makeErrorReply ("clip.gain.set_batch action requires numeric gain_db: " + clipId);

        auto* clip = findClipByID (*edit, clipId);
        if (clip == nullptr)
            return makeErrorReply ("clip.gain.set_batch clip not found for clip_id: " + clipId);

        auto* audioClip = dynamic_cast<te::AudioClipBase*> (clip);
        if (audioClip == nullptr)
            return makeErrorReply ("clip.gain.set_batch requires audio clips: " + clipId);

        seenClipIds.add (clipId);
        targets.push_back ({ clip, audioClip, clipId, clipTrackIdOrEmpty (*clip), gainDb });
    }

    auto& undo = edit->getUndoManager();
    undo.beginNewTransaction ("Set clip gain batch");

    juce::Array<juce::var> results;
    results.ensureStorageAllocated (static_cast<int> (targets.size()));
    for (const auto& target : targets)
    {
        target.audioClip->setGainDB (static_cast<float> (target.gainDb));
        target.clip->flushStateToValueTree();

        auto row = std::make_unique<juce::DynamicObject>();
        row->setProperty ("status", "ok");
        row->setProperty ("clip_id", target.clipId);
        row->setProperty ("track_id", target.trackId);
        row->setProperty ("requested_gain_db", target.gainDb);
        row->setProperty ("clip_gain_db", static_cast<double> (target.audioClip->getGainDB()));
        results.add (juce::var (row.release()));
    }

    edit->dispatchPendingUpdatesSynchronously();
    edit->getTransport().ensureContextAllocated (true);

    auto response = std::make_unique<juce::DynamicObject>();
    response->setProperty ("status", "ok");
    response->setProperty ("message", "Clip gain batch applied");
    response->setProperty ("schema_version", "clip.gain.set_batch.v1");
    response->setProperty ("pending_action_count", static_cast<int> (targets.size()));
    response->setProperty ("applied_count", static_cast<int> (targets.size()));
    response->setProperty ("failed_count", 0);
    response->setProperty ("actions", results);
    return juce::JSON::toString (juce::var (response.release()));
}

juce::String ClipService::makeStatusReply (const juce::String& status, const juce::String& message)
{
    auto response = std::make_unique<juce::DynamicObject>();
    response->setProperty ("status", status);
    response->setProperty ("message", message);
    return juce::JSON::toString (juce::var (response.release()));
}

juce::String ClipService::makeErrorReply (const juce::String& message)
{
    return makeStatusReply ("error", message);
}

} // namespace vit
