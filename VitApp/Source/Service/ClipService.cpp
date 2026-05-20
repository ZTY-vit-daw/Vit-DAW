#include "ClipService.h"

#include "TiledSpectrogramBaker.h"

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

    applyMicroFadeIfNeeded (*clip);
    applyMicroFadeIfNeeded (*newClip);

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
            TiledSpectrogramBaker::invalidateClipBake (clip->itemID.toString());

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
