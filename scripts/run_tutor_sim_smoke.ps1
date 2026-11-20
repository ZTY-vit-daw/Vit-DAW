<#
TUTOR-SIM-1（池序 435）真栈烟测腿 —— RUNBOOK + 待授权执行器。

目标（全部满足 -> exit 0）：
  S1 已打开工程旁存在 .vit_agent/tutor_sim.yaml，agent 当前工程路径与 -ProjectPath 一致；
  S2 POST /agent/tutor/start 成功：status=ok、第 1/N 步（N 按 start 回执自适应）、LLM 开场非空；
     TUTOR-SIM-4：workflow_data 含 brief/materials 字段（v1 装置；v0 装置 materials 为空数组）；
  S3 错误输入（-WrongAnswer，默认 "0"）不推进：workflow=tutor_sim、passed=false、step_index=1；
  S4 逐步输入 -Answers（默认 "7,42,123"）-> 逐步推进至 completed；
  S5 GET /agent/tutor/status 与上述步骤一致，完成后 completed=true；
  S6 完成后继续输入不再推进、不再需要 LLM，stop_reason=tutor_sim_already_completed；
  S7 所有用户可见 reply 不含命令词（开始乐局 / 写好了 / 乐局状态 / 乐局事件），
     chat 回执不含 goal_id（未落普通 agent loop）。

退出码：0=PASS；1=断言/实验装置失败；2=环境/上游 LLM 失败；3=未授权（默认）。
本脚本只走 agent HTTP，不启动、不重启、不停止任何栈进程。

默认（无 -StackAuthorized）只打印 runbook 并 exit 3。
-DryRun 只打印执行计划，不发任何 HTTP，exit 0。
授权后示例（单行）：
  powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\run_tutor_sim_smoke.ps1 -StackAuthorized -RuntimeOwner "<主管登记 owner>" -ProjectPath "<工程.vit>"
tutor-sim-2（v1，四步小星星装置）：
  powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\run_tutor_sim_smoke.ps1 -StackAuthorized -RuntimeOwner "<owner>" -ProjectPath "<工程.vit>" -Answers "42,模进,已装载,完成"
#>
[CmdletBinding()]
param(
    [string]$RepoRoot = "",
    [string]$AgentBaseUrl = "http://127.0.0.1:7878",
    [string]$ProjectPath = "",
    [string]$ConversationID = "",
    [string]$RuntimeOwner = "",
    [string]$RunRoot = "",
    [int]$TimeoutSeconds = 120,
    # TUTOR-SIM-4（438）：逐步正确答案（逗号分隔；默认= tutor-sim-1 三步装置）。
    [string]$Answers = "7,42,123",
    # TUTOR-SIM-4：错误答案（默认 0；必须不等于任何 -Answers 项）。
    [string]$WrongAnswer = "0",
    [switch]$StackAuthorized,
    [switch]$DryRun
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

function Write-Step { param([string]$Message) Write-Host ""; Write-Host ("== " + $Message) -ForegroundColor Cyan }
function Write-Ok { param([string]$Message) Write-Host ("ok: " + $Message) -ForegroundColor Green }
function Write-WarnLine { param([string]$Message) Write-Host ("warn: " + $Message) -ForegroundColor Yellow }
function Fail { param([string]$Message) throw $Message }

function Get-JsonMember {
    param($Object, [string]$Name)
    if ($null -eq $Object) { return $null }
    $property = $Object.PSObject.Properties[$Name]
    if ($null -eq $property) { return $null }
    return $property.Value
}

function Get-NormalizedPath {
    param([string]$Path)
    if ([string]::IsNullOrWhiteSpace($Path)) { return "" }
    $full = [System.IO.Path]::GetFullPath($Path)
    return $full.TrimEnd([System.IO.Path]::DirectorySeparatorChar, [System.IO.Path]::AltDirectorySeparatorChar)
}

function Test-SamePath {
    param([string]$Left, [string]$Right)
    return [string]::Equals((Get-NormalizedPath $Left), (Get-NormalizedPath $Right), [System.StringComparison]::OrdinalIgnoreCase)
}

function Get-ActiveProjectPath {
    param($State)
    $current = Get-JsonMember $State "current_project"
    $path = Get-JsonMember $current "path"
    if (-not [string]::IsNullOrWhiteSpace([string]$path)) { return [string]$path }
    $history = Get-JsonMember $State "project_history"
    $path = Get-JsonMember $history "project_path"
    if (-not [string]::IsNullOrWhiteSpace([string]$path)) { return [string]$path }
    $shadow = Get-JsonMember $State "shadow"
    $path = Get-JsonMember $shadow "project_path"
    if (-not [string]::IsNullOrWhiteSpace([string]$path)) { return [string]$path }
    return ""
}

function Get-SidecarPath {
    param([string]$Project)
    if ([string]::IsNullOrWhiteSpace($Project)) { return "" }
    $leaf = Get-Item -LiteralPath $Project -ErrorAction SilentlyContinue
    if ($null -ne $leaf -and $leaf.PSIsContainer) {
        $dir = $leaf.FullName
    } else {
        $dir = Split-Path -Parent $Project
    }
    return (Join-Path (Join-Path $dir ".vit_agent") "tutor_sim.yaml")
}

function Find-CommandWords {
    param([string]$Text)
    $found = @()
    foreach ($word in @("开始乐局", "写好了", "乐局状态", "乐局事件")) {
        if (-not [string]::IsNullOrWhiteSpace($Text) -and $Text.Contains($word)) { $found += $word }
    }
    return $found
}

function Assert-True {
    param([bool]$Condition, [string]$Message)
    if (-not $Condition) { throw $Message }
}

if ([string]::IsNullOrWhiteSpace($RepoRoot)) {
    $RepoRoot = Split-Path -Parent (Split-Path -Parent $MyInvocation.MyCommand.Path)
} else {
    $RepoRoot = (Resolve-Path -LiteralPath $RepoRoot).Path
}

if (-not $StackAuthorized -and -not $DryRun) {
    Write-Host "TUTOR-SIM-1（池序 435）真栈烟测腿：RUNBOOK 已就位，未授权不运行。" -ForegroundColor Yellow
    Write-Host "  前置：主管登记 coord/resources/PC-RUNTIME-STACK.md 栈位并书面授权；" -ForegroundColor Yellow
    Write-Host "        工程已由产品入口打开，且 <工程目录>/.vit_agent/tutor_sim.yaml 已放置；" -ForegroundColor Yellow
    Write-Host "        实验文件模板见 coord/experiments/tutor-sim-1/tutor_sim.yaml 与 README.md。" -ForegroundColor Yellow
    Write-Host "  授权后追加：-StackAuthorized -RuntimeOwner '<owner>' -ProjectPath '<工程.vit>'。" -ForegroundColor Yellow
    Write-Host "        v1 四步装置（tutor-sim-2）追加：-Answers '42,模进,已装载,完成'。" -ForegroundColor Yellow
    Write-Host "  本脚本不启动/重启/停止栈进程（只走 agent HTTP）。" -ForegroundColor Yellow
    exit 3
}

$resolvedProject = ""
$sidecarPath = ""
if (-not [string]::IsNullOrWhiteSpace($ProjectPath)) {
    if (Test-Path -LiteralPath $ProjectPath) {
        $resolvedProject = (Resolve-Path -LiteralPath $ProjectPath).Path
    } else {
        $resolvedProject = [System.IO.Path]::GetFullPath($ProjectPath)
    }
    $sidecarPath = Get-SidecarPath $resolvedProject
}

if ($DryRun) {
    $plan = [ordered]@{
        card = "TUTOR-SIM-1"
        mode = "dry_run"
        repo_root = $RepoRoot
        agent_base_url = $AgentBaseUrl
        project_path = $resolvedProject
        sidecar_path = $sidecarPath
        sidecar_exists = ($sidecarPath -ne "" -and (Test-Path -LiteralPath $sidecarPath))
        runtime_owner = $RuntimeOwner
        answers = $Answers
        wrong_answer = $WrongAnswer
        planned_steps = @(
            "GET /health 与 /agent/state（真栈时）",
            "POST /agent/tutor/start {conversation_id, project_path}",
            "POST /agent/chat 错输入 $WrongAnswer -> 断言不推进",
            "POST /agent/chat 逐步答案 $Answers -> 断言推进/完成 + workflow_data brief/materials",
            "GET /agent/tutor/status 断言 completed",
            "完成后追加输入 -> 断言 already_completed"
        )
    }
    $plan | ConvertTo-Json -Depth 16
    exit 0
}

if ([string]::IsNullOrWhiteSpace($RuntimeOwner)) {
    Fail "真栈执行必须在 -RuntimeOwner 记录主管登记的栈位 owner（防未登记占用）"
}
if ([string]::IsNullOrWhiteSpace($resolvedProject)) {
    Fail "真栈执行必须用 -ProjectPath 指定已打开的实验工程"
}
if (-not (Test-Path -LiteralPath $resolvedProject)) {
    Fail "工程不存在: $resolvedProject"
}
if ([string]::IsNullOrWhiteSpace($sidecarPath) -or -not (Test-Path -LiteralPath $sidecarPath)) {
    Fail ("工程旁缺少实验 sidecar: " + $sidecarPath + "（模板见 coord/experiments/tutor-sim-1/）")
}

$stamp = Get-Date -Format "yyyyMMdd_HHmmss"
if ([string]::IsNullOrWhiteSpace($RunRoot)) {
    $RunRoot = Join-Path (Join-Path (Join-Path (Join-Path $RepoRoot "VitApp") "Workspace") "Artifacts") ("smoke/tutor_sim_" + $stamp)
}
$RunRoot = [System.IO.Path]::GetFullPath($RunRoot)
New-Item -ItemType Directory -Path $RunRoot -Force | Out-Null
New-Item -ItemType Directory -Path (Join-Path $RunRoot "requests") -Force | Out-Null
New-Item -ItemType Directory -Path (Join-Path $RunRoot "responses") -Force | Out-Null

if ([string]::IsNullOrWhiteSpace($ConversationID)) {
    $ConversationID = "tutor-smoke-" + [guid]::NewGuid().ToString("N").Substring(0, 10)
}

$summary = [ordered]@{
    schema_version = "vit_tutor_sim_smoke.v1"
    card = "TUTOR-SIM-1"
    status = "running"
    runtime_owner = $RuntimeOwner
    agent_base_url = $AgentBaseUrl
    project_path = $resolvedProject
    sidecar_path = $sidecarPath
    conversation_id = $ConversationID
    agent_state_path = ""
    start = $null
    turns = @()
    final_status = $null
    command_words_found = @()
    answer_leak_warnings = @()
    created_at = (Get-Date).ToString("o")
    finished_at = ""
    error = ""
}
$turnRecords = @()
$commandWordsFound = @()
$answerLeakWarnings = @()

# TUTOR-SIM-4（438）：逐步答案与错误答案（答案序列按 -Answers 参数自适应）。
$answerSequence = @()
foreach ($answer in ($Answers -split ",")) {
    $trimmedAnswer = $answer.Trim()
    if (-not [string]::IsNullOrWhiteSpace($trimmedAnswer)) { $answerSequence += $trimmedAnswer }
}
if ($answerSequence.Count -eq 0) { Fail "-Answers 至少需要一个答案（逗号分隔）" }
$wrongAnswerText = $WrongAnswer.Trim()
if ($answerSequence -contains $wrongAnswerText) {
    Fail "-WrongAnswer ($wrongAnswerText) 不得与任何 -Answers 项相同（错输入断言会失真）"
}
# start 回执自适应的期望总步数（v0 三步 / v1 按装置）。
$script:expectedStepTotal = 0
$script:startSchemaVersion = ""

function Assert-TutorWorkflowDataPayload {
    # TUTOR-SIM-4：v1 装置断言 workflow_data 含 brief/materials 字段；
    # v0 装置断言 materials 为空数组（UI 不渲染卡的契约面）。
    param($Data, [string]$Name)
    Assert-True ($null -ne $Data) ($Name + " 缺 workflow_data")
    $schemaVersion = [string](Get-JsonMember $Data "schema_version")
    $brief = Get-JsonMember $Data "brief"
    $materials = Get-JsonMember $Data "materials"
    if ($schemaVersion -eq "vit.tutor_sim.v1") {
        Assert-True ($null -ne $brief) ($Name + " v1 workflow_data 缺 brief")
        Assert-True ($null -ne $materials) ($Name + " v1 workflow_data 缺 materials")
    } else {
        Assert-True ($null -eq $materials -or @($materials).Count -eq 0) ($Name + " v0 workflow_data materials 应为空")
    }
}

function Save-Summary {
    param([string]$Status, [string]$ErrorText)
    $summary.status = $Status
    $summary.error = $ErrorText
    $summary.turns = @($turnRecords)
    $summary.command_words_found = @($commandWordsFound)
    $summary.answer_leak_warnings = @($answerLeakWarnings)
    $summary.finished_at = (Get-Date).ToString("o")
    $summary | ConvertTo-Json -Depth 32 | Set-Content -LiteralPath (Join-Path $RunRoot "summary.json") -Encoding UTF8
}

function Stop-Environment {
    param([string]$Message)
    Save-Summary "environment_failure" $Message
    Write-Host ("ENV: " + $Message) -ForegroundColor Red
    exit 2
}

function Stop-Upstream {
    param([string]$Message)
    Save-Summary "upstream_failure" $Message
    Write-Host ("UPSTREAM: " + $Message) -ForegroundColor Red
    exit 2
}

function Invoke-AgentJson {
    param([string]$Name, [string]$Method, [string]$Path, $Body)
    $uri = ($AgentBaseUrl.TrimEnd("/") + $Path)
    $requestFile = Join-Path (Join-Path $RunRoot "requests") ($Name + ".json")
    $responseFile = Join-Path (Join-Path $RunRoot "responses") ($Name + ".json")
    try {
        if ($null -ne $Body) {
            $json = ConvertTo-Json -InputObject $Body -Depth 32 -Compress
            $json | Set-Content -LiteralPath $requestFile -Encoding UTF8
            $bytes = [System.Text.Encoding]::UTF8.GetBytes($json)
            $response = Invoke-RestMethod -Method $Method -Uri $uri -ContentType "application/json; charset=utf-8" -Body $bytes -TimeoutSec $TimeoutSeconds
        } else {
            $response = Invoke-RestMethod -Method $Method -Uri $uri -TimeoutSec $TimeoutSeconds
        }
        $response | ConvertTo-Json -Depth 32 | Set-Content -LiteralPath $responseFile -Encoding UTF8
        return $response
    } catch {
        $statusCode = 0
        $bodyText = ""
        if ($null -ne $_.Exception.Response) {
            $statusCode = [int]$_.Exception.Response.StatusCode
            try {
                $stream = $_.Exception.Response.GetResponseStream()
                if ($null -ne $stream) {
                    $reader = New-Object System.IO.StreamReader($stream)
                    $bodyText = $reader.ReadToEnd()
                    $reader.Close()
                }
            } catch { }
        }
        if ([string]::IsNullOrWhiteSpace($bodyText) -and $null -ne $_.ErrorDetails) {
            $bodyText = [string]$_.ErrorDetails.Message
        }
        $failure = [ordered]@{ http_status = $statusCode; message = $_.Exception.Message; body = $bodyText }
        $failure | ConvertTo-Json -Depth 16 | Set-Content -LiteralPath $responseFile -Encoding UTF8
        return [pscustomobject]@{ __smoke_error = $true; http_status = $statusCode; message = $_.Exception.Message; body = $bodyText }
    }
}

function Test-SmokeError {
    param($Response)
    return [bool](Get-JsonMember $Response "__smoke_error")
}

function Get-FailureClass {
    param($Response)
    $bodyText = [string](Get-JsonMember $Response "body")
    if ([string]::IsNullOrWhiteSpace($bodyText)) { return "" }
    try {
        $parsed = $bodyText | ConvertFrom-Json
        return [string](Get-JsonMember $parsed "failure_class")
    } catch {
        return ""
    }
}

function Record-Reply {
    param([string]$Name, [string]$Reply)
    $found = @(Find-CommandWords $Reply)
    if ($found.Count -gt 0) {
        $script:commandWordsFound = @($script:commandWordsFound) + $found
        throw ("reply " + $Name + " 含命令词: " + ($found -join ","))
    }
}

function Get-TutorStatus {
    param([string]$Name)
    $response = Invoke-AgentJson -Name $Name -Method "Get" -Path ("/agent/tutor/status?conversation_id=" + [System.Uri]::EscapeDataString($ConversationID)) -Body $null
    if (Test-SmokeError $response) {
        Stop-Environment ("status 请求失败: " + [string](Get-JsonMember $response "message"))
    }
    return $response
}

function Send-TutorChat {
    param([string]$Name, [string]$Message)
    $response = Invoke-AgentJson -Name $Name -Method "Post" -Path "/agent/chat" -Body ([ordered]@{
        conversation_id = $ConversationID
        message = $Message
        context = @{}
    })
    if (Test-SmokeError $response) {
        $class = Get-FailureClass $response
        if ($class -ne "") { Stop-Upstream ("chat " + $Name + " HTTP " + [string](Get-JsonMember $response "http_status") + " failure_class=" + $class) }
        throw ("chat " + $Name + " 请求失败: " + [string](Get-JsonMember $response "message"))
    }
    $failureClass = [string](Get-JsonMember (Get-JsonMember $response "workflow_data") "failure_class")
    if (-not [string]::IsNullOrWhiteSpace($failureClass) -or -not [string]::IsNullOrWhiteSpace([string](Get-JsonMember $response "error"))) {
        Stop-Upstream ("chat " + $Name + " LLM failure_class=" + $failureClass + " error=" + [string](Get-JsonMember $response "error"))
    }
    Assert-True ([string](Get-JsonMember $response "workflow") -eq "tutor_sim") ("chat " + $Name + " workflow=" + [string](Get-JsonMember $response "workflow"))
    Assert-True ([string]::IsNullOrWhiteSpace([string](Get-JsonMember $response "goal_id"))) ("chat " + $Name + " 落入普通 agent loop（goal_id 非空）")
    $reply = [string](Get-JsonMember $response "reply")
    Assert-True (-not [string]::IsNullOrWhiteSpace($reply)) ("chat " + $Name + " reply 为空")
    Record-Reply $Name $reply
    return $response
}

function Assert-Turn {
    param($Response, [string]$Name, [bool]$Passed, [bool]$Completed, [int]$StepIndex, [string]$StepID, [string]$StopReason)
    $data = Get-JsonMember $Response "workflow_data"
    Assert-True ($null -ne $data) ($Name + " 缺 workflow_data")
    Assert-True ([bool](Get-JsonMember $data "passed") -eq $Passed) ($Name + " passed=" + [string](Get-JsonMember $data "passed") + "，期望 " + $Passed)
    Assert-True ([bool](Get-JsonMember $data "completed") -eq $Completed) ($Name + " completed=" + [string](Get-JsonMember $data "completed") + "，期望 " + $Completed)
    Assert-True ([int](Get-JsonMember $data "step_index") -eq $StepIndex) ($Name + " step_index=" + [string](Get-JsonMember $data "step_index") + "，期望 " + $StepIndex)
    Assert-TutorWorkflowDataPayload -Data $data -Name $Name
    Assert-True ([int](Get-JsonMember $data "step_total") -eq $script:expectedStepTotal) ($Name + " step_total=" + [string](Get-JsonMember $data "step_total") + "，期望 " + [string]$script:expectedStepTotal)
    if (-not [string]::IsNullOrWhiteSpace($StepID)) {
        Assert-True ([string](Get-JsonMember $data "step_id") -eq $StepID) ($Name + " step_id=" + [string](Get-JsonMember $data "step_id") + "，期望 " + $StepID)
    }
    if (-not [string]::IsNullOrWhiteSpace($StopReason)) {
        Assert-True ([string](Get-JsonMember $Response "stop_reason") -eq $StopReason) ($Name + " stop_reason=" + [string](Get-JsonMember $Response "stop_reason") + "，期望 " + $StopReason)
    }
    $turnRecords = $script:turnRecords
    $script:turnRecords = @($turnRecords) + @([ordered]@{
        name = $Name
        step_index = [int](Get-JsonMember $data "step_index")
        step_total = [int](Get-JsonMember $data "step_total")
        step_id = [string](Get-JsonMember $data "step_id")
        passed = [bool](Get-JsonMember $data "passed")
        completed = [bool](Get-JsonMember $data "completed")
        stop_reason = [string](Get-JsonMember $Response "stop_reason")
        reply = [string](Get-JsonMember $Response "reply")
    })
}

try {
    Write-Step "S0 preflight（agent HTTP + 栈身份；本脚本不启停栈）"
    $health = Invoke-AgentJson -Name "health" -Method "Get" -Path "/health" -Body $null
    if (Test-SmokeError $health) {
        Stop-Environment ("agent /health 不可达: " + $AgentBaseUrl)
    }
    Assert-True ([string](Get-JsonMember $health "status") -eq "ok") "agent /health 未返回 status=ok"

    $state = Invoke-AgentJson -Name "state" -Method "Get" -Path "/agent/state" -Body $null
    if (Test-SmokeError $state) {
        Stop-Environment ("agent 面不可达（/agent/state）: " + [string](Get-JsonMember $state "message"))
    }
    $activePath = Get-ActiveProjectPath $state
    $summary.agent_state_path = $activePath
    Assert-True (-not [string]::IsNullOrWhiteSpace($activePath)) "agent 当前没有已打开工程；请先用产品入口打开 -ProjectPath 指向的工程"
    Assert-True (Test-SamePath $activePath $resolvedProject) ("agent 当前工程路径与 -ProjectPath 不一致: " + $activePath + " != " + $resolvedProject)
    Write-Ok "agent 可见且当前工程匹配"

    Write-Step "S1 start：无命令词、LLM 人设开场、第 1/N 步（N 按 start 自适应）"
    $start = Invoke-AgentJson -Name "start" -Method "Post" -Path "/agent/tutor/start" -Body ([ordered]@{
        conversation_id = $ConversationID
        project_path = $resolvedProject
    })
    if (Test-SmokeError $start) {
        $httpStatus = [int](Get-JsonMember $start "http_status")
        $class = Get-FailureClass $start
        if ($httpStatus -ge 500 -or $class -ne "") {
            Stop-Upstream ("start HTTP " + $httpStatus + " failure_class=" + $class + " message=" + [string](Get-JsonMember $start "message"))
        }
        throw ("start 请求失败: HTTP " + $httpStatus + " " + [string](Get-JsonMember $start "message"))
    }
    Assert-True ([string](Get-JsonMember $start "status") -eq "ok") ("start status=" + [string](Get-JsonMember $start "status") + " reason=" + [string](Get-JsonMember $start "reason"))
    Assert-True ([int](Get-JsonMember $start "step_index") -eq 1) "start step_index 不是 1"
    # TUTOR-SIM-4：schema 与总步数按 start 回执自适应（v0/v1 装置共用此脚本）。
    $script:startSchemaVersion = [string](Get-JsonMember $start "schema_version")
    Assert-True ($script:startSchemaVersion -eq "vit.tutor_sim.v0" -or $script:startSchemaVersion -eq "vit.tutor_sim.v1") ("start schema_version 不是 v0/v1: " + $script:startSchemaVersion)
    $script:expectedStepTotal = [int](Get-JsonMember $start "step_total")
    Assert-True ($script:expectedStepTotal -ge 1) "start step_total < 1"
    Assert-True (-not [string]::IsNullOrWhiteSpace([string](Get-JsonMember $start "step_id"))) "start step_id 为空"
    Assert-True ([string](Get-JsonMember $start "conversation_id") -eq $ConversationID) "start conversation_id 与请求不一致"
    Assert-True ($answerSequence.Count -eq $script:expectedStepTotal) ("-Answers 项数 " + [string]($answerSequence.Count) + " 与 step_total " + [string]$script:expectedStepTotal + " 不一致")
    Assert-TutorWorkflowDataPayload -Data (Get-JsonMember $start "workflow_data") -Name "start"
    $startReply = [string](Get-JsonMember $start "reply")
    Assert-True (-not [string]::IsNullOrWhiteSpace($startReply)) "start reply 为空（LLM 未生成开场）"
    Record-Reply "start" $startReply
    $summary.start = [ordered]@{
        conversation_id = [string](Get-JsonMember $start "conversation_id")
        experiment_id = [string](Get-JsonMember $start "experiment_id")
        schema_version = $script:startSchemaVersion
        step_index = [int](Get-JsonMember $start "step_index")
        step_total = [int](Get-JsonMember $start "step_total")
        step_id = [string](Get-JsonMember $start "step_id")
        reply = $startReply
    }
    foreach ($expectValue in $answerSequence) {
        if ($startReply.Contains($expectValue)) {
            $script:answerLeakWarnings = @($script:answerLeakWarnings) + @("start reply 含 expect 值 " + $expectValue)
        }
    }
    Write-Ok "开场已生成"

    Write-Step ("S2 错输入 " + $wrongAnswerText + "：判定失败、不推进")
    $wrong = Send-TutorChat -Name "wrong_0" -Message $wrongAnswerText
    Assert-Turn -Response $wrong -Name "wrong_0" -Passed $false -Completed $false -StepIndex 1 -StepID ([string](Get-JsonMember $start "step_id")) -StopReason "tutor_sim_retry"
    $wrongStatus = Get-TutorStatus "status_after_wrong"
    Assert-True ([int](Get-JsonMember $wrongStatus "step_index") -eq 1) "错误输入后 status step_index 不是 1"
    Assert-True (-not [bool](Get-JsonMember $wrongStatus "completed")) "错误输入后 status completed=true"
    Assert-True ([int](Get-JsonMember $wrongStatus "attempts") -eq 1) "错误输入后 attempts 不是 1"
    Assert-True ([int](Get-JsonMember $wrongStatus "failures") -eq 1) "错误输入后 failures 不是 1"
    Write-Ok "错误输入未推进"

    for ($step = 1; $step -le $answerSequence.Count; $step++) {
        $answer = $answerSequence[$step - 1]
        if ($step -lt $answerSequence.Count) {
            $nextStep = $step + 1
            Write-Step ("S" + ($step + 2) + " 输入 " + $answer + "：推进到第 " + $nextStep + " 步")
            $turn = Send-TutorChat -Name ("answer_" + $step) -Message $answer
            Assert-Turn -Response $turn -Name ("answer_" + $step) -Passed $true -Completed $false -StepIndex $nextStep -StepID "" -StopReason "tutor_sim_step_advanced"
            $turnStatus = Get-TutorStatus ("status_after_" + $step)
            Assert-True ([int](Get-JsonMember $turnStatus "step_index") -eq $nextStep) ($answer + " 之后 status step_index 不是 " + $nextStep)
            Assert-True (-not [bool](Get-JsonMember $turnStatus "completed")) ($answer + " 之后 status completed=true")
            Write-Ok ("第 " + $nextStep + " 步")
        } else {
            Write-Step ("S" + ($step + 2) + " 输入 " + $answer + "：完成")
            $turn = Send-TutorChat -Name ("answer_" + $step) -Message $answer
            Assert-Turn -Response $turn -Name ("answer_" + $step) -Passed $true -Completed $true -StepIndex $script:expectedStepTotal -StepID "" -StopReason "tutor_sim_completed"
            Assert-True ([string](Get-JsonMember $turn "goal_status") -eq "completed") "完成回执 goal_status 不是 completed"
            Write-Ok "实验 completed"
        }
    }

    $finalStatus = Get-TutorStatus "status_completed"
    Assert-True ([bool](Get-JsonMember $finalStatus "completed")) "完成后 status completed=false"
    Assert-True ([string](Get-JsonMember $finalStatus "session_status") -eq "completed") "完成后 session_status 不是 completed"
    Assert-True ([int](Get-JsonMember $finalStatus "step_index") -eq $script:expectedStepTotal) ("完成后 status step_index 不是 " + [string]$script:expectedStepTotal)
    $summary.final_status = [ordered]@{
        step_index = [int](Get-JsonMember $finalStatus "step_index")
        step_total = [int](Get-JsonMember $finalStatus "step_total")
        step_id = [string](Get-JsonMember $finalStatus "step_id")
        session_status = [string](Get-JsonMember $finalStatus "session_status")
        completed = [bool](Get-JsonMember $finalStatus "completed")
        attempts = [int](Get-JsonMember $finalStatus "attempts")
        failures = [int](Get-JsonMember $finalStatus "failures")
    }
    Write-Ok "完成态核对"

    Write-Step "S6 完成后追加输入：不推进、不进入普通 loop"
    $after = Send-TutorChat -Name "after_completed" -Message $answerSequence[0]
    $afterData = Get-JsonMember $after "workflow_data"
    Assert-True ([string](Get-JsonMember $after "stop_reason") -eq "tutor_sim_already_completed") ("完成后 stop_reason=" + [string](Get-JsonMember $after "stop_reason"))
    Assert-True ([bool](Get-JsonMember $afterData "completed")) "完成后追加输入 completed=false"
    Assert-True (-not [bool](Get-JsonMember $afterData "passed")) "完成后追加输入 passed=true"
    $afterStatus = Get-TutorStatus "status_after_completed"
    Assert-True ([int](Get-JsonMember $afterStatus "step_index") -eq $script:expectedStepTotal) "完成后追加输入改变了 step_index"
    Assert-True ([int](Get-JsonMember $afterStatus "attempts") -eq [int](Get-JsonMember $finalStatus "attempts")) "完成后追加输入改变了 attempts"
    Write-Ok "完成后已停止"

    Save-Summary "passed" ""
    Write-Host ""
    Write-Host ("PASS TUTOR-SIM-1；工件: " + $RunRoot) -ForegroundColor Green
    exit 0
} catch {
    $message = $_.Exception.Message
    Save-Summary "failed" $message
    Write-Host ""
    Write-Host ("FAIL: " + $message) -ForegroundColor Red
    Write-Host ("工件: " + $RunRoot) -ForegroundColor Yellow
    exit 1
}
