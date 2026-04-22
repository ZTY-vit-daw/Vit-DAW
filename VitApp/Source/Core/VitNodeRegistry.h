#pragma once

#include <JuceHeader.h>

namespace vit
{

class VitNodeRegistry final
{
public:
    static juce::var createSnapshot (const juce::File& projectFile);
};

} // namespace vit
