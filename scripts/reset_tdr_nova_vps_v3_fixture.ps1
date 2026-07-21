[CmdletBinding(SupportsShouldProcess = $true)]
param(
    [string]$RepoRoot = "D:\Vit_DAW",
    [string]$VPSLibraryPath = "",
    [string]$SPALProviderStorePath = "",
    [string]$ArtifactDir = "",
    [switch]$Apply,
    [switch]$AllowRunning
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$TDRProfileKey = "plugin_f9788adda4203df8"
$TDRProviderID = "lab.tdr_nova.vst3.static_bell.v0"

function Resolve-DefaultVPSLibraryPath {
    if (-not [string]::IsNullOrWhiteSpace($VPSLibraryPath)) {
        return $VPSLibraryPath
    }
    if (-not [string]::IsNullOrWhiteSpace($env:VIT_VPS_LIBRARY_V3_PATH)) {
        return $env:VIT_VPS_LIBRARY_V3_PATH
    }
    return (Join-Path $env:APPDATA "Vit\Agent\vps_library_v3.json")
}

function Resolve-DefaultSPALStorePath {
    if (-not [string]::IsNullOrWhiteSpace($SPALProviderStorePath)) {
        return $SPALProviderStorePath
    }
    if (-not [string]::IsNullOrWhiteSpace($env:VIT_SPAL_REFERENCE_EQ_PROVIDER_STORE)) {
        return $env:VIT_SPAL_REFERENCE_EQ_PROVIDER_STORE
    }
    return (Join-Path $env:APPDATA "Vit\Agent\spal_reference_eq_providers_v0.json")
}

function Test-TDRIdentity {
    param([object]$Identity)
    if ($null -eq $Identity) {
        return $false
    }
    $name = [string]$Identity.name
    $profileKey = [string]$Identity.profile_key
    $path = [string]$Identity.install_path
    return $profileKey -eq $TDRProfileKey -or
        $name -match "(?i)\btdr\s+nova\b" -or
        $path -match "(?i)tdr\s+nova"
}

function Write-JsonAtomic {
    param([string]$Path, [object]$Value)
    $directory = Split-Path -Parent $Path
    if (-not [string]::IsNullOrWhiteSpace($directory)) {
        New-Item -ItemType Directory -Path $directory -Force | Out-Null
    }
    $temporary = $Path + ".tmp"
    $Value | ConvertTo-Json -Depth 64 | Set-Content -LiteralPath $temporary -Encoding UTF8
    Move-Item -LiteralPath $temporary -Destination $Path -Force
}

function Assert-NoRelevantProcesses {
    if ($AllowRunning) {
        return
    }
    $names = @("VitAgent", "VitApp", "VspHub", "Godot", "Godot_v4.6.1-stable_win64", "Godot_v4.6.1-stable_win64_console")
    $running = @(Get-Process -ErrorAction SilentlyContinue | Where-Object { $names -contains $_.ProcessName })
    if ($running.Count -gt 0) {
        $detail = ($running | ForEach-Object { $_.ProcessName + "#" + $_.Id }) -join ", "
        throw "Refusing to clear TDR Nova state while relevant processes are running: $detail. Stop them first or use -AllowRunning only for controlled recovery."
    }
}

$vpsPath = Resolve-DefaultVPSLibraryPath
$spalPath = Resolve-DefaultSPALStorePath
$globalProfilePath = Join-Path $RepoRoot ("VitApp\Workspace\plugin_grabber_profiles\" + $TDRProfileKey + ".json")
if ([string]::IsNullOrWhiteSpace($ArtifactDir)) {
    $ArtifactDir = Join-Path $RepoRoot ("artifacts\tdr_nova_reset_" + (Get-Date -Format "yyyyMMdd_HHmmss"))
}

$summary = [ordered]@{
    schema_version = "tdr_nova_vps_v3_reset.v1"
    status = "planned"
    apply = [bool]$Apply
    vps_library_path = $vpsPath
    spal_v0_store_path = $spalPath
    global_profile_path = $globalProfilePath
    removed_vps_documents = @()
    removed_inventory_entries = @()
    removed_spal_v0_records = @()
    global_profile = "absent"
}

if ($Apply) {
    Assert-NoRelevantProcesses
    New-Item -ItemType Directory -Path $ArtifactDir -Force | Out-Null
}

if (Test-Path -LiteralPath $vpsPath) {
    $library = Get-Content -LiteralPath $vpsPath -Raw -Encoding UTF8 | ConvertFrom-Json
    $documents = @($library.documents)
    $removeDocuments = @($documents | Where-Object { Test-TDRIdentity -Identity $_.plugin_identity })
    $summary.removed_vps_documents = @($removeDocuments | ForEach-Object { [string]$_.id })
    $inventory = @($library.plugin_inventory)
    $removeInventory = @($inventory | Where-Object { Test-TDRIdentity -Identity $_.identity })
    $summary.removed_inventory_entries = @($removeInventory | ForEach-Object { [string]$_.id })
    if ($Apply -and ($removeDocuments.Count -gt 0 -or $removeInventory.Count -gt 0)) {
        Copy-Item -LiteralPath $vpsPath -Destination (Join-Path $ArtifactDir "vps_library_v3_before_reset.json") -Force
        $documentIDs = @{}
        foreach ($row in $removeDocuments) { $documentIDs[[string]$row.id] = $true }
        $inventoryIDs = @{}
        foreach ($row in $removeInventory) { $inventoryIDs[[string]$row.id] = $true }
        $library.documents = @($documents | Where-Object { -not $documentIDs.ContainsKey([string]$_.id) })
        $library.plugin_inventory = @($inventory | Where-Object { -not $inventoryIDs.ContainsKey([string]$_.id) })
        Write-JsonAtomic -Path $vpsPath -Value $library
    }
}

if (Test-Path -LiteralPath $spalPath) {
    $store = Get-Content -LiteralPath $spalPath -Raw -Encoding UTF8 | ConvertFrom-Json
    $records = @($store.records)
    $removeRecords = @($records | Where-Object {
            [string]$_.provider_id -eq $TDRProviderID -or
            [string]$_.plugin_profile_key -eq $TDRProfileKey -or
            [string]$_.instance.provider_id -eq $TDRProviderID
        })
    $summary.removed_spal_v0_records = @($removeRecords | ForEach-Object { [string]$_.id })
    if ($Apply -and $removeRecords.Count -gt 0) {
        Copy-Item -LiteralPath $spalPath -Destination (Join-Path $ArtifactDir "spal_reference_eq_providers_v0_before_reset.json") -Force
        $recordIDs = @{}
        foreach ($row in $removeRecords) { $recordIDs[[string]$row.id] = $true }
        $store.records = @($records | Where-Object { -not $recordIDs.ContainsKey([string]$_.id) })
        Write-JsonAtomic -Path $spalPath -Value $store
    }
}

if (Test-Path -LiteralPath $globalProfilePath) {
    $summary.global_profile = "would_remove"
    if ($Apply) {
        Copy-Item -LiteralPath $globalProfilePath -Destination (Join-Path $ArtifactDir "tdr_nova_global_profile_before_reset.json") -Force
        Remove-Item -LiteralPath $globalProfilePath -Force
        $summary.global_profile = "removed"
    }
}

if ($Apply) {
    $summary.status = "applied"
}

New-Item -ItemType Directory -Path $ArtifactDir -Force | Out-Null
$summaryPath = Join-Path $ArtifactDir "tdr_nova_vps_v3_reset_summary.json"
$summary | ConvertTo-Json -Depth 64 | Set-Content -LiteralPath $summaryPath -Encoding UTF8
Write-Output $summaryPath
