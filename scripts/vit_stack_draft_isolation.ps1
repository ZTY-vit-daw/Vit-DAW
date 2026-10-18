# Shared draft-root isolation helper for real-stack smoke/diagnostic scripts.
# Card 144 (FIX-STACK-DRAFT-ROOT-ISOLATION-1): every script that starts the
# VitAgent process (directly or through a Godot-launched stack) must redirect
# the agent's conversation draft root away from the user's real
# %LOCALAPPDATA%\Vit\ProjectHistory\drafts (AGENTS.md section 10 spirit).
#
# Contract (same precedence as dev_agent_smoke.ps1 journey berth):
#   1. A caller-provided VIT_HISTORY_DRAFT_ROOT always wins (external env).
#   2. Otherwise the drafts go under the script's run/artifact dir
#      (<RunDir>\agent_drafts) when -RunDir is given, else a per-invocation
#      dir under %TEMP%\VitStackDrafts\<Label>_<timestamp>\agent_drafts.
#   3. Assert-VitDraftRootIsolation must be called before the stack start;
#      it throws when the env is empty, missing on disk, or (for script-owned
#      roots) not rooted inside the isolation base.
#
# Usage:
#   . (Join-Path $RepoRoot "scripts\vit_stack_draft_isolation.ps1")
#   $draftInfo = Enable-VitDraftRootIsolation -Label "run_xxx_smoke" -RunDir $RunDir
#   Assert-VitDraftRootIsolation -Info $draftInfo
#   ... Start-Process VitAgent ...   # inherits env:VIT_HISTORY_DRAFT_ROOT
#
# Windows PowerShell 5.1 compatible (no ?? / -Environment syntax).

function Enable-VitDraftRootIsolation {
    param(
        [Parameter(Mandatory = $true)][string]$Label,
        [string]$RunDir = ""
    )
    $existing = [System.Environment]::GetEnvironmentVariable("VIT_HISTORY_DRAFT_ROOT", "Process")
    if (-not [string]::IsNullOrWhiteSpace($existing)) {
        return @{
            source            = "external"
            draft_root        = $existing
            isolation_base    = ""
            label             = $Label
        }
    }
    $isolationBase = ""
    if (-not [string]::IsNullOrWhiteSpace($RunDir)) {
        $isolationBase = $RunDir
    }
    else {
        $isolationBase = Join-Path ([System.IO.Path]::GetTempPath()) `
            ("VitStackDrafts\" + $Label + "_" + (Get-Date -Format "yyyyMMdd_HHmmss"))
    }
    $draftRoot = Join-Path $isolationBase "agent_drafts"
    New-Item -ItemType Directory -Path $draftRoot -Force | Out-Null
    Set-Item -LiteralPath "env:VIT_HISTORY_DRAFT_ROOT" -Value $draftRoot
    return @{
        source         = "default"
        draft_root     = (Resolve-Path -LiteralPath $draftRoot).Path
        isolation_base = (Resolve-Path -LiteralPath $isolationBase).Path
        label          = $Label
    }
}

function Assert-VitDraftRootIsolation {
    param(
        [Parameter(Mandatory = $true)][hashtable]$Info
    )
    $envRoot = [System.Environment]::GetEnvironmentVariable("VIT_HISTORY_DRAFT_ROOT", "Process")
    if ([string]::IsNullOrWhiteSpace($envRoot)) {
        throw ("VIT_HISTORY_DRAFT_ROOT is empty before stack start (144 isolation contract, label=" + $Info.label + ")")
    }
    if (-not (Test-Path -LiteralPath $envRoot -PathType Container)) {
        throw ("VIT_HISTORY_DRAFT_ROOT path does not exist or is not a directory: " + $envRoot)
    }
    if ($Info.source -eq "external") {
        return
    }
    $resolvedRoot = (Resolve-Path -LiteralPath $envRoot).Path.TrimEnd('\')
    $resolvedBase = (Resolve-Path -LiteralPath $Info.isolation_base).Path.TrimEnd('\')
    if (-not $resolvedRoot.StartsWith($resolvedBase + '\', [System.StringComparison]::OrdinalIgnoreCase)) {
        throw ("VIT_HISTORY_DRAFT_ROOT is not inside the isolation base (root=" + $resolvedRoot + " base=" + $resolvedBase + ")")
    }
}
