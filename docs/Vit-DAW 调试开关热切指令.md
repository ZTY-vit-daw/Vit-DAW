# Vit-DAW 调试开关热切指令

本文档用于在 **Godot 已启动后**，通过后台方式（PowerShell/环境变量）热切 UI 调试日志，不依赖任何 UI 按钮。

## 1) 运行中热切（推荐）

目标文件：

- `D:\Vit_DAW\VitApp\Workspace\Commands\debug_flags.json`

### 开启 UI 日志

```powershell
Set-Content -Path "D:\Vit_DAW\VitApp\Workspace\Commands\debug_flags.json" -Value '{ "ui_log_enabled": true }' -Encoding UTF8
```

### 关闭 UI 日志

```powershell
Set-Content -Path "D:\Vit_DAW\VitApp\Workspace\Commands\debug_flags.json" -Value '{ "ui_log_enabled": false }' -Encoding UTF8
```

> 说明：`VitDebugFlags.gd` 会周期轮询该文件，通常 1 秒内生效。

## 2) 启动前环境变量（会话级）

### 开启

```powershell
$env:VIT_UI_LOG="1"
```

### 关闭

```powershell
$env:VIT_UI_LOG="0"
```

> 说明：该方式通常需要在启动 Godot 前设置。

## 3) 自定义命令文件路径（可选）

如果希望把热切文件放到其他目录，可在启动前设置：

```powershell
$env:VIT_DEBUG_FLAGS_FILE="D:\custom\debug_flags.json"
```

然后向该文件写入：

```json
{ "ui_log_enabled": true }
```

## 4) 快速检查

查看当前热切文件内容：

```powershell
Get-Content "D:\Vit_DAW\VitApp\Workspace\Commands\debug_flags.json"
```

## 5) 默认值建议（发布模式）

- `ui_log_enabled = false`
- 仅在排障时临时开启，结束后立即关闭
