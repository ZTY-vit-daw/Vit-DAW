#pragma once

#include <functional>

#include <JuceHeader.h>

#include "AudioFeatureTypes.h"

namespace vit
{

class L3AcousticAnalyzer final
{
public:
    using PublishCallback = std::function<void(const juce::String&)>;

    static void startAnalyze (AudioFeatureBakeRequest request,
                              PublishCallback publishCallback);
};

} // namespace vit
