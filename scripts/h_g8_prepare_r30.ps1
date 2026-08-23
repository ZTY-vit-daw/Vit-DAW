[CmdletBinding()]
param()
Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"
$src = "D:\Vit_DAW_g8_artifacts\h-g8-no-target-20260821\runtime\h-run-remediation-20260822-r28"
$dst = "D:\Vit_DAW_g8_artifacts\h-g8-no-target-20260821\runtime\h-run-remediation-20260822-r30-grok"
if (Test-Path -LiteralPath $dst) { throw "Target already exists: $dst" }
New-Item -ItemType Directory -Path $dst | Out-Null
New-Item -ItemType Directory -Path (Join-Path $dst "logs") | Out-Null
New-Item -ItemType Directory -Path (Join-Path $dst "localappdata") | Out-Null
New-Item -ItemType Directory -Path (Join-Path $dst "project") | Out-Null
Copy-Item -LiteralPath (Join-Path $src "kernel") -Destination $dst -Recurse
Copy-Item -LiteralPath (Join-Path $src "VitApp") -Destination $dst -Recurse
Copy-Item -LiteralPath (Join-Path $src "project\h_g8_problem_no_target.vit") -Destination (Join-Path $dst "project")
Copy-Item -LiteralPath (Join-Path $src "project\stems") -Destination (Join-Path $dst "project") -Recurse
Copy-Item -LiteralPath (Join-Path $src "VitAgent-H-remediated.exe") -Destination $dst
$debug = Join-Path $dst "VitApp\Workspace\Logs\agent_message_loop_debug.jsonl"
if (Test-Path -LiteralPath $debug) { Remove-Item -LiteralPath $debug -Force }
Get-ChildItem -LiteralPath $dst -Force | Select-Object Name,Mode
