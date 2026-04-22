#pragma once

#include <JuceHeader.h>
#include <tracktion_engine/tracktion_engine.h>

namespace vit
{

namespace te = tracktion;

class VitDagChecker final
{
public:
    static bool wouldCreateCycle (const te::RackType&, te::EditItemID sourceId, te::EditItemID destId);
};

} // namespace vit
