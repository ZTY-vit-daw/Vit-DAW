#include "MidiService.h"

#include <cmath>

namespace vit
{

namespace
{

enum class CommandTimeUnit
{
    seconds,
    beats
};

constexpr double kMinimumSurvivingClipLengthSeconds = 0.01;
const juce::Identifier kVitNoteId ("vit_note_id");

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

juce::var stringArrayToVar (const juce::StringArray& values)
{
    juce::Array<juce::var> out;
    for (const auto& value : values)
        out.add (value);
    return juce::var (out);
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

te::MidiNote* findMidiNoteByVitId (te::MidiList& list, const juce::String& id)
{
    if (id.isEmpty())
        return nullptr;

    for (auto* note : list.getNotes())
        if (note != nullptr && note->state.getProperty (kVitNoteId).toString() == id)
            return note;

    return nullptr;
}

bool tryReadMidiInt (const juce::DynamicObject& o, const juce::String& key, int& out)
{
    if (! o.hasProperty (key))
        return false;

    const auto v = o.getProperty (key);

    if (v.isInt() || v.isInt64())
    {
        out = static_cast<int> (v);
        return true;
    }

    if (v.isDouble())
    {
        out = static_cast<int> (std::lround (static_cast<double> (v)));
        return true;
    }

    return false;
}

bool clipLocalNoteGeometryToBeats (te::Edit& edit,
                                   te::MidiClip& clip,
                                   CommandTimeUnit unit,
                                   double startVal,
                                   double lengthVal,
                                   double& outStartBeats,
                                   double& outLenBeats,
                                   juce::String& error)
{
    if (unit == CommandTimeUnit::beats)
    {
        if (lengthVal <= 0.0)
        {
            error = "note length must be greater than zero";
            return false;
        }

        outStartBeats = startVal;
        outLenBeats = lengthVal;
        return true;
    }

    if (lengthVal <= 0.0)
    {
        error = "note length in seconds must be greater than zero";
        return false;
    }

    const auto clipStart = clip.getPosition().getStart();
    const auto clipStartBeat = edit.tempoSequence.toBeats (clipStart);
    const auto noteStartTime = clipStart + te::TimeDuration::fromSeconds (startVal);
    const auto noteEndTime = noteStartTime + te::TimeDuration::fromSeconds (lengthVal);
    const auto noteStartBeat = edit.tempoSequence.toBeats (noteStartTime);
    const auto noteEndBeat = edit.tempoSequence.toBeats (noteEndTime);
    outStartBeats = noteStartBeat.inBeats() - clipStartBeat.inBeats();
    outLenBeats = noteEndBeat.inBeats() - noteStartBeat.inBeats();

    if (outLenBeats <= 0.0)
    {
        error = "note length conversion produced non-positive length";
        return false;
    }

    return true;
}

} // namespace

MidiService::MidiService (EditGetter editGetter)
    : getEdit (std::move (editGetter))
{
}

juce::String MidiService::handleAddMidiNotes (const juce::DynamicObject& object, const juce::String&) const
{
    auto* edit = getEdit != nullptr ? getEdit() : nullptr;

    if (edit == nullptr)
        return makeErrorReply ("No active edit loaded");

    const auto trackId = object.getProperty ("track_id").toString().trim();
    const auto clipId = object.getProperty ("clip_id").toString().trim();

    if (trackId.isEmpty())
        return makeErrorReply ("add_midi_notes requires a non-empty track_id");

    if (clipId.isEmpty())
        return makeErrorReply ("add_midi_notes requires a non-empty clip_id");

    CommandTimeUnit timeUnit = CommandTimeUnit::seconds;
    juce::String parseError;

    if (! parseTimeUnit (object, timeUnit, parseError))
        return makeErrorReply ("add_midi_notes: " + parseError);

    const auto notesVar = object.getProperty ("notes");

    if (! notesVar.isArray())
        return makeErrorReply ("add_midi_notes requires notes array");

    auto* arr = notesVar.getArray();

    if (arr == nullptr || arr->isEmpty())
        return makeErrorReply ("add_midi_notes requires a non-empty notes array");

    auto* clipTrack = dynamic_cast<te::ClipTrack*> (findTrackByID (*edit, trackId));

    if (clipTrack == nullptr)
        return makeErrorReply ("add_midi_notes track not found or is not a clip track: " + trackId);

    auto* clip = findClipByID (*edit, clipId);

    if (clip == nullptr)
        return makeErrorReply ("add_midi_notes clip not found for clip_id: " + clipId);

    if (clip->getClipTrack() == nullptr || clip->getClipTrack()->itemID.toString() != trackId)
        return makeErrorReply ("add_midi_notes track_id does not match clip's current parent track");

    auto* midiClip = dynamic_cast<te::MidiClip*> (clip);

    if (midiClip == nullptr)
        return makeErrorReply ("add_midi_notes clip is not a MIDI clip");

    auto& seq = midiClip->getSequence();
    auto& undo = edit->getUndoManager();
    undo.beginNewTransaction ("Add MIDI notes");
    int added = 0;

    for (const auto& item : *arr)
    {
        auto* noteObj = item.getDynamicObject();

        if (noteObj == nullptr)
            return makeErrorReply ("add_midi_notes: each note must be an object");

        const auto noteId = noteObj->getProperty ("id").toString().trim();

        if (noteId.isEmpty())
            return makeErrorReply ("add_midi_notes: each note requires a non-empty id");

        if (findMidiNoteByVitId (seq, noteId) != nullptr)
            return makeErrorReply ("add_midi_notes: duplicate note id: " + noteId);

        double rawStart = 0.0;
        double rawLen = 0.0;

        if (! readNumericProperty (*noteObj, { "start" }, rawStart))
            return makeErrorReply ("add_midi_notes: note missing numeric start: " + noteId);

        if (! readNumericProperty (*noteObj, { "length" }, rawLen))
            return makeErrorReply ("add_midi_notes: note missing numeric length: " + noteId);

        double startBeats = 0.0;
        double lenBeats = 0.0;

        if (! clipLocalNoteGeometryToBeats (*edit, *midiClip, timeUnit, rawStart, rawLen, startBeats, lenBeats, parseError))
            return makeErrorReply ("add_midi_notes: " + parseError);

        int pitch = 60;

        if (! tryReadMidiInt (*noteObj, "pitch", pitch))
            return makeErrorReply ("add_midi_notes: note missing pitch: " + noteId);

        pitch = juce::jlimit (0, 127, pitch);

        int velocity = 100;

        if (tryReadMidiInt (*noteObj, "velocity", velocity))
            velocity = juce::jlimit (1, 127, velocity);

        auto* created = seq.addNote (pitch,
                                     te::BeatPosition::fromBeats (startBeats),
                                     te::BeatDuration::fromBeats (lenBeats),
                                     velocity,
                                     0,
                                     &undo);

        if (created == nullptr)
            return makeErrorReply ("add_midi_notes: engine refused note: " + noteId);

        created->state.setProperty (kVitNoteId, noteId, &undo);
        ++added;
    }

    edit->invalidateStoredLength();
    edit->dispatchPendingUpdatesSynchronously();
    edit->getTransport().ensureContextAllocated (true);

    auto response = std::make_unique<juce::DynamicObject>();
    response->setProperty ("status", "ok");
    response->setProperty ("message", "MIDI notes added");
    response->setProperty ("clip_id", clipId);
    response->setProperty ("track_id", trackId);
    response->setProperty ("added_count", added);
    return juce::JSON::toString (juce::var (response.release()));
}

juce::String MidiService::handleAddMidiNotesBulk (const juce::DynamicObject& object, const juce::String&) const
{
    auto* edit = getEdit != nullptr ? getEdit() : nullptr;

    if (edit == nullptr)
        return makeErrorReply ("No active edit loaded");

    const auto clipId = object.getProperty ("clip_id").toString().trim();

    if (clipId.isEmpty())
        return makeErrorReply ("add_midi_notes_bulk requires a non-empty clip_id");

    auto* clip = findClipByID (*edit, clipId);

    if (clip == nullptr)
        return makeErrorReply ("add_midi_notes_bulk clip not found for clip_id: " + clipId);

    auto* clipTrack = clip->getClipTrack();

    if (clipTrack == nullptr)
        return makeErrorReply ("add_midi_notes_bulk clip has no parent clip track");

    juce::String trackId = object.getProperty ("track_id").toString().trim();

    if (trackId.isEmpty())
        trackId = clipTrack->itemID.toString();
    else if (clipTrack->itemID.toString() != trackId)
        return makeErrorReply ("add_midi_notes_bulk track_id does not match clip's current parent track");

    auto* targetTrack = dynamic_cast<te::ClipTrack*> (findTrackByID (*edit, trackId));

    if (targetTrack == nullptr)
        return makeErrorReply ("add_midi_notes_bulk track not found or is not a clip track: " + trackId);

    auto* midiClip = dynamic_cast<te::MidiClip*> (clip);

    if (midiClip == nullptr)
        return makeErrorReply ("add_midi_notes_bulk clip is not a MIDI clip");

    CommandTimeUnit timeUnit = CommandTimeUnit::seconds;
    juce::String parseError;

    if (! parseTimeUnit (object, timeUnit, parseError))
        return makeErrorReply ("add_midi_notes_bulk: " + parseError);

    const auto notesVar = object.getProperty ("notes");

    if (! notesVar.isArray())
        return makeErrorReply ("add_midi_notes_bulk requires notes array");

    auto* arr = notesVar.getArray();

    if (arr == nullptr || arr->isEmpty())
        return makeErrorReply ("add_midi_notes_bulk requires a non-empty notes array");

    auto& seq = midiClip->getSequence();
    auto& undo = edit->getUndoManager();
    undo.beginNewTransaction ("Add MIDI Block");
    int added = 0;

    for (const auto& item : *arr)
    {
        auto* noteObj = item.getDynamicObject();

        if (noteObj == nullptr)
            return makeErrorReply ("add_midi_notes_bulk: each note must be an object");

        auto noteId = noteObj->getProperty ("id").toString().trim();

        if (noteId.isEmpty())
            noteId = juce::String ("blk_") + juce::Uuid().toString();

        if (findMidiNoteByVitId (seq, noteId) != nullptr)
            return makeErrorReply ("add_midi_notes_bulk: duplicate note id: " + noteId);

        double rawStart = 0.0;
        double rawLen = 0.0;

        if (! readNumericProperty (*noteObj, { "start" }, rawStart))
            return makeErrorReply ("add_midi_notes_bulk: note missing numeric start: " + noteId);

        if (! readNumericProperty (*noteObj, { "length" }, rawLen))
            return makeErrorReply ("add_midi_notes_bulk: note missing numeric length: " + noteId);

        double startBeats = 0.0;
        double lenBeats = 0.0;

        if (! clipLocalNoteGeometryToBeats (*edit, *midiClip, timeUnit, rawStart, rawLen, startBeats, lenBeats, parseError))
            return makeErrorReply ("add_midi_notes_bulk: " + parseError);

        int pitch = 60;

        if (! tryReadMidiInt (*noteObj, "pitch", pitch))
            return makeErrorReply ("add_midi_notes_bulk: note missing pitch: " + noteId);

        pitch = juce::jlimit (0, 127, pitch);

        int velocity = 100;

        if (tryReadMidiInt (*noteObj, "velocity", velocity))
            velocity = juce::jlimit (1, 127, velocity);

        auto* created = seq.addNote (pitch,
                                     te::BeatPosition::fromBeats (startBeats),
                                     te::BeatDuration::fromBeats (lenBeats),
                                     velocity,
                                     0,
                                     &undo);

        if (created == nullptr)
            return makeErrorReply ("add_midi_notes_bulk: engine refused note: " + noteId);

        created->state.setProperty (kVitNoteId, noteId, &undo);
        ++added;
    }

    edit->invalidateStoredLength();
    edit->dispatchPendingUpdatesSynchronously();
    edit->getTransport().ensureContextAllocated (true);

    auto response = std::make_unique<juce::DynamicObject>();
    response->setProperty ("status", "ok");
    response->setProperty ("message", "MIDI block notes added");
    response->setProperty ("clip_id", clipId);
    response->setProperty ("track_id", trackId);
    response->setProperty ("added_count", added);
    return juce::JSON::toString (juce::var (response.release()));
}

juce::String MidiService::handleMutateMidiNotes (const juce::DynamicObject& object, const juce::String&) const
{
    auto* edit = getEdit != nullptr ? getEdit() : nullptr;

    if (edit == nullptr)
        return makeErrorReply ("No active edit loaded");

    const auto trackId = object.getProperty ("track_id").toString().trim();
    const auto clipId = object.getProperty ("clip_id").toString().trim();

    if (trackId.isEmpty())
        return makeErrorReply ("mutate_midi_notes requires a non-empty track_id");

    if (clipId.isEmpty())
        return makeErrorReply ("mutate_midi_notes requires a non-empty clip_id");

    CommandTimeUnit timeUnit = CommandTimeUnit::seconds;
    juce::String parseError;

    if (! parseTimeUnit (object, timeUnit, parseError))
        return makeErrorReply ("mutate_midi_notes: " + parseError);

    const auto notesVar = object.getProperty ("notes");

    if (! notesVar.isArray())
        return makeErrorReply ("mutate_midi_notes requires notes array");

    auto* arr = notesVar.getArray();

    if (arr == nullptr || arr->isEmpty())
        return makeErrorReply ("mutate_midi_notes requires a non-empty notes array");

    auto* clipTrack = dynamic_cast<te::ClipTrack*> (findTrackByID (*edit, trackId));

    if (clipTrack == nullptr)
        return makeErrorReply ("mutate_midi_notes track not found or is not a clip track: " + trackId);

    auto* clip = findClipByID (*edit, clipId);

    if (clip == nullptr)
        return makeErrorReply ("mutate_midi_notes clip not found for clip_id: " + clipId);

    if (clip->getClipTrack() == nullptr || clip->getClipTrack()->itemID.toString() != trackId)
        return makeErrorReply ("mutate_midi_notes track_id does not match clip's current parent track");

    auto* midiClip = dynamic_cast<te::MidiClip*> (clip);

    if (midiClip == nullptr)
        return makeErrorReply ("mutate_midi_notes clip is not a MIDI clip");

    auto& seq = midiClip->getSequence();
    auto& undo = edit->getUndoManager();
    undo.beginNewTransaction ("Mutate MIDI notes");
    int mutated = 0;
    juce::StringArray missingIds;

    for (const auto& item : *arr)
    {
        auto* noteObj = item.getDynamicObject();

        if (noteObj == nullptr)
            return makeErrorReply ("mutate_midi_notes: each note must be an object");

        const auto noteId = noteObj->getProperty ("id").toString().trim();

        if (noteId.isEmpty())
            return makeErrorReply ("mutate_midi_notes: each note requires a non-empty id");

        auto* note = findMidiNoteByVitId (seq, noteId);

        if (note == nullptr)
        {
            missingIds.addIfNotAlreadyThere (noteId);
            continue;
        }

        double startBeats = note->getStartBeat().inBeats();
        double lenBeats = note->getLengthBeats().inBeats();
        double rawStart = 0.0;
        double rawLen = 0.0;
        const bool hasStart = readNumericProperty (*noteObj, { "start" }, rawStart);
        const bool hasLength = readNumericProperty (*noteObj, { "length" }, rawLen);

        if (hasStart || hasLength)
        {
            if (timeUnit == CommandTimeUnit::beats)
            {
                if (hasStart)
                    startBeats = rawStart;

                if (hasLength)
                    lenBeats = rawLen;

                if (lenBeats <= 0.0)
                    return makeErrorReply ("mutate_midi_notes: length must be greater than zero");
            }
            else
            {
                if (! hasStart || ! hasLength)
                    return makeErrorReply ("mutate_midi_notes: with time_unit seconds, both start and length are required when changing note geometry");

                if (! clipLocalNoteGeometryToBeats (*edit, *midiClip, timeUnit, rawStart, rawLen, startBeats, lenBeats, parseError))
                    return makeErrorReply ("mutate_midi_notes: " + parseError);
            }

            note->setStartAndLength (te::BeatPosition::fromBeats (startBeats),
                                     te::BeatDuration::fromBeats (lenBeats),
                                     &undo);
        }

        int pitch = 0;

        if (tryReadMidiInt (*noteObj, "pitch", pitch))
            note->setNoteNumber (juce::jlimit (0, 127, pitch), &undo);

        int velocity = 0;

        if (tryReadMidiInt (*noteObj, "velocity", velocity))
            note->setVelocity (juce::jlimit (1, 127, velocity), &undo);

        ++mutated;
    }

    if (missingIds.size() == static_cast<int> (arr->size()))
    {
        return makeErrorReply ("mutate_midi_notes: no matching notes (missing vit_note_id? ids: "
                               + missingIds.joinIntoString (", ") + ")");
    }

    edit->invalidateStoredLength();
    edit->dispatchPendingUpdatesSynchronously();
    edit->getTransport().ensureContextAllocated (true);

    auto response = std::make_unique<juce::DynamicObject>();
    response->setProperty ("status", "ok");
    response->setProperty ("message", "MIDI notes mutated");
    response->setProperty ("clip_id", clipId);
    response->setProperty ("track_id", trackId);
    response->setProperty ("mutated_count", mutated);
    response->setProperty ("missing_note_ids", stringArrayToVar (missingIds));
    return juce::JSON::toString (juce::var (response.release()));
}

juce::String MidiService::handleDeleteMidiNotes (const juce::DynamicObject& object, const juce::String&) const
{
    auto* edit = getEdit != nullptr ? getEdit() : nullptr;

    if (edit == nullptr)
        return makeErrorReply ("No active edit loaded");

    const auto trackId = object.getProperty ("track_id").toString().trim();
    const auto clipId = object.getProperty ("clip_id").toString().trim();

    if (trackId.isEmpty())
        return makeErrorReply ("delete_midi_notes requires a non-empty track_id");

    if (clipId.isEmpty())
        return makeErrorReply ("delete_midi_notes requires a non-empty clip_id");

    const auto idsVar = object.getProperty ("note_ids");

    if (! idsVar.isArray())
        return makeErrorReply ("delete_midi_notes requires note_ids array");

    auto* arr = idsVar.getArray();

    if (arr == nullptr || arr->isEmpty())
        return makeErrorReply ("delete_midi_notes requires a non-empty note_ids array");

    auto* clipTrack = dynamic_cast<te::ClipTrack*> (findTrackByID (*edit, trackId));

    if (clipTrack == nullptr)
        return makeErrorReply ("delete_midi_notes track not found or is not a clip track: " + trackId);

    auto* clip = findClipByID (*edit, clipId);

    if (clip == nullptr)
        return makeErrorReply ("delete_midi_notes clip not found for clip_id: " + clipId);

    if (clip->getClipTrack() == nullptr || clip->getClipTrack()->itemID.toString() != trackId)
        return makeErrorReply ("delete_midi_notes track_id does not match clip's current parent track");

    auto* midiClip = dynamic_cast<te::MidiClip*> (clip);

    if (midiClip == nullptr)
        return makeErrorReply ("delete_midi_notes clip is not a MIDI clip");

    auto& seq = midiClip->getSequence();
    auto& undo = edit->getUndoManager();
    undo.beginNewTransaction ("Delete MIDI notes");
    int removed = 0;
    juce::StringArray missingIds;

    for (const auto& item : *arr)
    {
        const auto noteId = item.toString().trim();

        if (noteId.isEmpty())
            continue;

        auto* note = findMidiNoteByVitId (seq, noteId);

        if (note == nullptr)
        {
            missingIds.addIfNotAlreadyThere (noteId);
            continue;
        }

        seq.removeNote (*note, &undo);
        ++removed;
    }

    edit->invalidateStoredLength();
    edit->dispatchPendingUpdatesSynchronously();
    edit->getTransport().ensureContextAllocated (true);

    auto response = std::make_unique<juce::DynamicObject>();
    response->setProperty ("status", "ok");
    response->setProperty ("message", "MIDI notes deleted");
    response->setProperty ("clip_id", clipId);
    response->setProperty ("track_id", trackId);
    response->setProperty ("removed_count", removed);
    response->setProperty ("missing_note_ids", stringArrayToVar (missingIds));
    return juce::JSON::toString (juce::var (response.release()));
}

juce::String MidiService::handleGetMidiClipNotes (const juce::DynamicObject& object, const juce::String&) const
{
    auto* edit = getEdit != nullptr ? getEdit() : nullptr;

    if (edit == nullptr)
        return makeErrorReply ("No active edit loaded");

    const auto clipId = object.getProperty ("clip_id").toString().trim();

    if (clipId.isEmpty())
        return makeErrorReply ("get_midi_clip_notes requires clip_id");

    const auto trackId = object.getProperty ("track_id").toString().trim();

    auto* clip = findClipByID (*edit, clipId);

    if (clip == nullptr)
        return makeErrorReply ("get_midi_clip_notes clip not found for clip_id: " + clipId);

    if (trackId.isNotEmpty())
    {
        auto* clipTrack = clip->getClipTrack();

        if (clipTrack == nullptr || clipTrack->itemID.toString() != trackId)
            return makeErrorReply ("get_midi_clip_notes track_id does not match clip's current parent track");
    }

    auto* midiClip = dynamic_cast<te::MidiClip*> (clip);

    if (midiClip == nullptr)
        return makeErrorReply ("get_midi_clip_notes clip is not a MIDI clip");

    auto& seq = midiClip->getSequence();
    juce::Array<juce::var> notesArr;

    for (auto* n : seq.getNotes())
    {
        if (n == nullptr)
            continue;

        auto row = std::make_unique<juce::DynamicObject>();
        row->setProperty ("id", n->state.getProperty (kVitNoteId).toString());
        row->setProperty ("start", n->getStartBeat().inBeats());
        row->setProperty ("length", n->getLengthBeats().inBeats());
        row->setProperty ("pitch", n->getNoteNumber());
        row->setProperty ("velocity", n->getVelocity());
        notesArr.add (juce::var (row.release()));
    }

    auto response = std::make_unique<juce::DynamicObject>();
    response->setProperty ("status", "ok");
    response->setProperty ("message", "MIDI clip notes");
    response->setProperty ("clip_id", clipId);
    response->setProperty ("track_id", trackId);
    response->setProperty ("time_unit", "beats");
    response->setProperty ("notes", juce::var (notesArr));
    return juce::JSON::toString (juce::var (response.release()));
}

juce::String MidiService::handleGetMidiClipData (const juce::DynamicObject& object, const juce::String&) const
{
    auto* edit = getEdit != nullptr ? getEdit() : nullptr;

    if (edit == nullptr)
        return makeErrorReply ("No active edit loaded");

    const auto clipId = object.getProperty ("clip_id").toString().trim();

    if (clipId.isEmpty())
        return makeErrorReply ("get_midi_clip_data requires clip_id");

    auto* clip = findClipByID (*edit, clipId);

    if (clip == nullptr)
        return makeErrorReply ("get_midi_clip_data clip not found for clip_id: " + clipId);

    auto* midiClip = dynamic_cast<te::MidiClip*> (clip);

    auto response = std::make_unique<juce::DynamicObject>();
    response->setProperty ("status", "ok");
    response->setProperty ("clip_id", clipId);
    response->setProperty ("time_unit", "beats");

    if (midiClip == nullptr)
    {
        response->setProperty ("notes", juce::var (juce::Array<juce::var>()));
        return juce::JSON::toString (juce::var (response.release()));
    }

    const auto pos = clip->getPosition();
    response->setProperty ("start_seconds", pos.getStart().inSeconds());
    response->setProperty ("length_seconds", pos.getLength().inSeconds());
    response->setProperty ("offset_in_source_seconds", pos.getOffset().inSeconds());

    auto& seq = midiClip->getSequence();
    juce::Array<juce::var> notesArr;

    for (auto* n : seq.getNotes())
    {
        if (n == nullptr)
            continue;

        auto row = std::make_unique<juce::DynamicObject>();
        row->setProperty ("id", n->state.getProperty (kVitNoteId).toString());
        row->setProperty ("start", n->getStartBeat().inBeats());
        row->setProperty ("length", n->getLengthBeats().inBeats());
        row->setProperty ("pitch", n->getNoteNumber());
        row->setProperty ("velocity", n->getVelocity());
        notesArr.add (juce::var (row.release()));
    }

    response->setProperty ("notes", juce::var (notesArr));
    return juce::JSON::toString (juce::var (response.release()));
}

juce::String MidiService::handleInsertMidiClip (const juce::DynamicObject& object, const juce::String&) const
{
    auto* edit = getEdit != nullptr ? getEdit() : nullptr;

    if (edit == nullptr)
        return makeErrorReply ("No active edit loaded");

    const auto trackId = object.getProperty ("track_id").toString().trim();

    if (trackId.isEmpty())
        return makeErrorReply ("insert_midi_clip requires track_id");

    CommandTimeUnit timeUnit = CommandTimeUnit::beats;
    juce::String parseError;

    if (! parseTimeUnit (object, timeUnit, parseError))
        return makeErrorReply ("insert_midi_clip: " + parseError);

    double startVal = 0.0;
    readNumericProperty (object, { "start", "start_beat", "start_beats" }, startVal);

    double lengthVal = 4.0;

    if (! readNumericProperty (object, { "length", "length_beat", "length_beats", "initial_length" }, lengthVal) || lengthVal <= 0.0)
        lengthVal = 4.0;

    double startSeconds = 0.0;

    if (! convertTimeValueToSeconds (*edit, timeUnit, startVal, startSeconds, parseError))
        return makeErrorReply ("insert_midi_clip: " + parseError);

    double lengthSeconds = 0.0;

    if (! convertDurationValueToSeconds (*edit, timeUnit, startSeconds, lengthVal, lengthSeconds, parseError))
        return makeErrorReply ("insert_midi_clip: " + parseError);

    lengthSeconds = juce::jmax (lengthSeconds, kMinimumSurvivingClipLengthSeconds);

    auto* clipTrack = dynamic_cast<te::ClipTrack*> (findTrackByID (*edit, trackId));

    if (clipTrack == nullptr)
        return makeErrorReply ("insert_midi_clip track not found or cannot host clips: " + trackId);

    const auto tr = te::TimeRange (te::TimePosition::fromSeconds (startSeconds),
                                   te::TimePosition::fromSeconds (startSeconds + lengthSeconds));

    auto& undo = edit->getUndoManager();
    undo.beginNewTransaction ("Insert MIDI clip");

    const auto newClip = clipTrack->insertMIDIClip (tr, nullptr);

    if (newClip == nullptr)
        return makeErrorReply ("insert_midi_clip: engine refused to create clip");

    edit->invalidateStoredLength();
    edit->dispatchPendingUpdatesSynchronously();
    edit->getTransport().ensureContextAllocated (true);

    auto response = std::make_unique<juce::DynamicObject>();
    response->setProperty ("status", "ok");
    response->setProperty ("message", "MIDI clip inserted");
    response->setProperty ("clip_id", newClip->itemID.toString());
    response->setProperty ("new_clip_id", newClip->itemID.toString());
    response->setProperty ("track_id", trackId);
    response->setProperty ("start_seconds", newClip->getPosition().getStart().inSeconds());
    response->setProperty ("length_seconds", newClip->getPosition().getLength().inSeconds());
    return juce::JSON::toString (juce::var (response.release()));
}

juce::String MidiService::makeStatusReply (const juce::String& status, const juce::String& message)
{
    auto response = std::make_unique<juce::DynamicObject>();
    response->setProperty ("status", status);
    response->setProperty ("message", message);
    return juce::JSON::toString (juce::var (response.release()));
}

juce::String MidiService::makeErrorReply (const juce::String& message)
{
    return makeStatusReply ("error", message);
}

} // namespace vit
