#include "MidiService.h"

#include <algorithm>
#include <cmath>
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

bool parseTimeUnitWithDefault (const juce::DynamicObject& object, CommandTimeUnit defaultUnit, CommandTimeUnit& out, juce::String& error)
{
    const auto raw = object.getProperty ("time_unit").toString().trim();

    if (raw.isEmpty())
    {
        out = defaultUnit;
        return true;
    }

    return parseTimeUnit (object, out, error);
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

bool clipLocalPositionToBeats (te::Edit& edit,
                               te::MidiClip& clip,
                               CommandTimeUnit unit,
                               double value,
                               double& outBeats,
                               juce::String& error)
{
    if (value < 0.0)
    {
        error = "note start must be greater than or equal to zero";
        return false;
    }

    if (unit == CommandTimeUnit::beats)
    {
        outBeats = value;
        return true;
    }

    const auto clipStart = clip.getPosition().getStart();
    const auto clipStartBeat = edit.tempoSequence.toBeats (clipStart);
    const auto noteStartTime = clipStart + te::TimeDuration::fromSeconds (value);
    outBeats = edit.tempoSequence.toBeats (noteStartTime).inBeats() - clipStartBeat.inBeats();
    return true;
}

bool clipLocalDurationToBeatsAt (te::Edit& edit,
                                 te::MidiClip& clip,
                                 double startBeats,
                                 double durationSeconds,
                                 double& outLengthBeats,
                                 juce::String& error)
{
    if (durationSeconds <= 0.0)
    {
        error = "note length in seconds must be greater than zero";
        return false;
    }

    const auto clipStartBeat = edit.tempoSequence.toBeats (clip.getPosition().getStart());
    const auto absoluteStartBeat = clipStartBeat + te::BeatDuration::fromBeats (startBeats);
    const auto startTime = edit.tempoSequence.toTime (absoluteStartBeat);
    const auto endTime = startTime + te::TimeDuration::fromSeconds (durationSeconds);
    outLengthBeats = (edit.tempoSequence.toBeats (endTime) - absoluteStartBeat).inBeats();

    if (outLengthBeats <= 0.0)
    {
        error = "note length conversion produced non-positive length";
        return false;
    }

    return true;
}

juce::String ensureMidiNoteId (te::MidiNote& note, juce::UndoManager* undoManager)
{
    auto id = note.state.getProperty (kVitNoteId).toString().trim();

    if (id.isEmpty())
    {
        id = "note_" + juce::Uuid().toString();
        note.state.setProperty (kVitNoteId, id, undoManager);
    }

    return id;
}

juce::var midiNoteToVar (te::MidiNote& note, te::MidiList& list, juce::UndoManager* undoManager)
{
    auto row = std::make_unique<juce::DynamicObject>();
    row->setProperty ("id", ensureMidiNoteId (note, undoManager));
    row->setProperty ("start", note.getStartBeat().inBeats());
    row->setProperty ("length", note.getLengthBeats().inBeats());
    row->setProperty ("pitch", note.getNoteNumber());
    row->setProperty ("velocity", note.getVelocity());
    row->setProperty ("channel", list.getMidiChannel().getChannelNumber());
    return juce::var (row.release());
}

juce::var midiNotesToVar (te::MidiList& list, juce::UndoManager* undoManager)
{
    juce::Array<juce::var> notes;
    for (auto* note : list.getNotes())
        if (note != nullptr)
            notes.add (midiNoteToVar (*note, list, undoManager));
    return juce::var (notes);
}

bool readOperationRegionBeats (te::Edit& edit,
                               te::MidiClip& clip,
                               CommandTimeUnit unit,
                               const juce::DynamicObject& object,
                               double& outStartBeats,
                               double& outLengthBeats,
                               juce::String& error)
{
    double startVal = 0.0;
    double lengthVal = 0.0;

    if (! readNumericProperty (object, { "start", "start_beat", "start_beats", "region_start", "region_start_beats" }, startVal))
    {
        error = "operation region requires numeric start";
        return false;
    }

    if (! readNumericProperty (object, { "length", "length_beat", "length_beats", "duration", "duration_beats", "region_length", "region_length_beats" }, lengthVal))
    {
        error = "operation region requires numeric length";
        return false;
    }

    return clipLocalNoteGeometryToBeats (edit, clip, unit, startVal, lengthVal, outStartBeats, outLengthBeats, error);
}

bool operationHasRegion (const juce::DynamicObject& object)
{
    double ignored = 0.0;
    return readNumericProperty (object, { "start", "start_beat", "start_beats", "region_start", "region_start_beats" }, ignored)
        && readNumericProperty (object, { "length", "length_beat", "length_beats", "duration", "duration_beats", "region_length", "region_length_beats" }, ignored);
}

juce::StringArray readOperationNoteIds (const juce::DynamicObject& object)
{
    juce::StringArray ids;

    for (const auto& key : { "note_id", "id" })
    {
        const auto value = object.getProperty (key).toString().trim();
        if (value.isNotEmpty())
            ids.addIfNotAlreadyThere (value);
    }

    const auto idsVar = object.getProperty ("note_ids");
    if (idsVar.isArray())
    {
        if (auto* arr = idsVar.getArray())
        {
            for (const auto& item : *arr)
            {
                const auto id = item.toString().trim();
                if (id.isNotEmpty())
                    ids.addIfNotAlreadyThere (id);
            }
        }
    }

    return ids;
}

bool vectorContainsNote (const std::vector<te::MidiNote*>& notes, te::MidiNote* note)
{
    return std::find (notes.begin(), notes.end(), note) != notes.end();
}

bool midiNoteOverlapsRegion (te::MidiNote& note, double regionStartBeats, double regionLengthBeats)
{
    const auto regionEndBeats = regionStartBeats + regionLengthBeats;
    const auto noteStartBeats = note.getStartBeat().inBeats();
    const auto noteEndBeats = noteStartBeats + note.getLengthBeats().inBeats();

    return noteStartBeats < regionEndBeats && noteEndBeats > regionStartBeats;
}

std::vector<te::MidiNote*> collectTargetNotes (te::Edit& edit,
                                               te::MidiClip& clip,
                                               te::MidiList& list,
                                               CommandTimeUnit unit,
                                               const juce::DynamicObject& object,
                                               juce::StringArray& missingIds,
                                               juce::String& error)
{
    std::vector<te::MidiNote*> notes;
    const auto ids = readOperationNoteIds (object);

    if (! ids.isEmpty())
    {
        for (const auto& id : ids)
        {
            if (auto* note = findMidiNoteByVitId (list, id))
            {
                if (! vectorContainsNote (notes, note))
                    notes.push_back (note);
            }
            else
            {
                missingIds.addIfNotAlreadyThere (id);
            }
        }

        return notes;
    }

    if (operationHasRegion (object))
    {
        double startBeats = 0.0;
        double lengthBeats = 0.0;

        if (! readOperationRegionBeats (edit, clip, unit, object, startBeats, lengthBeats, error))
            return notes;

        const auto endBeats = startBeats + lengthBeats;
        for (auto* note : list.getNotes())
        {
            if (note == nullptr)
                continue;

            const auto noteStart = note->getStartBeat().inBeats();
            if (noteStart >= startBeats && noteStart < endBeats)
                notes.push_back (note);
        }

        return notes;
    }

    error = "operation requires note_id/note_ids or start+length region";
    return notes;
}

bool parseQuantizeGridBeats (const juce::String& rawGrid, double& outGridBeats)
{
    const auto grid = rawGrid.trim().toLowerCase();

    if (grid == "1/4" || grid == "quarter" || grid == "quarter_note")
    {
        outGridBeats = 1.0;
        return true;
    }

    if (grid == "1/8" || grid == "eighth" || grid == "eighth_note")
    {
        outGridBeats = 0.5;
        return true;
    }

    if (grid == "1/16" || grid == "sixteenth" || grid == "sixteenth_note")
    {
        outGridBeats = 0.25;
        return true;
    }

    if (grid == "1/32" || grid == "thirty_second" || grid == "thirty_second_note")
    {
        outGridBeats = 0.125;
        return true;
    }

    return false;
}

juce::String readPatchUndoLabel (const juce::DynamicObject& object)
{
    const auto requested = object.getProperty ("undo_label").toString().trim();
    return requested.isNotEmpty() ? requested : juce::String ("Agent: MIDI note patch");
}

juce::String readRequestedNoteId (const juce::DynamicObject& noteObj)
{
    auto noteId = noteObj.getProperty ("id").toString().trim();
    if (noteId.isEmpty())
        noteId = noteObj.getProperty ("note_id").toString().trim();
    return noteId;
}

juce::StringArray noteIdsOverlappingRegion (te::MidiList& list, double regionStartBeats, double regionLengthBeats)
{
    juce::StringArray ids;

    for (auto* note : list.getNotes())
    {
        if (note == nullptr)
            continue;

        if (! midiNoteOverlapsRegion (*note, regionStartBeats, regionLengthBeats))
            continue;

        const auto id = note->state.getProperty (kVitNoteId).toString().trim();
        if (id.isNotEmpty())
            ids.addIfNotAlreadyThere (id);
    }

    return ids;
}

bool validatePatchInsertNoteObject (te::Edit& edit,
                                    te::MidiClip& clip,
                                    te::MidiList& list,
                                    CommandTimeUnit unit,
                                    const juce::DynamicObject& noteObj,
                                    const juce::StringArray& allowedExistingIds,
                                    juce::StringArray& reservedInsertedIds,
                                    juce::String& error)
{
    const auto noteId = readRequestedNoteId (noteObj);

    if (noteId.isNotEmpty())
    {
        if (reservedInsertedIds.contains (noteId))
        {
            error = "duplicate note id: " + noteId;
            return false;
        }

        if (findMidiNoteByVitId (list, noteId) != nullptr && ! allowedExistingIds.contains (noteId))
        {
            error = "duplicate note id: " + noteId;
            return false;
        }
    }

    double rawStart = 0.0;
    double rawLen = 0.0;

    if (! readNumericProperty (noteObj, { "start", "start_beat", "start_beats", "relative_start", "relative_start_beats" }, rawStart))
    {
        error = noteId.isNotEmpty() ? "note missing numeric start: " + noteId : "note missing numeric start";
        return false;
    }

    if (! readNumericProperty (noteObj, { "length", "length_beat", "length_beats", "duration", "duration_beats" }, rawLen))
    {
        error = noteId.isNotEmpty() ? "note missing numeric length: " + noteId : "note missing numeric length";
        return false;
    }

    double startBeats = 0.0;
    double lenBeats = 0.0;
    if (! clipLocalNoteGeometryToBeats (edit, clip, unit, rawStart, rawLen, startBeats, lenBeats, error))
        return false;

    int pitch = 60;
    if (! tryReadMidiInt (noteObj, "pitch", pitch))
    {
        error = noteId.isNotEmpty() ? "note missing pitch: " + noteId : "note missing pitch";
        return false;
    }

    if (noteId.isNotEmpty())
        reservedInsertedIds.add (noteId);

    return true;
}

bool validateMidiPatchOperations (te::Edit& edit,
                                  te::MidiClip& clip,
                                  te::MidiList& list,
                                  CommandTimeUnit timeUnit,
                                  const juce::Array<juce::var>& operations,
                                  juce::String& error)
{
    juce::StringArray reservedInsertedIds;

    for (const auto& item : operations)
    {
        auto* opObj = item.getDynamicObject();

        if (opObj == nullptr)
        {
            error = "each operation must be an object";
            return false;
        }

        const auto op = opObj->getProperty ("op").toString().trim().toLowerCase();

        if (op.isEmpty())
        {
            error = "operation missing op";
            return false;
        }

        if (op == "insert_note")
        {
            juce::String noteError;
            if (! validatePatchInsertNoteObject (edit, clip, list, timeUnit, *opObj, {}, reservedInsertedIds, noteError))
            {
                error = "insert_note: " + noteError;
                return false;
            }

            continue;
        }

        if (op == "replace_region")
        {
            double regionStartBeats = 0.0;
            double regionLengthBeats = 0.0;
            juce::String regionError;

            if (! readOperationRegionBeats (edit, clip, timeUnit, *opObj, regionStartBeats, regionLengthBeats, regionError))
            {
                error = "replace_region: " + regionError;
                return false;
            }

            const auto notesVar = opObj->getProperty ("notes");
            if (! notesVar.isArray())
            {
                error = "replace_region requires notes array";
                return false;
            }

            auto* notes = notesVar.getArray();
            if (notes == nullptr)
            {
                error = "replace_region requires notes array";
                return false;
            }

            const auto allowedExistingIds = noteIdsOverlappingRegion (list, regionStartBeats, regionLengthBeats);

            for (const auto& noteItem : *notes)
            {
                auto* noteObj = noteItem.getDynamicObject();
                if (noteObj == nullptr)
                {
                    error = "replace_region: each note must be an object";
                    return false;
                }

                juce::String noteError;
                if (! validatePatchInsertNoteObject (edit, clip, list, timeUnit, *noteObj, allowedExistingIds, reservedInsertedIds, noteError))
                {
                    error = "replace_region: " + noteError;
                    return false;
                }
            }

            continue;
        }

        juce::StringArray missingIds;
        juce::String targetError;
        collectTargetNotes (edit, clip, list, timeUnit, *opObj, missingIds, targetError);

        if (! targetError.isEmpty())
        {
            error = op + ": " + targetError;
            return false;
        }

        if (op == "delete_note")
            continue;

        if (op == "move_note")
        {
            double delta = 0.0;
            const bool hasDelta = readNumericProperty (*opObj, { "delta", "delta_beats", "offset", "offset_beats" }, delta);
            double newStart = 0.0;
            const bool hasStart = readNumericProperty (*opObj, { "start", "start_beat", "start_beats", "new_start", "new_start_beat", "new_start_beats" }, newStart);

            if (! hasDelta && ! hasStart)
            {
                error = "move_note requires delta or start";
                return false;
            }

            if (hasStart)
            {
                double convertedStart = 0.0;
                juce::String startError;
                if (! clipLocalPositionToBeats (edit, clip, timeUnit, newStart, convertedStart, startError))
                {
                    error = "move_note: " + startError;
                    return false;
                }
            }

            continue;
        }

        if (op == "resize_note")
        {
            double newLength = 0.0;
            if (! readNumericProperty (*opObj, { "length", "length_beat", "length_beats", "new_length", "new_length_beat", "new_length_beats", "duration", "duration_beats" }, newLength))
            {
                error = "resize_note requires length";
                return false;
            }

            if (newLength <= 0.0)
            {
                error = "resize_note length must be greater than zero";
                return false;
            }

            continue;
        }

        if (op == "transpose_note")
        {
            int semitones = 0;
            if (! tryReadMidiInt (*opObj, "semitones", semitones))
            {
                error = "transpose_note requires semitones";
                return false;
            }

            continue;
        }

        if (op == "set_velocity")
        {
            int velocity = 0;
            if (! tryReadMidiInt (*opObj, "velocity", velocity))
            {
                error = "set_velocity requires velocity";
                return false;
            }

            continue;
        }

        if (op == "quantize_region")
        {
            double gridBeats = 0.0;
            if (! parseQuantizeGridBeats (opObj->getProperty ("grid").toString(), gridBeats))
            {
                error = "quantize_region grid must be one of 1/4, 1/8, 1/16, 1/32";
                return false;
            }

            continue;
        }

        error = "unknown operation: " + op;
        return false;
    }

    return true;
}

te::MidiNote* insertNoteFromObject (te::Edit& edit,
                                    te::MidiClip& clip,
                                    te::MidiList& list,
                                    CommandTimeUnit unit,
                                    const juce::DynamicObject& noteObj,
                                    double startOffsetBeats,
                                    juce::UndoManager& undo,
                                     juce::String& noteId,
                                     juce::String& error)
{
    noteId = readRequestedNoteId (noteObj);
    if (noteId.isEmpty())
        noteId = "note_" + juce::Uuid().toString();

    if (findMidiNoteByVitId (list, noteId) != nullptr)
    {
        error = "duplicate note id: " + noteId;
        return nullptr;
    }

    double rawStart = 0.0;
    double rawLen = 0.0;

    if (! readNumericProperty (noteObj, { "start", "start_beat", "start_beats", "relative_start", "relative_start_beats" }, rawStart))
    {
        error = "note missing numeric start: " + noteId;
        return nullptr;
    }

    if (! readNumericProperty (noteObj, { "length", "length_beat", "length_beats", "duration", "duration_beats" }, rawLen))
    {
        error = "note missing numeric length: " + noteId;
        return nullptr;
    }

    double startBeats = 0.0;
    double lenBeats = 0.0;
    if (! clipLocalNoteGeometryToBeats (edit, clip, unit, rawStart, rawLen, startBeats, lenBeats, error))
        return nullptr;

    startBeats += startOffsetBeats;

    int pitch = 60;
    if (! tryReadMidiInt (noteObj, "pitch", pitch))
    {
        error = "note missing pitch: " + noteId;
        return nullptr;
    }

    pitch = juce::jlimit (0, 127, pitch);

    int velocity = 100;
    if (tryReadMidiInt (noteObj, "velocity", velocity))
        velocity = juce::jlimit (1, 127, velocity);

    auto* created = list.addNote (pitch,
                                  te::BeatPosition::fromBeats (startBeats),
                                  te::BeatDuration::fromBeats (lenBeats),
                                  velocity,
                                  0,
                                  &undo);

    if (created == nullptr)
    {
        error = "engine refused note: " + noteId;
        return nullptr;
    }

    created->state.setProperty (kVitNoteId, noteId, &undo);
    return created;
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
    juce::StringArray addedIds;

    for (const auto& item : *arr)
    {
        auto* noteObj = item.getDynamicObject();

        if (noteObj == nullptr)
            return makeErrorReply ("add_midi_notes: each note must be an object");

        auto noteId = readRequestedNoteId (*noteObj);
        if (noteId.isEmpty())
            noteId = juce::String ("note_") + juce::Uuid().toString();

        if (findMidiNoteByVitId (seq, noteId) != nullptr)
            return makeErrorReply ("add_midi_notes: duplicate note id: " + noteId);

        double rawStart = 0.0;
        double rawLen = 0.0;

        if (! readNumericProperty (*noteObj, { "start", "start_beat", "start_beats" }, rawStart))
            return makeErrorReply ("add_midi_notes: note missing numeric start: " + noteId);

        if (! readNumericProperty (*noteObj, { "length", "length_beat", "length_beats", "duration", "duration_beats" }, rawLen))
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
        addedIds.addIfNotAlreadyThere (noteId);
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
    response->setProperty ("inserted_count", added);
    response->setProperty ("inserted_note_ids", stringArrayToVar (addedIds));
    response->setProperty ("notes", midiNotesToVar (seq, &undo));
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
    juce::StringArray addedIds;

    for (const auto& item : *arr)
    {
        auto* noteObj = item.getDynamicObject();

        if (noteObj == nullptr)
            return makeErrorReply ("add_midi_notes_bulk: each note must be an object");

        auto noteId = readRequestedNoteId (*noteObj);

        if (noteId.isEmpty())
            noteId = juce::String ("blk_") + juce::Uuid().toString();

        if (findMidiNoteByVitId (seq, noteId) != nullptr)
            return makeErrorReply ("add_midi_notes_bulk: duplicate note id: " + noteId);

        double rawStart = 0.0;
        double rawLen = 0.0;

        if (! readNumericProperty (*noteObj, { "start", "start_beat", "start_beats" }, rawStart))
            return makeErrorReply ("add_midi_notes_bulk: note missing numeric start: " + noteId);

        if (! readNumericProperty (*noteObj, { "length", "length_beat", "length_beats", "duration", "duration_beats" }, rawLen))
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
        addedIds.addIfNotAlreadyThere (noteId);
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
    response->setProperty ("inserted_count", added);
    response->setProperty ("inserted_note_ids", stringArrayToVar (addedIds));
    response->setProperty ("notes", midiNotesToVar (seq, &undo));
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
    juce::StringArray mutatedIds;

    for (const auto& item : *arr)
    {
        auto* noteObj = item.getDynamicObject();

        if (noteObj == nullptr)
            return makeErrorReply ("mutate_midi_notes: each note must be an object");

        const auto noteId = readRequestedNoteId (*noteObj);

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
        const bool hasStart = readNumericProperty (*noteObj, { "start", "start_beat", "start_beats" }, rawStart);
        const bool hasLength = readNumericProperty (*noteObj, { "length", "length_beat", "length_beats", "duration", "duration_beats" }, rawLen);

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

        mutatedIds.addIfNotAlreadyThere (noteId);
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
    response->setProperty ("mutated_note_ids", stringArrayToVar (mutatedIds));
    response->setProperty ("missing_note_ids", stringArrayToVar (missingIds));
    response->setProperty ("notes", midiNotesToVar (seq, &undo));
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
    juce::StringArray removedIds;

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

        removedIds.addIfNotAlreadyThere (noteId);
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
    response->setProperty ("deleted_count", removed);
    response->setProperty ("deleted_note_ids", stringArrayToVar (removedIds));
    response->setProperty ("missing_note_ids", stringArrayToVar (missingIds));
    response->setProperty ("notes", midiNotesToVar (seq, &undo));
    return juce::JSON::toString (juce::var (response.release()));
}

juce::String MidiService::handleApplyMidiNotePatch (const juce::DynamicObject& object, const juce::String&) const
{
    auto* edit = getEdit != nullptr ? getEdit() : nullptr;

    if (edit == nullptr)
        return makeErrorReply ("No active edit loaded");

    const auto clipId = object.getProperty ("clip_id").toString().trim();

    if (clipId.isEmpty())
        return makeErrorReply ("apply_midi_note_patch requires clip_id");

    auto* clip = findClipByID (*edit, clipId);

    if (clip == nullptr)
        return makeErrorReply ("apply_midi_note_patch clip not found for clip_id: " + clipId);

    auto* clipTrack = clip->getClipTrack();

    if (clipTrack == nullptr)
        return makeErrorReply ("apply_midi_note_patch clip has no parent clip track");

    juce::String trackId = object.getProperty ("track_id").toString().trim();
    if (trackId.isEmpty())
        trackId = clipTrack->itemID.toString();
    else if (clipTrack->itemID.toString() != trackId)
        return makeErrorReply ("apply_midi_note_patch track_id does not match clip's current parent track");

    auto* midiClip = dynamic_cast<te::MidiClip*> (clip);

    if (midiClip == nullptr)
        return makeErrorReply ("apply_midi_note_patch clip is not a MIDI clip");

    CommandTimeUnit timeUnit = CommandTimeUnit::beats;
    juce::String parseError;

    if (! parseTimeUnitWithDefault (object, CommandTimeUnit::beats, timeUnit, parseError))
        return makeErrorReply ("apply_midi_note_patch: " + parseError);

    const auto operationsVar = object.getProperty ("operations");

    if (! operationsVar.isArray())
        return makeErrorReply ("apply_midi_note_patch requires operations array");

    auto* operations = operationsVar.getArray();

    if (operations == nullptr || operations->isEmpty())
        return makeErrorReply ("apply_midi_note_patch requires a non-empty operations array");

    auto& seq = midiClip->getSequence();
    juce::String validationError;
    if (! validateMidiPatchOperations (*edit, *midiClip, seq, timeUnit, *operations, validationError))
        return makeErrorReply ("apply_midi_note_patch: " + validationError);

    auto& undo = edit->getUndoManager();
    undo.beginNewTransaction (readPatchUndoLabel (object));

    int inserted = 0;
    int deleted = 0;
    int mutated = 0;
    int quantized = 0;
    int replaced = 0;
    juce::StringArray insertedIds;
    juce::StringArray deletedIds;
    juce::StringArray mutatedIds;
    juce::StringArray quantizedIds;
    juce::StringArray missingIds;
    juce::StringArray ignoredChannels;

    for (const auto& item : *operations)
    {
        auto* opObj = item.getDynamicObject();

        if (opObj == nullptr)
            return makeErrorReply ("apply_midi_note_patch: each operation must be an object");

        const auto op = opObj->getProperty ("op").toString().trim().toLowerCase();

        if (op.isEmpty())
            return makeErrorReply ("apply_midi_note_patch: operation missing op");

        if (opObj->hasProperty ("channel"))
            ignoredChannels.addIfNotAlreadyThere (opObj->getProperty ("channel").toString());

        if (op == "insert_note")
        {
            juce::String noteId;
            juce::String error;
            auto* created = insertNoteFromObject (*edit, *midiClip, seq, timeUnit, *opObj, 0.0, undo, noteId, error);

            if (created == nullptr)
                return makeErrorReply ("apply_midi_note_patch insert_note: " + error);

            insertedIds.addIfNotAlreadyThere (noteId);
            ++inserted;
            continue;
        }

        if (op == "replace_region")
        {
            double regionStartBeats = 0.0;
            double regionLengthBeats = 0.0;
            juce::String error;

            if (! readOperationRegionBeats (*edit, *midiClip, timeUnit, *opObj, regionStartBeats, regionLengthBeats, error))
                return makeErrorReply ("apply_midi_note_patch replace_region: " + error);

            std::vector<te::MidiNote*> toRemove;

            for (auto* note : seq.getNotes())
            {
                if (note == nullptr)
                    continue;

                if (midiNoteOverlapsRegion (*note, regionStartBeats, regionLengthBeats))
                    toRemove.push_back (note);
            }

            for (auto* note : toRemove)
            {
                deletedIds.addIfNotAlreadyThere (ensureMidiNoteId (*note, &undo));
                seq.removeNote (*note, &undo);
                ++deleted;
            }

            const auto notesVar = opObj->getProperty ("notes");
            if (! notesVar.isArray())
                return makeErrorReply ("apply_midi_note_patch replace_region requires notes array");

            auto* notes = notesVar.getArray();
            if (notes == nullptr)
                return makeErrorReply ("apply_midi_note_patch replace_region requires notes array");

            for (const auto& noteItem : *notes)
            {
                auto* noteObj = noteItem.getDynamicObject();
                if (noteObj == nullptr)
                    return makeErrorReply ("apply_midi_note_patch replace_region: each note must be an object");

                juce::String noteId;
                auto* created = insertNoteFromObject (*edit, *midiClip, seq, timeUnit, *noteObj, regionStartBeats, undo, noteId, error);

                if (created == nullptr)
                    return makeErrorReply ("apply_midi_note_patch replace_region: " + error);

                insertedIds.addIfNotAlreadyThere (noteId);
                ++inserted;
            }

            ++replaced;
            continue;
        }

        juce::String targetError;
        auto targets = collectTargetNotes (*edit, *midiClip, seq, timeUnit, *opObj, missingIds, targetError);

        if (! targetError.isEmpty())
            return makeErrorReply ("apply_midi_note_patch " + op + ": " + targetError);

        if (op == "delete_note")
        {
            for (auto* note : targets)
            {
                if (note == nullptr)
                    continue;

                const auto noteId = ensureMidiNoteId (*note, &undo);
                deletedIds.addIfNotAlreadyThere (noteId);
                seq.removeNote (*note, &undo);
                ++deleted;
            }

            continue;
        }

        if (op == "move_note")
        {
            double delta = 0.0;
            const bool hasDelta = readNumericProperty (*opObj, { "delta", "delta_beats", "offset", "offset_beats" }, delta);
            double newStart = 0.0;
            const bool hasStart = readNumericProperty (*opObj, { "start", "start_beat", "start_beats", "new_start", "new_start_beat", "new_start_beats" }, newStart);

            if (! hasDelta && ! hasStart)
                return makeErrorReply ("apply_midi_note_patch move_note requires delta or start");

            for (auto* note : targets)
            {
                if (note == nullptr)
                    continue;

                auto startBeats = note->getStartBeat().inBeats();
                if (hasStart)
                {
                    double convertedStart = 0.0;
                    juce::String error;
                    if (! clipLocalPositionToBeats (*edit, *midiClip, timeUnit, newStart, convertedStart, error))
                        return makeErrorReply ("apply_midi_note_patch move_note: " + error);
                    startBeats = convertedStart;
                }
                else
                {
                    startBeats += delta;
                }

                if (startBeats < 0.0)
                    startBeats = 0.0;

                note->setStartAndLength (te::BeatPosition::fromBeats (startBeats), note->getLengthBeats(), &undo);
                mutatedIds.addIfNotAlreadyThere (ensureMidiNoteId (*note, &undo));
                ++mutated;
            }

            continue;
        }

        if (op == "resize_note")
        {
            double newLength = 0.0;
            if (! readNumericProperty (*opObj, { "length", "length_beat", "length_beats", "new_length", "new_length_beat", "new_length_beats", "duration", "duration_beats" }, newLength))
                return makeErrorReply ("apply_midi_note_patch resize_note requires length");

            for (auto* note : targets)
            {
                if (note == nullptr)
                    continue;

                const auto startBeats = note->getStartBeat().inBeats();
                double lenBeats = newLength;
                if (timeUnit == CommandTimeUnit::seconds)
                {
                    juce::String error;
                    if (! clipLocalDurationToBeatsAt (*edit, *midiClip, startBeats, newLength, lenBeats, error))
                        return makeErrorReply ("apply_midi_note_patch resize_note: " + error);
                }

                if (lenBeats <= 0.0)
                    return makeErrorReply ("apply_midi_note_patch resize_note length must be greater than zero");

                note->setStartAndLength (te::BeatPosition::fromBeats (startBeats), te::BeatDuration::fromBeats (lenBeats), &undo);
                mutatedIds.addIfNotAlreadyThere (ensureMidiNoteId (*note, &undo));
                ++mutated;
            }

            continue;
        }

        if (op == "transpose_note")
        {
            int semitones = 0;
            if (! tryReadMidiInt (*opObj, "semitones", semitones))
                return makeErrorReply ("apply_midi_note_patch transpose_note requires semitones");

            for (auto* note : targets)
            {
                if (note == nullptr)
                    continue;

                note->setNoteNumber (juce::jlimit (0, 127, note->getNoteNumber() + semitones), &undo);
                mutatedIds.addIfNotAlreadyThere (ensureMidiNoteId (*note, &undo));
                ++mutated;
            }

            continue;
        }

        if (op == "set_velocity")
        {
            int velocity = 0;
            if (! tryReadMidiInt (*opObj, "velocity", velocity))
                return makeErrorReply ("apply_midi_note_patch set_velocity requires velocity");

            velocity = juce::jlimit (1, 127, velocity);

            for (auto* note : targets)
            {
                if (note == nullptr)
                    continue;

                note->setVelocity (velocity, &undo);
                mutatedIds.addIfNotAlreadyThere (ensureMidiNoteId (*note, &undo));
                ++mutated;
            }

            continue;
        }

        if (op == "quantize_region")
        {
            double gridBeats = 0.0;
            if (! parseQuantizeGridBeats (opObj->getProperty ("grid").toString(), gridBeats))
                return makeErrorReply ("apply_midi_note_patch quantize_region grid must be one of 1/4, 1/8, 1/16, 1/32");

            for (auto* note : targets)
            {
                if (note == nullptr)
                    continue;

                const auto current = note->getStartBeat().inBeats();
                const auto rounded = std::round (current / gridBeats) * gridBeats;
                note->setStartAndLength (te::BeatPosition::fromBeats (juce::jmax (0.0, rounded)), note->getLengthBeats(), &undo);
                quantizedIds.addIfNotAlreadyThere (ensureMidiNoteId (*note, &undo));
                ++quantized;
            }

            continue;
        }

        return makeErrorReply ("apply_midi_note_patch unknown operation: " + op);
    }

    edit->invalidateStoredLength();
    edit->dispatchPendingUpdatesSynchronously();
    edit->getTransport().ensureContextAllocated (true);

    auto response = std::make_unique<juce::DynamicObject>();
    response->setProperty ("status", "ok");
    response->setProperty ("message", "MIDI note patch applied");
    response->setProperty ("clip_id", clipId);
    response->setProperty ("track_id", trackId);
    response->setProperty ("time_unit", "beats");
    response->setProperty ("operation_count", static_cast<int> (operations->size()));
    response->setProperty ("inserted_count", inserted);
    response->setProperty ("deleted_count", deleted);
    response->setProperty ("mutated_count", mutated);
    response->setProperty ("quantized_count", quantized);
    response->setProperty ("replaced_region_count", replaced);
    response->setProperty ("inserted_note_ids", stringArrayToVar (insertedIds));
    response->setProperty ("deleted_note_ids", stringArrayToVar (deletedIds));
    response->setProperty ("mutated_note_ids", stringArrayToVar (mutatedIds));
    response->setProperty ("quantized_note_ids", stringArrayToVar (quantizedIds));
    response->setProperty ("missing_note_ids", stringArrayToVar (missingIds));
    response->setProperty ("ignored_channels", stringArrayToVar (ignoredChannels));
    response->setProperty ("notes", midiNotesToVar (seq, &undo));
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

    auto& undo = edit->getUndoManager();

    for (auto* n : seq.getNotes())
        if (n != nullptr)
            notesArr.add (midiNoteToVar (*n, seq, &undo));

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

    auto& undo = edit->getUndoManager();

    for (auto* n : seq.getNotes())
        if (n != nullptr)
            notesArr.add (midiNoteToVar (*n, seq, &undo));

    response->setProperty ("notes", juce::var (notesArr));
    return juce::JSON::toString (juce::var (response.release()));
}

juce::String MidiService::handleImportMidiToTrack (const juce::DynamicObject& object, const juce::String&) const
{
    auto* edit = getEdit != nullptr ? getEdit() : nullptr;

    if (edit == nullptr)
        return makeErrorReply ("No active edit loaded");

    const auto trackId = object.getProperty ("track_id").toString().trim();
    const auto filePath = object.getProperty ("file_path").toString().trim();

    if (trackId.isEmpty())
        return makeErrorReply ("import_midi_to_track requires track_id");

    if (filePath.isEmpty())
        return makeErrorReply ("import_midi_to_track requires file_path");

    double startBeats = 0.0;
    readNumericProperty (object, { "start_time_beats", "start_beats", "start_beat", "start" }, startBeats);

    if (startBeats < 0.0)
        return makeErrorReply ("import_midi_to_track start_time_beats must be greater than or equal to zero");

    auto sourceFile = juce::File (filePath);

    if (! sourceFile.existsAsFile())
        return makeErrorReply ("MIDI file does not exist: " + filePath);

    juce::FileInputStream stream (sourceFile);

    if (! stream.openedOk())
        return makeErrorReply ("Could not open MIDI file: " + filePath);

    juce::MidiFile midiFile;

    if (! midiFile.readFrom (stream))
        return makeErrorReply ("Unsupported or unreadable MIDI file: " + filePath);

    const auto timeFormat = midiFile.getTimeFormat();

    if (timeFormat <= 0)
        return makeErrorReply ("import_midi_to_track only supports PPQ Standard MIDI Files");

    const double ticksPerQuarter = static_cast<double> (timeFormat);
    auto* clipTrack = dynamic_cast<te::ClipTrack*> (findTrackByID (*edit, trackId));

    if (clipTrack == nullptr)
        return makeErrorReply ("import_midi_to_track track not found or cannot host clips: " + trackId);

    struct ImportedNote
    {
        int pitch = 60;
        int velocity = 100;
        double startBeat = 0.0;
        double lengthBeats = 0.0;
    };

    std::vector<ImportedNote> importedNotes;
    double maxEndBeats = 0.0;

    for (int trackIndex = 0; trackIndex < midiFile.getNumTracks(); ++trackIndex)
    {
        const auto* sourceSequence = midiFile.getTrack (trackIndex);

        if (sourceSequence == nullptr)
            continue;

        juce::MidiMessageSequence sequence (*sourceSequence);
        sequence.updateMatchedPairs();

        for (int eventIndex = 0; eventIndex < sequence.getNumEvents(); ++eventIndex)
        {
            auto* event = sequence.getEventPointer (eventIndex);

            if (event == nullptr || ! event->message.isNoteOn() || event->noteOffObject == nullptr)
                continue;

            const auto startTick = event->message.getTimeStamp();
            const auto endTick = event->noteOffObject->message.getTimeStamp();

            if (endTick <= startTick)
                continue;

            ImportedNote note;
            note.pitch = juce::jlimit (0, 127, event->message.getNoteNumber());
            note.velocity = juce::jlimit (1, 127, static_cast<int> (event->message.getVelocity()));
            note.startBeat = juce::jmax (0.0, startTick / ticksPerQuarter);
            note.lengthBeats = juce::jmax (0.0, (endTick - startTick) / ticksPerQuarter);

            if (note.lengthBeats <= 0.0)
                continue;

            maxEndBeats = juce::jmax (maxEndBeats, note.startBeat + note.lengthBeats);
            importedNotes.push_back (note);
        }
    }

    if (importedNotes.empty())
        return makeErrorReply ("MIDI file contains no importable notes: " + filePath);

    const double lengthBeats = juce::jmax (maxEndBeats, 4.0);
    double startSeconds = 0.0;
    juce::String parseError;

    if (! convertTimeValueToSeconds (*edit, CommandTimeUnit::beats, startBeats, startSeconds, parseError))
        return makeErrorReply ("import_midi_to_track: " + parseError);

    double lengthSeconds = 0.0;

    if (! convertDurationValueToSeconds (*edit, CommandTimeUnit::beats, startSeconds, lengthBeats, lengthSeconds, parseError))
        return makeErrorReply ("import_midi_to_track: " + parseError);

    lengthSeconds = juce::jmax (lengthSeconds, kMinimumSurvivingClipLengthSeconds);

    const auto tr = te::TimeRange (te::TimePosition::fromSeconds (startSeconds),
                                   te::TimePosition::fromSeconds (startSeconds + lengthSeconds));

    auto& undo = edit->getUndoManager();
    undo.beginNewTransaction ("Import MIDI file");

    auto newClip = clipTrack->insertMIDIClip (tr, nullptr);

    if (newClip == nullptr)
        return makeErrorReply ("import_midi_to_track: engine refused to create clip");

    const auto clipName = sourceFile.getFileNameWithoutExtension();
    newClip->setName (clipName);
    newClip->state.setProperty ("vit_original_source_path", sourceFile.getFullPathName(), &undo);
    newClip->state.setProperty ("vit_source_file_kind", "midi", &undo);

    auto& seq = newClip->getSequence();
    int noteIndex = 0;

    for (const auto& note : importedNotes)
    {
        auto* created = seq.addNote (note.pitch,
                                     te::BeatPosition::fromBeats (note.startBeat),
                                     te::BeatDuration::fromBeats (note.lengthBeats),
                                     note.velocity,
                                     0,
                                     &undo);

        if (created != nullptr)
            created->state.setProperty (kVitNoteId, "midi_file_note_" + juce::String (++noteIndex), &undo);
    }

    edit->invalidateStoredLength();
    edit->dispatchPendingUpdatesSynchronously();
    edit->getTransport().ensureContextAllocated (true);

    auto response = std::make_unique<juce::DynamicObject>();
    response->setProperty ("status", "ok");
    response->setProperty ("message", "MIDI file imported");
    response->setProperty ("clip_id", newClip->itemID.toString());
    response->setProperty ("new_clip_id", newClip->itemID.toString());
    response->setProperty ("track_id", trackId);
    response->setProperty ("clip_name", clipName);
    response->setProperty ("source_file_path", sourceFile.getFullPathName());
    response->setProperty ("start_time_beats", startBeats);
    response->setProperty ("length_beats", lengthBeats);
    response->setProperty ("start_seconds", newClip->getPosition().getStart().inSeconds());
    response->setProperty ("length_seconds", newClip->getPosition().getLength().inSeconds());
    response->setProperty ("note_count", static_cast<int> (importedNotes.size()));
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
