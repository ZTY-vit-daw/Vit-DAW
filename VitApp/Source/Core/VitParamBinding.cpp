#include "VitParamBinding.h"

namespace vit
{

juce::ValueTree VitParamBinding::createBindingState (const juce::String& bindingId,
                                                     const juce::String& sourceNodeId,
                                                     const juce::String& sourceOutput,
                                                     const juce::String& targetKind,
                                                     const juce::String& targetPluginId,
                                                     const juce::String& targetParamId,
                                                     const juce::String& targetNodeId,
                                                     const juce::String& targetInput,
                                                     double minValue,
                                                     double maxValue,
                                                     const juce::String& curve)
{
    juce::ValueTree state ("VIT_PARAM_BINDING");
    state.setProperty ("binding_id", bindingId, nullptr);
    state.setProperty ("source_node_id", sourceNodeId, nullptr);
    state.setProperty ("source_output", sourceOutput.trim().isNotEmpty() ? sourceOutput.trim() : "value", nullptr);
    state.setProperty ("target_kind", targetKind.trim().isNotEmpty() ? targetKind.trim().toLowerCase() : "plugin_param", nullptr);
    state.setProperty ("target_plugin_id", targetPluginId, nullptr);
    state.setProperty ("target_param_id", targetParamId, nullptr);
    state.setProperty ("target_node_id", targetNodeId, nullptr);
    state.setProperty ("target_input", targetInput.trim().isNotEmpty() ? targetInput.trim() : "value", nullptr);
    state.setProperty ("range_min", minValue, nullptr);
    state.setProperty ("range_max", maxValue, nullptr);
    state.setProperty ("curve", curve.trim().isNotEmpty() ? curve.trim().toLowerCase() : "linear", nullptr);
    state.setProperty ("enabled", true, nullptr);
    return state;
}

juce::var VitParamBinding::createSnapshot (const juce::ValueTree& bindingState)
{
    auto object = std::make_unique<juce::DynamicObject>();
    object->setProperty ("binding_id", bindingState.getProperty ("binding_id").toString());
    object->setProperty ("source_node_id", bindingState.getProperty ("source_node_id").toString());
    object->setProperty ("source_output", bindingState.getProperty ("source_output").toString());
    object->setProperty ("target_kind", bindingState.getProperty ("target_kind").toString());
    object->setProperty ("target_plugin_id", bindingState.getProperty ("target_plugin_id").toString());
    object->setProperty ("target_param_id", bindingState.getProperty ("target_param_id").toString());
    object->setProperty ("target_node_id", bindingState.getProperty ("target_node_id").toString());
    object->setProperty ("target_input", bindingState.getProperty ("target_input").toString());
    object->setProperty ("range_min", bindingState.getProperty ("range_min"));
    object->setProperty ("range_max", bindingState.getProperty ("range_max"));
    object->setProperty ("curve", bindingState.getProperty ("curve").toString());
    object->setProperty ("enabled", bindingState.getProperty ("enabled"));
    return juce::var (object.release());
}

} // namespace vit
