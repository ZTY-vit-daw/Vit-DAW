; Vit-DAW Windows installer (Inno Setup 6)
; 用法：在资源管理器中双击本文件，或在 Inno 里 File -> Open，然后 Build -> Compile（F9）。
; 输出：与本脚本同目录下的 Vit_DAW_0.4_Setup.exe
;
; 打包前请确认已生成目录：D:\Vit_DAW\Export\Vit DAW v0.4
;   powershell -ExecutionPolicy Bypass -File D:\Vit_DAW\scripts\assemble_windows_release.ps1 `
;     -GodotUiExe "D:\Godot\project\vit-daw-frontend\Vit DAW v0.4.exe" `
;     -OutDir "D:\Vit_DAW\Export\Vit DAW v0.4"
;
; 可选：将 vc_redist.x64.exe 放到本目录下的 redist\ ，安装程序会在末尾静默安装 VC++ 2015-2022 x64。

#define MyAppName "Vit DAW"
#define MyAppVersion "0.4"
#define MyAppVersionInfo "0.4.0.0"
#define MyAppPublisher "Vit-DAW"
#define MyAppExeName "Vit DAW.exe"
#define MyGodotUiExeName "Vit DAW v0.4.exe"

; 始终相对「本 .iss 所在文件夹」解析，避免从别的 cwd 启动编译器时找不到文件。
#define ScriptDir ExtractFileDir(SourcePath)
#define ReleaseDir ScriptDir + "\..\Vit DAW v0.4"

#if !DirExists(ReleaseDir)
  #error 未找到打包目录。请先运行 scripts\assemble_windows_release.ps1 生成：Export\Vit DAW v0.4
#endif

[Setup]
AppId={{A7B8C9D0-E1F2-4A5B-9C8D-7E6F5A4B3C2D}
AppName={#MyAppName}
AppVersion={#MyAppVersion}
AppVerName={#MyAppName} {#MyAppVersion}
AppPublisher={#MyAppPublisher}
DefaultDirName={autopf}\{#MyAppName}
UsePreviousAppDir=yes
DisableDirPage=no
DefaultGroupName={#MyAppName}
OutputDir={#ScriptDir}
OutputBaseFilename=Vit_DAW_{#MyAppVersion}_Setup
UninstallDisplayIcon={app}\{#MyAppExeName}
Compression=lzma2/ultra64
SolidCompression=yes
PrivilegesRequired=admin
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
DisableProgramGroupPage=yes
WizardStyle=modern
ShowLanguageDialog=no
VersionInfoVersion={#MyAppVersionInfo}
VersionInfoCompany={#MyAppPublisher}
VersionInfoDescription={#MyAppName} Setup
VersionInfoProductName={#MyAppName}
VersionInfoProductVersion={#MyAppVersionInfo}
MinVersion=10.0.17763

[Languages]
Name: "english"; MessagesFile: "compiler:Default.isl"

[Tasks]
Name: "desktopicon"; Description: "{cm:CreateDesktopIcon}"; GroupDescription: "{cm:AdditionalIcons}"; Flags: unchecked

[Files]
Source: "{#ReleaseDir}\*"; DestDir: "{app}"; Flags: ignoreversion recursesubdirs createallsubdirs

#if FileExists(ScriptDir + "\redist\vc_redist.x64.exe")
[Files]
Source: "redist\vc_redist.x64.exe"; DestDir: "{tmp}"; Flags: deleteafterinstall
#endif

[Icons]
Name: "{group}\{#MyAppName}"; Filename: "{app}\{#MyAppExeName}"; WorkingDir: "{app}"; Comment: "Start Vit DAW (kernel, bridge, UI)"
Name: "{group}\{#MyAppName} (UI only)"; Filename: "{app}\{#MyGodotUiExeName}"; WorkingDir: "{app}"; Comment: "UI only — use main shortcut, or start kernel + bridge first"
Name: "{group}\{#MyAppName} (start_all)"; Filename: "{app}\start_all.bat"; WorkingDir: "{app}"; Comment: "PowerShell: kernel + bridge + UI"
Name: "{autodesktop}\{#MyAppName}"; Filename: "{app}\{#MyAppExeName}"; WorkingDir: "{app}"; Tasks: desktopicon

#if FileExists(ScriptDir + "\redist\vc_redist.x64.exe")
[Run]
Filename: "{tmp}\vc_redist.x64.exe"; Parameters: "/install /quiet /norestart"; StatusMsg: "Installing Microsoft Visual C++ 2015-2022 (x64)..."; Flags: waituntilterminated
#endif
Filename: "powershell.exe"; Parameters: "-ExecutionPolicy Bypass -File ""{app}\health_check.ps1"""; Description: "Run environment health check"; Flags: postinstall shellexec skipifsilent unchecked
Filename: "{app}\{#MyAppExeName}"; Description: "Launch Vit DAW now"; WorkingDir: "{app}"; Flags: postinstall nowait skipifsilent
