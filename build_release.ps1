#requires -Version 5.1
<#
    Vit-DAW 自动化构建脚本（Windows PowerShell）

    功能：
    1) 使用 CMake 编译 C++ 内核（Release）
    2) 复制内核运行文件（VitApp.exe / bridge_core.py）并尝试复制扩展 DLL 依赖
    3) 调用 Godot 命令行无头导出 Windows Release 包
    4) 将导出目录压缩为 zip 发版包

    使用方式（在 D:\Vit_DAW 根目录执行）：
      powershell -ExecutionPolicy Bypass -File .\build_release.ps1

    可选参数示例：
      .\build_release.ps1 -GodotExe "D:\Godot\Godot_v4.3-stable_win64_console.exe"
#>

[CmdletBinding()]
param(
    # 允许手工指定 Godot 可执行文件；不传则脚本会尝试自动探测。
    [string]$GodotExe = "",
    # 允许手工指定 CMake 生成器，例如 "Visual Studio 17 2022"。
    [string]$CMakeGenerator = "",
    # CMake 并行编译任务数，0 表示让 CMake 自行决定。
    [int]$BuildJobs = 0
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

function Write-Step {
    param([string]$Message)
    Write-Host ""
    Write-Host "==================================================" -ForegroundColor DarkCyan
    Write-Host $Message -ForegroundColor Cyan
    Write-Host "==================================================" -ForegroundColor DarkCyan
}

function Resolve-FirstFileByPattern {
    param(
        [string]$Root,
        [string[]]$Patterns
    )
    if (-not (Test-Path -LiteralPath $Root)) {
        return $null
    }
    foreach ($pattern in $Patterns) {
        $hit = Get-ChildItem -Path $Root -Recurse -File -Filter $pattern -ErrorAction SilentlyContinue |
            Sort-Object LastWriteTime -Descending |
            Select-Object -First 1
        if ($null -ne $hit) {
            return $hit.FullName
        }
    }
    return $null
}

# --------------------------------------------------
# 0) 配置区（按你的工程实际可随时调整）
# --------------------------------------------------

# 项目根目录（当前脚本所在目录）。
$RepoRoot = Split-Path -Parent $MyInvocation.MyCommand.Path

# C++ 内核（CMake 工程）默认位置：D:\Vit_DAW\VitApp
$CppKernelSourceDir = Join-Path $RepoRoot "VitApp"
# CMake 构建目录（out-of-source build）
$CppKernelBuildDir = Join-Path $CppKernelSourceDir "build_release"
# 你提供的内核程序固定路径
$KernelExePath = Join-Path $RepoRoot "VitApp\build\VitApp_artefacts\Release\VitApp.exe"

# 你提供的 Python 桥接脚本固定路径
$BridgeScriptPath = Join-Path $RepoRoot "scripts\bridge_core.py"
$BridgeProdScriptPath = Join-Path $RepoRoot "scripts\bridge_prod.py"
$BridgeProdConfigPath = Join-Path $RepoRoot "scripts\bridge_prod.config.json"

# Godot 前端项目默认位置：D:\Godot\project\vit-daw-frontend
$GodotProjectDir = "D:\Godot\project\vit-daw-frontend"

# GDExtension 动态库落地目录（按 vit_extension.gdextension 的约定）
$FrontendExtensionBinDir = Join-Path $GodotProjectDir "extension\bin"

# 导出根目录与版本目录
$ExportRootDir = Join-Path $RepoRoot "Export"
$ReleaseFolderName = "Vit_DAW_V0.1"
$ReleaseDir = Join-Path $ExportRootDir $ReleaseFolderName
$ReleaseExePath = Join-Path $ReleaseDir "Vit_DAW.exe"
$ReleaseZipPath = Join-Path $ExportRootDir ($ReleaseFolderName + ".zip")
# 发布目录内，内核与桥接脚本的放置位置
$ReleaseRuntimeDir = Join-Path $ReleaseDir "runtime"

# 需要复制的核心 DLL（支持通配符，按顺序匹配，先命中先用）
# 说明：你的内核最终 DLL 命名可能不同，必要时把你自己的名字放到最前面。
$CoreDllPatterns = @(
    "vit_extension.windows.template_release.x86_64.dll",
    "vit_extension*.dll",
    "Vit*.dll",
    "*.dll"
)

# 第三方依赖 DLL 清单（默认含 libzmq.dll）
# 如果后续新增依赖（如 xxx.dll），直接在这里追加。
$ThirdPartyDllNames = @(
    "libzmq.dll"
)

# 从哪些目录搜索可复制的 DLL
$DllSearchRoots = @(
    $CppKernelBuildDir,
    (Join-Path $CppKernelSourceDir "build"),
    $CppKernelSourceDir
)

# Godot 导出预设名（需与 Godot 导出配置一致）
$GodotExportPreset = "Windows Desktop"

# --------------------------------------------------
# 1) 编译 C++ 内核（CMake Release）
# --------------------------------------------------
Write-Step "Step 1/4 - 使用 CMake 编译 C++ 内核（Release）"

if (-not (Test-Path -LiteralPath $CppKernelSourceDir)) {
    throw "未找到 C++ 内核目录：$CppKernelSourceDir。请修改脚本配置变量 `$CppKernelSourceDir。"
}

# 确保构建目录存在
New-Item -ItemType Directory -Path $CppKernelBuildDir -Force | Out-Null

$cmakeConfigureArgs = @("-S", $CppKernelSourceDir, "-B", $CppKernelBuildDir)
if (-not [string]::IsNullOrWhiteSpace($CMakeGenerator)) {
    $cmakeConfigureArgs += @("-G", $CMakeGenerator)
}

Write-Host ">>> cmake $($cmakeConfigureArgs -join ' ')"
& cmake @cmakeConfigureArgs
if ($LASTEXITCODE -ne 0) {
    throw "CMake configure 失败，退出码：$LASTEXITCODE"
}

$cmakeBuildArgs = @("--build", $CppKernelBuildDir, "--config", "Release")
if ($BuildJobs -gt 0) {
    $cmakeBuildArgs += @("--parallel", $BuildJobs)
}

Write-Host ">>> cmake $($cmakeBuildArgs -join ' ')"
& cmake @cmakeBuildArgs
if ($LASTEXITCODE -ne 0) {
    throw "CMake build 失败，退出码：$LASTEXITCODE"
}

# --------------------------------------------------
# 2) 转移运行文件/依赖到 Godot 与发布目录
# --------------------------------------------------
Write-Step "Step 2/4 - 复制内核运行文件与依赖"

if (-not (Test-Path -LiteralPath $GodotProjectDir)) {
    throw "未找到 Godot 前端目录：$GodotProjectDir。请修改脚本配置变量 `$GodotProjectDir。"
}

New-Item -ItemType Directory -Path $FrontendExtensionBinDir -Force | Out-Null
New-Item -ItemType Directory -Path $ReleaseRuntimeDir -Force | Out-Null

# 2.1 复制内核可执行文件（你提供的 VitApp.exe）
$kernelExeResolved = $null
if (Test-Path -LiteralPath $KernelExePath) {
    $kernelExeResolved = (Resolve-Path -LiteralPath $KernelExePath).Path
    Copy-Item -LiteralPath $kernelExeResolved -Destination (Join-Path $ReleaseRuntimeDir "VitApp.exe") -Force
    Write-Host "已复制内核 EXE 到发布目录: $kernelExeResolved"
}
else {
    throw "未找到内核 EXE：$KernelExePath。请确认构建产物路径是否变化。"
}

# 2.2 复制桥接脚本（你提供的 bridge_core.py）
if (Test-Path -LiteralPath $BridgeScriptPath) {
    Copy-Item -LiteralPath $BridgeScriptPath -Destination (Join-Path $ReleaseRuntimeDir "bridge_core.py") -Force
    Write-Host "已复制桥接脚本到发布目录: $BridgeScriptPath"
}
else {
    throw "未找到桥接脚本：$BridgeScriptPath。"
}

# 2.2.1 复制发布桥接入口（bridge_prod.py）和可选配置
if (Test-Path -LiteralPath $BridgeProdScriptPath) {
    Copy-Item -LiteralPath $BridgeProdScriptPath -Destination (Join-Path $ReleaseRuntimeDir "bridge_prod.py") -Force
    Write-Host "已复制发布桥接脚本到发布目录: $BridgeProdScriptPath"
}
if (Test-Path -LiteralPath $BridgeProdConfigPath) {
    Copy-Item -LiteralPath $BridgeProdConfigPath -Destination (Join-Path $ReleaseRuntimeDir "bridge_prod.config.json") -Force
    Write-Host "已复制 bridge_prod 配置到发布目录: $BridgeProdConfigPath"
}

# 2.3 可选：查找并复制内核 DLL（如果你的架构有 GDExtension DLL）
$coreDll = $null
foreach ($root in $DllSearchRoots) {
    $coreDll = Resolve-FirstFileByPattern -Root $root -Patterns $CoreDllPatterns
    if ($null -ne $coreDll) { break }
}

if ($null -eq $coreDll) {
    Write-Warning "未找到可复制的内核 DLL（如果当前架构只用 VitApp.exe，可忽略）。"
}
else {
    Copy-Item -LiteralPath $coreDll -Destination (Join-Path $FrontendExtensionBinDir (Split-Path $coreDll -Leaf)) -Force
    Write-Host "已复制内核 DLL 到 Godot extension/bin: $coreDll"
}

# 2.4 复制第三方依赖 DLL（存在才复制，不存在给出警告）
foreach ($depDllName in $ThirdPartyDllNames) {
    $depPath = $null
    foreach ($root in $DllSearchRoots) {
        $candidate = Resolve-FirstFileByPattern -Root $root -Patterns @($depDllName)
        if ($null -ne $candidate) {
            $depPath = $candidate
            break
        }
    }

    if ($null -eq $depPath) {
        Write-Warning "未找到依赖 DLL：$depDllName（已跳过，若运行时报缺失请补充路径）"
        continue
    }

    $depLeaf = Split-Path $depPath -Leaf
    Copy-Item -LiteralPath $depPath -Destination (Join-Path $FrontendExtensionBinDir $depLeaf) -Force
    Copy-Item -LiteralPath $depPath -Destination (Join-Path $ReleaseRuntimeDir $depLeaf) -Force
    Write-Host "已复制依赖 DLL 到 Godot + 发布目录: $depPath"
}

# --------------------------------------------------
# 3) Godot 无头导出 Release
# --------------------------------------------------
Write-Step "Step 3/4 - Godot 命令行无头导出 Release"

if ([string]::IsNullOrWhiteSpace($GodotExe)) {
    # 优先从 PATH 中找 Godot 命令
    $cmdCandidates = @("godot", "godot4", "Godot_v4.4", "Godot_v4.3", "Godot_v4.2")
    foreach ($cmd in $cmdCandidates) {
        $found = Get-Command $cmd -ErrorAction SilentlyContinue
        if ($null -ne $found) {
            $GodotExe = $found.Source
            break
        }
    }

    # PATH 没找到时，再扫描 D:\Godot 下常见命名（console/headless 优先）
    if ([string]::IsNullOrWhiteSpace($GodotExe) -and (Test-Path "D:\Godot")) {
        $exeHit = Get-ChildItem -Path "D:\Godot" -Recurse -File -Filter "Godot*.exe" -ErrorAction SilentlyContinue |
            Sort-Object @{
                Expression = { if ($_.Name -match "console|headless") { 0 } else { 1 } }
            }, @{
                Expression = { $_.LastWriteTime }
                Descending = $true
            } |
            Select-Object -First 1

        if ($null -ne $exeHit) {
            $GodotExe = $exeHit.FullName
        }
    }
}

if ([string]::IsNullOrWhiteSpace($GodotExe) -or -not (Test-Path -LiteralPath $GodotExe)) {
    throw "未找到 Godot 可执行文件。请用参数 -GodotExe 指定，例如：-GodotExe `"D:\Godot\Godot_v4.3-stable_win64_console.exe`""
}

New-Item -ItemType Directory -Path $ReleaseDir -Force | Out-Null

Push-Location $GodotProjectDir
try {
    # 与你给的参考命令一致：--headless --export-release "<Preset>" "<OutExe>"
    $godotArgs = @(
        "--headless",
        "--path", ".",
        "--export-release", $GodotExportPreset,
        $ReleaseExePath
    )

    Write-Host ">>> `"$GodotExe`" $($godotArgs -join ' ')"
    & $GodotExe @godotArgs
    if ($LASTEXITCODE -ne 0) {
        throw "Godot 导出失败，退出码：$LASTEXITCODE"
    }
}
finally {
    Pop-Location
}

# --------------------------------------------------
# 4) 打包 Zip
# --------------------------------------------------
Write-Step "Step 4/4 - 压缩发布目录为 ZIP"

if (-not (Test-Path -LiteralPath $ReleaseDir)) {
    throw "发布目录不存在：$ReleaseDir"
}

$embedPy = Join-Path $ReleaseDir "python_embed\python.exe"
if (-not (Test-Path -LiteralPath $embedPy)) {
    Write-Warning "未检测到 python_embed（桥接仍依赖系统 Python）。发版前请执行: powershell -ExecutionPolicy Bypass -File `"$RepoRoot\scripts\bundle_python_embed.ps1`""
}

if (Test-Path -LiteralPath $ReleaseZipPath) {
    Remove-Item -LiteralPath $ReleaseZipPath -Force
}

# 一键启动脚本：与 Export\staging 保持一致（内核 / 桥隐式窗口 + 日志目录）
$startPs1Path = Join-Path $ReleaseDir "start_all.ps1"
$startBatPath = Join-Path $ReleaseDir "start_all.bat"
$stagingPs1 = Join-Path $RepoRoot "Export\staging\start_all.ps1"
$stagingBat = Join-Path $RepoRoot "Export\staging\start_all.bat"
if (-not (Test-Path -LiteralPath $stagingPs1) -or -not (Test-Path -LiteralPath $stagingBat)) {
    throw "缺少 Export\staging\start_all.ps1 或 start_all.bat，无法写入发布目录。"
}
Copy-Item -LiteralPath $stagingPs1 -Destination $startPs1Path -Force
Copy-Item -LiteralPath $stagingBat -Destination $startBatPath -Force

Compress-Archive -Path (Join-Path $ReleaseDir "*") -DestinationPath $ReleaseZipPath -CompressionLevel Optimal

Write-Host ""
Write-Host "构建与打包完成。" -ForegroundColor Green
Write-Host "发布目录: $ReleaseDir"
Write-Host "发布压缩包: $ReleaseZipPath"
