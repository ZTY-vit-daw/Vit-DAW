#pragma once

#include <functional>

#include <JuceHeader.h>
#include <tracktion_engine/tracktion_engine.h>

namespace vit
{

namespace te = tracktion;

class MidiService final
{
public:
    using EditGetter = std::function<te::Edit*()>;

    explicit MidiService (EditGetter editGetter);

    juce::String handleAddMidiNotes (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleAddMidiNotesBulk (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleMutateMidiNotes (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleDeleteMidiNotes (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleGetMidiClipNotes (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleGetMidiClipData (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleInsertMidiClip (const juce::DynamicObject&, const juce::String&) const;

private:
    static juce::String makeStatusReply (const juce::String& status, const juce::String& message);
    static juce::String makeErrorReply (const juce::String& message);

    EditGetter getEdit;
};

} // namespace vit
