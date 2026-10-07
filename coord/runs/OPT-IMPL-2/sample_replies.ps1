# OPT-IMPL-2 summary 质量样例采样器（run 工件，不入 scripts/ 体系）
# 真实观察问答路径：kernel（复用 J2 run 留存的 journey_fixture.vit——含 analysis
# manifest）+ 本工作树新构建 agent（含 presentation 改动）+ 真实 LLM。三轮只读
# 观察问句，采集 ChatResponse 的 reply+presentation。
# 泊位：kernel 工作区重定向 run 目录（VIT_PROJECT_XML）；agent 草稿根重定向；
# finally 必拆；不启动 Godot。AGENTS §8 概率性声明：N=3 问句各一次，成功条件=
# presentation 在场且 summary ≤3 行含结论；失败分类分记（无块=保守退化不算崩溃）；
# 止损=环境故障一次即停。
param(
    [string]$RepoRoot = "D:\Vit_DAW",
    [string]$FixtureVit = "D:\Vit_DAW\coord\runs\JOURNEY-2\20261007_110716\project\journey_fixture.vit",
    [int]$HttpPort = 8093,
    [int]$TurnTimeoutSec = 300
)
$ErrorActionPreference = "Stop"
$runRoot = Join-Path $RepoRoot ("coord\runs\OPT-IMPL-2\samples_" + (Get-Date -Format "yyyyMMdd_HHmmss"))
New-Item -ItemType Directory -Path $runRoot -Force | Out-Null
$kernelWorkspace = Join-Path $runRoot "kernel_workspace"
$projectDir = Join-Path $runRoot "project"
$drafts = Join-Path $runRoot "agent_drafts"
foreach ($d in @($kernelWorkspace, (Join-Path $kernelWorkspace "Settings"), (Join-Path $kernelWorkspace "Logs"), $projectDir, $drafts)) {
    New-Item -ItemType Directory -Path $d -Force | Out-Null
}
Copy-Item -LiteralPath (Join-Path $RepoRoot "VitApp\Workspace\default_project.xml") -Destination (Join-Path $kernelWorkspace "default_project.xml") -Force
$fixtureCopy = Join-Path $projectDir "observe_fixture.vit"
Copy-Item -LiteralPath $FixtureVit -Destination $fixtureCopy -Force
$kernelProc = $null
$agentProc = $null
$samples = @()
try {
    # 1) kernel：VIT_PROJECT_XML 重定向到 run 目录空白默认工程（不碰 VitApp\Workspace）
    $kernelExe = Join-Path $RepoRoot "Export\staging\runtime\VitApp.exe"
    if (-not (Test-Path -LiteralPath $kernelExe)) { throw "kernel exe missing: $kernelExe" }
    $priorXml = [System.Environment]::GetEnvironmentVariable("VIT_PROJECT_XML", "Process")
    $env:VIT_PROJECT_XML = (Join-Path $kernelWorkspace "default_project.xml")
    try { $kernelProc = Start-Process -FilePath $kernelExe -WorkingDirectory $kernelWorkspace -WindowStyle Hidden -PassThru }
    finally {
        if ([string]::IsNullOrWhiteSpace($priorXml)) { Remove-Item Env:VIT_PROJECT_XML -ErrorAction SilentlyContinue }
        else { $env:VIT_PROJECT_XML = $priorXml }
    }
    Start-Sleep -Seconds 3

    # 2) agent：本工作树构建（含 presentation 改动），草稿根重定向
    $agentExe = Join-Path $runRoot "VitAgent.sample.exe"
    Push-Location (Join-Path $RepoRoot "agent")
    try {
        & go build -o $agentExe .\cmd\vitagent
        if ($LASTEXITCODE -ne 0) { throw "agent build failed" }
    } finally { Pop-Location }
    $agentLog = Join-Path $runRoot "agent.log"
    $priorDrafts = [System.Environment]::GetEnvironmentVariable("VIT_HISTORY_DRAFT_ROOT", "Process")
    $env:VIT_HISTORY_DRAFT_ROOT = $drafts
    try {
        $agentProc = Start-Process -FilePath $agentExe -ArgumentList @("-http", ("127.0.0.1:" + $HttpPort), "-last-log-path", $agentLog, "-keep-last-log-lines", "1200") -WorkingDirectory (Join-Path $RepoRoot "agent") -WindowStyle Hidden -PassThru
    } finally {
        if ([string]::IsNullOrWhiteSpace($priorDrafts)) { Remove-Item Env:VIT_HISTORY_DRAFT_ROOT -ErrorAction SilentlyContinue }
        else { $env:VIT_HISTORY_DRAFT_ROOT = $priorDrafts }
    }
    $base = "http://127.0.0.1:" + $HttpPort
    $ready = $false
    for ($i = 0; $i -lt 30; $i++) {
        try { Invoke-RestMethod -Method GET -Uri ($base + "/agent/state") -TimeoutSec 3 | Out-Null; $ready = $true; break } catch { Start-Sleep -Seconds 2 }
    }
    if (-not $ready) { throw "agent http never ready at " + $base }

    # 3) 打开 fixture 工程（含既有 analysis manifest）
    $openBodyJson = ConvertTo-Json @{ tool = "project.open"; confirmed = $true; args = @{ file_path = $fixtureCopy } } -Depth 6
    $open = Invoke-RestMethod -Method POST -Uri ($base + "/agent/invoke") -ContentType "application/json; charset=utf-8" -TimeoutSec 180 -Body ([System.Text.Encoding]::UTF8.GetBytes($openBodyJson))
    $open | ConvertTo-Json -Depth 12 | Set-Content -LiteralPath (Join-Path $runRoot "open_response.json") -Encoding UTF8
    if ([string]$open.status -ne "ok") { throw "project.open status=" + $open.status }
    Start-Sleep -Seconds 4

    # 4) 三轮真实只读观察问句
    $questions = @(
        @{ id = "q1_lowfreq"; text = "这是一个新的独立要求：请只读分析这首歌的低频能量分布并给出观察结论，不要修改工程" },
        @{ id = "q2_levels"; text = "这是一个新的独立要求：请只读分析 Lead Vocal 与 Bass 的电平关系并给出观察结论" },
        @{ id = "q3_stereo"; text = "这是一个新的独立要求：请只读分析这首歌的立体声声场与相位相关度并给出观察结论" }
    )
    foreach ($q in $questions) {
        $convId = "observe_sample_" + $q.id + "_" + (Get-Date -Format "HHmmss")
        $chatBodyJson = ConvertTo-Json @{ conversation_id = $convId; message = $q.text } -Depth 4
        $resp = Invoke-RestMethod -Method POST -Uri ($base + "/agent/chat") -ContentType "application/json; charset=utf-8" -TimeoutSec $TurnTimeoutSec -Body ([System.Text.Encoding]::UTF8.GetBytes($chatBodyJson))
        $resp | ConvertTo-Json -Depth 16 | Set-Content -LiteralPath (Join-Path $runRoot ("chat_" + $q.id + ".json")) -Encoding UTF8
        $hasPresentation = ($null -ne $resp.presentation -and [string]$resp.presentation.detail_mode -eq "layered")
        $summaryLines = 0
        $summaryText = ""
        if ($hasPresentation) {
            $summaryText = [string]$resp.presentation.summary
            $summaryLines = ($summaryText -split "`n" | Where-Object { $_.Trim() -ne "" }).Count
        }
        $samples += [pscustomobject]@{
            question = $q.text
            stop_reason = [string]$resp.stop_reason
            has_presentation = $hasPresentation
            summary_lines = $summaryLines
            summary = $summaryText
            evidence_entries = if ($hasPresentation) { @($resp.presentation.evidence_entries).Count } else { 0 }
        }
    }
}
finally {
    if ($null -ne $agentProc) { try { Stop-Process -Id $agentProc.Id -Force -ErrorAction SilentlyContinue } catch {} }
    if ($null -ne $kernelProc) { try { Stop-Process -Id $kernelProc.Id -Force -ErrorAction SilentlyContinue } catch {} }
    Start-Sleep -Seconds 2
}
$samples | ConvertTo-Json -Depth 6 | Set-Content -LiteralPath (Join-Path $runRoot "samples_summary.json") -Encoding UTF8
$samples | Format-Table question, stop_reason, has_presentation, summary_lines, evidence_entries | Out-String | Write-Host
$withBlock = @($samples | Where-Object { $_.has_presentation }).Count
Write-Host ("samples with presentation: " + $withBlock + "/3; artifacts: " + $runRoot)
