param(
    [string]$DsmPath = 'D:\Vit_DAW\VitApp\Workspace\Artifacts\smoke\product_path_20260804_081349\compressor_control\plugin_alliance_blind_v1\run\cases\18_pro_audio_dsp_dsm_v3_negative\evidence.json',
    [string]$CurvesEquatorPath = 'D:\Vit_DAW\artifacts\eq_compat_matrix\20260727_144918\capture\curves_equator_stereo_negative.json',
    [string]$ProQPath = 'D:\Vit_DAW\temp\multiband-pluginprobe-census\fabfilter_pro_q_dynamic_eq_negative.json',
    [string]$ProMBPath = 'D:\Vit_DAW\temp\multiband-pluginprobe-census\fabfilter_pro_mb_training.json'
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

function Read-ParameterNames([string]$Path) {
    if (-not (Test-Path -LiteralPath $Path)) {
        throw "evidence file not found: $Path"
    }
    $names = [System.Collections.Generic.List[string]]::new()
    foreach ($line in Get-Content -LiteralPath $Path) {
        if ($line -match '"name"\s*:\s*"([^"]+)"') {
            [void]$names.Add($Matches[1])
        }
    }
    return @($names | Sort-Object -Unique)
}

function CountNames([string[]]$Names, [string]$Pattern) {
    return @($Names | Where-Object { $_ -match $Pattern }).Count
}

$dsm = Read-ParameterNames $DsmPath
$curves = Read-ParameterNames $CurvesEquatorPath
$proQ = Read-ParameterNames $ProQPath
$proMB = Read-ParameterNames $ProMBPath

@(
    [ordered]@{
        surface = 'training_dsm_v3'
        frequency_indexed = CountNames $dsm '^Frequency [1-9]$'
        threshold_indexed = CountNames $dsm '^Threshold [1-9]$'
        q_indexed = CountNames $dsm '^Q [1-9]$'
        global_threshold = CountNames $dsm '^Threshold$'
        global_attack = CountNames $dsm '^Attack$'
        global_release = CountNames $dsm '^Release$'
        global_ratio = CountNames $dsm '^(Compressor Ratio|Expander Ratio)$'
        global_knee = CountNames $dsm '^Knee$'
        capture_or_freeze = CountNames $dsm '^(Capture|Freeze Gain)$'
        expected_boundary = 'training_positive_candidate'
    }
    [ordered]@{
        surface = 'regression_curves_equator'
        frequency_indexed = CountNames $curves '^Node [0-9]+ Frequency$'
        gain_indexed = CountNames $curves '^Node [0-9]+ Gain$'
        q_indexed = CountNames $curves '^Node [0-9]+ Q$'
        threshold_indexed = CountNames $curves '^(Node|Band) [0-9]+ .*Threshold'
        global_attack = CountNames $curves '^Attack$'
        global_release = CountNames $curves '^Release$'
        expected_boundary = 'unresolved_spectral_dynamics_surface'
    }
    [ordered]@{
        surface = 'regression_dynamic_eq_negative'
        frequency_indexed = CountNames $proQ '^Band [0-9]+ Frequency$'
        gain_indexed = CountNames $proQ '^Band [0-9]+ Gain$'
        dynamic_range_indexed = CountNames $proQ '^Band [0-9]+ Dynamic Range'
        threshold_indexed = CountNames $proQ '^Band [0-9]+ Threshold$'
        q_indexed = CountNames $proQ '^Band [0-9]+ Q$'
        expected_boundary = 'unsupported_dynamic_eq'
    }
    [ordered]@{
        surface = 'regression_multiband_negative'
        threshold_indexed = CountNames $proMB '^Band [0-9]+ Threshold$'
        attack_indexed = CountNames $proMB '^Band [0-9]+ Attack$'
        release_indexed = CountNames $proMB '^Band [0-9]+ Release$'
        crossover_named = CountNames $proMB 'Crossover|Xover|Cross'
        expected_boundary = 'unsupported_multiband_dynamics'
    }
) | ConvertTo-Json -Depth 4
