param()
$ErrorActionPreference = 'Stop'
# FE-RACK-DELETE-REDRAW-1(212) offline regression family runner (209 family list, 25 pieces)
# Transport-inert offline probes only (tools/probe_*.tscn, non stack_probe_*) —
# zero enqueue / zero UDP send by design (209 grep-verified family).
$FrontendRoot = 'D:\Godot\project\vit-daw-frontend-wt-rackdel1'
$GodotExe = 'D:\Godot\Godot_v4.6.1-stable_win64_console.exe'
$OutDir = 'D:\Vit_DAW_wt_212stack\coord\runs\FE-RACK-DELETE-REDRAW-1\rackdel_20261028_1\offline_regression'
New-Item -ItemType Directory -Force -Path $OutDir | Out-Null
$probes = @(
    'probe_autolane_edit','probe_autolane_layout_grow','probe_autolane_prodchain','probe_autolane_rebuild',
    'probe_automation_datachan','probe_envelope_lane_auto','probe_envelope_lane_toggle','probe_envelope_lane_ui',
    'probe_envelope_teardown','probe_fe_gui_shrink_phase_a','probe_fe_gui_shrink_phase_b','probe_fe_gui_shrink_phase_c',
    'probe_file_reply_volume_face','probe_fx_wet_chain_badge','probe_lane_catalog_fail_loud',
    'probe_rack_plugin_search_popup','probe_rack_sendlevel_fader','probe_rack_sendlevel_realinput',
    'probe_rack_sendroute_macro','probe_start_feed_view','probe_vitnote_circle_state','probe_vitnote_container',
    'probe_vitnote_face_resolve','probe_vitnote_input','probe_vitnote_notestream','probe_vitnote_panel_summary'
)
$failed = @()
foreach ($p in $probes) {
    $log = Join-Path $OutDir ($p + '.log')
    # scene mode when a .tscn pair exists (file_reply_volume_face family), else -s script mode
    # (probe family usage form: godot --headless --path . -s tools/probe_x.gd)
    $scenePath = Join-Path $FrontendRoot ('tools\' + $p + '.tscn')
    $scriptPath = 'tools/' + $p + '.gd'
    if (Test-Path -LiteralPath $scenePath) {
        $argForm = 'res://tools/' + $p + '.tscn'
    } else {
        $argForm = ('-s ' + $scriptPath)
    }
    # wrap via cmd /c: Godot prints benign GDExtension warnings on stderr which
    # PS 5.1 + ErrorActionPreference=Stop turns into a terminating NativeCommandError
    $cmdLine = ('"' + $GodotExe + '" --headless --path "' + $FrontendRoot + '" ' + $argForm + ' > "' + $log + '" 2>&1')
    cmd /c $cmdLine
    $code = $LASTEXITCODE
    Write-Output ($p + ' exit=' + $code)
    if ($code -ne 0) { $failed += ($p + ':' + $code) }
}
if ($failed.Count -gt 0) { Write-Output ('FAILED: ' + ($failed -join ',')); exit 1 }
Write-Output ('ALL_PASS count=' + $probes.Count)
exit 0
