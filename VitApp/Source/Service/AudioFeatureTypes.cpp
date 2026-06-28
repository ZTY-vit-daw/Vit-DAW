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
        case AudioFeatureType::BandEnergySummary:       return "band_energy_summary";
        case AudioFeatureType::StereoRelationSummary:   return "stereo_relation_summary";
        case AudioFeatureType::LoudnessSummary:         return "loudness_summary";
        case AudioFeatureType::L3AcousticSummary:       return "l3_acoustic_summary";
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
    if (key == "stereo_relation_field" || key == "ba")
        return AudioFeatureType::StereoRelationField;
    if (key == "band_energy_summary" || key == "band_energy" || key == "band_summary")
        return AudioFeatureType::BandEnergySummary;
    if (key == "stereo_relation_summary" || key == "stereo_relation" || key == "stereo_summary" || key == "stereo_correlation")
        return AudioFeatureType::StereoRelationSummary;
    if (key == "loudness_summary" || key == "loudness" || key == "lufs_summary" || key == "lufs_analysis")
        return AudioFeatureType::LoudnessSummary;
    if (key == "l3_acoustic_summary" || key == "l3_summary" || key == "dad_l3")
        return AudioFeatureType::L3AcousticSummary;
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
    return "audio_feature.v1.2";
}

juce::String audioFeatureProductVersion (AudioFeatureType type)
{
    switch (type)
    {
        case AudioFeatureType::WaveformEnvelope:        return "waveform_envelope.v1";
        case AudioFeatureType::TimeEnergy:              return "time_energy.v1";
        case AudioFeatureType::SpectralField:           return "spectral_field.v2";
        case AudioFeatureType::StereoRelationField:     return "stereo_relation_field.v1";
        case AudioFeatureType::BandEnergySummary:       return "band_energy_summary.v1";
        case AudioFeatureType::StereoRelationSummary:   return "stereo_relation_summary.v1";
        case AudioFeatureType::LoudnessSummary:         return "loudness_summary.v1";
        case AudioFeatureType::L3AcousticSummary:       return "l3_acoustic_summary.v1";
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
