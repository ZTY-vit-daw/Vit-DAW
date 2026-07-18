#pragma once

#include <functional>

#include <JuceHeader.h>
#include <tracktion_engine/tracktion_engine.h>

namespace vit
{

namespace te = tracktion;

class ClipService final
{
public:
    using EditGetter = std::function<te::Edit*()>;

    explicit ClipService (EditGetter editGetter);

    juce::String handleMoveClip (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleCloneClip (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleResizeClip (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleSplitClip (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleRemoveClips (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleSetClipFade (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleReadClipFade (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleSetClipGain (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleSetClipGainBatch (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleReadClipGain (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleAnalyzeStripSilence (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleApplyStripSilence (const juce::DynamicObject&, const juce::String&) const;

private:
    static juce::String makeStatusReply (const juce::String& status, const juce::String& message);
    static juce::String makeErrorReply (const juce::String& message);

    EditGetter getEdit;
};

} // namespace vit
