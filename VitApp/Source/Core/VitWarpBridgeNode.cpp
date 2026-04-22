#include "VitWarpBridgeNode.h"

namespace vit
{

VitWarpDescriptor VitWarpBridgeNode::fromRequest (te::Edit& edit, const juce::DynamicObject& object)
{
    VitWarpDescriptor descriptor;

    if (auto* tempo = edit.tempoSequence.getTempo (0))
        descriptor.targetBpm = tempo->getBpm();

    const auto originBpmVar = object.getProperty ("origin_bpm");
    if (originBpmVar.isDouble() || originBpmVar.isInt() || originBpmVar.isInt64())
        descriptor.originBpm = static_cast<double> (originBpmVar);

    descriptor.originKey = object.getProperty ("origin_key").toString().trim();
    descriptor.warpMode = object.getProperty ("warp_mode").toString().trim();
    descriptor.warpState = object.getProperty ("warp_state").toString().trim();

    if (descriptor.warpMode.isEmpty())
        descriptor.warpMode = descriptor.originBpm > 0.0 ? "tempo_map" : "bypassed";

    if (descriptor.warpState.isEmpty())
    {
        if (descriptor.warpMode == "bypassed")
            descriptor.warpState = "bypassed";
        else
            descriptor.warpState = "raw";
    }

    return descriptor;
}

void VitWarpBridgeNode::applyClipProperties (juce::ValueTree& state,
                                             const VitWarpDescriptor& descriptor,
                                             juce::UndoManager* undoManager)
{
    state.setProperty ("vit_origin_bpm", descriptor.originBpm, undoManager);
    state.setProperty ("vit_origin_key", descriptor.originKey, undoManager);
    state.setProperty ("vit_warp_target_bpm", descriptor.targetBpm, undoManager);
    state.setProperty ("vit_warp_mode", descriptor.warpMode, undoManager);
    state.setProperty ("vit_warp_state", descriptor.warpState, undoManager);
}

} // namespace vit
