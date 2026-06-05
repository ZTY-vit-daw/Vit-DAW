#include "AudioFeatureTypes.h"

namespace vit
{

juce::String audioFeatureTypeToString (AudioFeatureType type)
{
    switch (type)
    {
        case AudioFeatureType::WaveformEnvelope:        return "waveform_envelope";
        case AudioFeatureType::TimeEnergy:              return "time_energy";
        case AudioFeatureType::SpectralField:           return "spectral_field";
        case AudioFeatureType::StereoRelationField:     return "stereo_relation_field";
        case AudioFeatureType::SegmentationPrimitives:  return "segmentation_primitives";
        case AudioFeatureType::ThreeDField:             return "3d_field";
        case AudioFeatureType::MasterOutputPreview:     return "master_output_preview";
    }

    return "spectral_field";
}

AudioFeatureType audioFeatureTypeFromString (const juce::String& text, AudioFeatureType fallback)
{
    const auto key = text.trim().toLowerCase();

    if (key == "waveform_envelope" || key == "waveform")
        return AudioFeatureType::WaveformEnvelope;
    if (key == "time_energy" || key == "energy")
        return AudioFeatureType::TimeEnergy;
    if (key == "spectral_field" || key == "spectrogram" || key == "rgba_spectral_tile")
        return AudioFeatureType::SpectralField;
    if (key == "stereo_relation_field" || key == "stereo_relation" || key == "ba")
        return AudioFeatureType::StereoRelationField;
    if (key == "segmentation_primitives" || key == "segmentation")
        return AudioFeatureType::SegmentationPrimitives;
    if (key == "3d_field" || key == "three_d_field" || key == "field_3d")
        return AudioFeatureType::ThreeDField;
    if (key == "master_output_preview" || key == "master_preview")
        return AudioFeatureType::MasterOutputPreview;

    return fallback;
}

juce::String audioFeaturePriorityToString (AudioFeaturePriority priority)
{
    switch (priority)
    {
        case AudioFeaturePriority::ImportImmediate: return "import_immediate";
        case AudioFeaturePriority::BackgroundWarm:  return "background_warm";
        case AudioFeaturePriority::OnDemand:        return "on_demand";
    }

    return "on_demand";
}

juce::String audioFeatureAnalysisVersion()
{
    return "audio_feature.v1";
}

juce::String audioFeatureProductVersion (AudioFeatureType type)
{
    switch (type)
    {
        case AudioFeatureType::WaveformEnvelope:        return "waveform_envelope.v1";
        case AudioFeatureType::TimeEnergy:              return "time_energy.v1";
        case AudioFeatureType::SpectralField:           return "spectral_field.v2";
        case AudioFeatureType::StereoRelationField:     return "stereo_relation_field.v1";
        case AudioFeatureType::SegmentationPrimitives:  return "segmentation_primitives.v1";
        case AudioFeatureType::ThreeDField:             return "3d_field.v1";
        case AudioFeatureType::MasterOutputPreview:     return "master_output_preview.v1";
    }

    return "audio_feature.v1";
}

bool audioFeatureUsesSpectralTextureTile (AudioFeatureType type)
{
    return type == AudioFeatureType::SpectralField
        || type == AudioFeatureType::StereoRelationField
        || type == AudioFeatureType::ThreeDField;
}

} // namespace vit
