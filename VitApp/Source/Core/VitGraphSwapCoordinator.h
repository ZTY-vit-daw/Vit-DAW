#pragma once

#include <JuceHeader.h>
#include <tracktion_engine/tracktion_engine.h>

#include "VitGraphRevisionLedger.h"

namespace vit
{

namespace te = tracktion;

class VitGraphSwapCoordinator final
{
public:
    static void resetForEdit (te::Edit&, const juce::String& reason);
    static VitGraphRevisionSnapshot publishGraphChange (te::Edit&, const VitGraphChangeEvent&);
    static VitGraphRevisionSnapshot serviceGraphLifecycle (te::Edit&);
    static VitGraphRevisionSnapshot getSnapshot (te::Edit&);
};

} // namespace vit
