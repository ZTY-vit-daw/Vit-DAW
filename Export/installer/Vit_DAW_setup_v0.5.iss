; Vit-DAW Windows installer (Inno Setup 6)
; 输出：与本脚本同目录下的 Vit_DAW_0.5_Setup.exe
;
; 打包前可直接运行：
;   powershell -ExecutionPolicy Bypass -File D:\Vit_DAW\scripts\build_installer_v0.5.ps1
;
; 仅组装目录（不编译 Inno）：
;   powershell -ExecutionPolicy Bypass -File D:\Vit_DAW\scripts\build_installer_v0.5.ps1 -SkipInnoCompile
;
; 可选：将 vc_redist.x64.exe 放到本目录下的 redist\ ，安装程序会在末尾静默安装 VC++ 2015-2022 x64。

#define MyAppName "Vit DAW"
#define MyAppVersion "0.5"
#define MyAppVersionInfo "0.5.0.0"
#define MyAppPublisher "Vit-DAW"
#define MyAppExeName "Vit DAW v0.5.exe"

; 始终相对「本 .iss 所在文件夹」解析，避免从别的 cwd 启动编译器时找不到文件。
#define ScriptDir ExtractFileDir(SourcePath)
#ifndef CustomReleaseDir
#define ReleaseDir ScriptDir + "\..\Vit DAW v0.5"
#else
  #define ReleaseDir CustomReleaseDir
#endif
#if FileExists(ScriptDir + "\redist\vc_redist.x64.exe")
  #define HasVcRedist "1"
#else
  #define HasVcRedist "0"
#endif

#if !DirExists(ReleaseDir)
  #error 未找到打包目录。请先运行 scripts\build_installer_v0.5.ps1 生成：Export\Vit DAW v0.5
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
Name: "{group}\{#MyAppName}"; Filename: "{app}\{#MyAppExeName}"; WorkingDir: "{app}"; Comment: "Start Vit DAW UI"
Name: "{autodesktop}\{#MyAppName}"; Filename: "{app}\{#MyAppExeName}"; WorkingDir: "{app}"; Tasks: desktopicon

#if FileExists(ScriptDir + "\redist\vc_redist.x64.exe")
[Run]
Filename: "{tmp}\vc_redist.x64.exe"; Parameters: "/install /quiet /norestart"; StatusMsg: "Installing Microsoft Visual C++ 2015-2022 (x64)..."; Flags: waituntilterminated
#endif
Filename: "{app}\{#MyAppExeName}"; Description: "Launch Vit DAW now"; WorkingDir: "{app}"; Flags: postinstall nowait skipifsilent

[Code]
function IsVCRuntimePresent: Boolean;
begin
  Result :=
    FileExists(ExpandConstant('{sys}\vcruntime140.dll')) and
    FileExists(ExpandConstant('{sys}\vcruntime140_1.dll'));
end;

function InitializeSetup: Boolean;
begin
  Result := True;

  if not IsWin64 then
  begin
    MsgBox('Vit DAW requires a 64-bit Windows system.', mbError, MB_OK);
    Result := False;
    exit;
  end;

  if ('{#HasVcRedist}' = '0') and (not IsVCRuntimePresent) then
  begin
    MsgBox(
      'Microsoft Visual C++ 2015-2022 x64 runtime is required.'#13#10 +
      'This installer was built without bundled vc_redist.x64.exe.'#13#10 +
      'Please install VC++ runtime manually, or rebuild installer with redist\vc_redist.x64.exe present.',
      mbError, MB_OK);
    Result := False;
    exit;
  end;
end;
