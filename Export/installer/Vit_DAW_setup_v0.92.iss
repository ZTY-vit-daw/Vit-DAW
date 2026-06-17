; Vit-DAW Windows installer (Inno Setup 6)
; Output: Vit_DAW_0.92_Setup.exe
;
; One-click build:
;   powershell -ExecutionPolicy Bypass -File D:\Vit_DAW\scripts\build_installer_v0.92.ps1
;
; Assemble only:
;   powershell -ExecutionPolicy Bypass -File D:\Vit_DAW\scripts\build_installer_v0.92.ps1 -SkipInnoCompile

#define MyAppName "Vit DAW"
#define MyAppVersion "0.92"
#define MyAppVersionInfo "0.92.0.0"
#define MyAppPublisher "Vit-DAW"
#define MyAppExeName "Vit DAW.exe"

#define ScriptDir ExtractFileDir(SourcePath)
#ifndef CustomReleaseDir
#define ReleaseDir ScriptDir + "\..\_build\Vit_DAW_v0.92_release"
#else
  #define ReleaseDir CustomReleaseDir
#endif
#if FileExists(ScriptDir + "\redist\vc_redist.x64.exe")
  #define HasVcRedist "1"
#else
  #define HasVcRedist "0"
#endif

#if !DirExists(ReleaseDir)
  #error Release directory not found. Run scripts\build_installer_v0.92.ps1 first to generate Export\_build\Vit_DAW_v0.92_release
#endif

[Setup]
AppId={{A7B8C9D0-E1F2-4A5B-9C8D-7E6F5A4B3C2D}
AppName={#MyAppName}
AppVersion={#MyAppVersion}
AppVerName={#MyAppName} {#MyAppVersion}
AppPublisher={#MyAppPublisher}
DefaultDirName={localappdata}\Programs\{#MyAppName}
UsePreviousAppDir=no
DisableDirPage=no
DefaultGroupName={#MyAppName}
OutputDir={#ScriptDir}
OutputBaseFilename=Vit_DAW_{#MyAppVersion}_Setup
SetupIconFile={#ScriptDir}\Vit-logo.ico
UninstallDisplayIcon={app}\{#MyAppExeName}
Compression=lzma2/ultra64
SolidCompression=yes
PrivilegesRequired=lowest
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

[InstallDelete]
Type: filesandordirs; Name: "{app}\bridge"
Type: filesandordirs; Name: "{app}\python_embed"
Type: files; Name: "{app}\start_all.ps1"
Type: files; Name: "{app}\start_all.bat"
Type: files; Name: "{app}\Vit_DAW.exe"
Type: files; Name: "{app}\Vit_DAW_Launcher.exe"

[Files]
Source: "{#ReleaseDir}\*"; DestDir: "{app}"; Flags: ignoreversion recursesubdirs createallsubdirs

#if FileExists(ScriptDir + "\redist\vc_redist.x64.exe")
[Files]
Source: "redist\vc_redist.x64.exe"; DestDir: "{tmp}"; Flags: deleteafterinstall
#endif

[Icons]
Name: "{group}\{#MyAppName}"; Filename: "{app}\{#MyAppExeName}"; WorkingDir: "{app}"; Comment: "Start Vit DAW"
Name: "{autodesktop}\{#MyAppName}"; Filename: "{app}\{#MyAppExeName}"; WorkingDir: "{app}"; Tasks: desktopicon

#if FileExists(ScriptDir + "\redist\vc_redist.x64.exe")
[Run]
Filename: "{tmp}\vc_redist.x64.exe"; Parameters: "/install /quiet /norestart"; StatusMsg: "Installing Microsoft Visual C++ 2015-2022 (x64)..."; Flags: waituntilterminated; Check: not IsVCRuntimePresent
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
