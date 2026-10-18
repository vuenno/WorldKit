; WorldKit installer (Inno Setup 6). Built by .github/workflows/release.yml:
;   iscc /DAppVersion=0.2.0 /DSourceExe=..\bin\worldkit.exe installer\worldkit.iss
; Installs per user (no admin prompt), matching the per-user "start with Windows" entry.

#ifndef AppVersion
  #error Pass the version: iscc /DAppVersion=x.y.z
#endif
#ifndef SourceExe
  #define SourceExe "..\bin\worldkit.exe"
#endif

[Setup]
; AppId identifies WorldKit across upgrades; never change it.
AppId={{6F3B2A8E-4C1D-4E7A-9B52-1D8C0F7A3E61}
AppName=WorldKit
AppVersion={#AppVersion}
AppVerName=WorldKit {#AppVersion}
AppPublisher=vuenno
AppPublisherURL=https://github.com/vuenno/WorldKit
DefaultDirName={localappdata}\Programs\WorldKit
DefaultGroupName=WorldKit
DisableProgramGroupPage=yes
PrivilegesRequired=lowest
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
OutputDir=..\bin
OutputBaseFilename=WorldKit-Setup-{#AppVersion}
UninstallDisplayIcon={app}\WorldKit.exe
UninstallDisplayName=WorldKit
Compression=lzma2/max
SolidCompression=yes
WizardStyle=modern
; Upgrades replace a running WorldKit, so close it first and reopen it afterwards.
CloseApplications=force
RestartApplications=no

[Tasks]
Name: "desktopicon"; Description: "{cm:CreateDesktopIcon}"; GroupDescription: "{cm:AdditionalIcons}"; Flags: unchecked

[Files]
Source: "{#SourceExe}"; DestDir: "{app}"; DestName: "WorldKit.exe"; Flags: ignoreversion

[Icons]
Name: "{autoprograms}\WorldKit"; Filename: "{app}\WorldKit.exe"
Name: "{autodesktop}\WorldKit"; Filename: "{app}\WorldKit.exe"; Tasks: desktopicon

[Run]
Filename: "{app}\WorldKit.exe"; Description: "{cm:LaunchProgram,WorldKit}"; Flags: nowait postinstall skipifsilent

[UninstallRun]
; Stop a running WorldKit so its exe can be removed.
Filename: "{sys}\taskkill.exe"; Parameters: "/IM WorldKit.exe /F"; Flags: runhidden; RunOnceId: "StopWorldKit"

[Registry]
; Remove the "start with Windows" entry WorldKit's Settings may have added (internal/autostart).
; User data in %AppData%\WorldKit is kept on purpose: chat logs are not replaceable.
Root: HKCU; Subkey: "Software\Microsoft\Windows\CurrentVersion\Run"; ValueType: none; ValueName: "WorldKit"; Flags: uninsdeletevalue dontcreatekey
