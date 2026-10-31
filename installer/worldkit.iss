; WorldKit installer (Inno Setup 6). Built by .github/workflows/release.yml:
;   iscc /DAppVersion=0.2.0 /DSourceExe=..\bin\worldkit.exe installer\worldkit.iss
; Installs per user (no admin prompt), matching the per-user "start with Windows" entry.
; WorldKit updates itself by running this installer silently (internal/update):
;   WorldKit-Setup-x.y.z.exe /VERYSILENT /SUPPRESSMSGBOXES /NORESTART /CLOSEAPPLICATIONS
;                            /RELAUNCH=1 /WAITPID=<WorldKit's process id> /LOG=<file>
; It waits for that WorldKit to quit, upgrades in place (same AppId, folder and tasks; user
; data in %AppData%\WorldKit is never touched) and starts the new WorldKit afterwards.

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
; Upgrades replace a running WorldKit, so close it first. A self-update reopens it through
; the /RELAUNCH entry in [Run], not Restart Manager.
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
; A self-update is silent, so the entry above is skipped: start the new WorldKit here.
Filename: "{app}\WorldKit.exe"; Flags: nowait; Check: IsRelaunch

[UninstallRun]
; Stop a running WorldKit so its exe can be removed.
Filename: "{sys}\taskkill.exe"; Parameters: "/IM WorldKit.exe /F"; Flags: runhidden; RunOnceId: "StopWorldKit"

[Registry]
; Remove the "start with Windows" entry WorldKit's Settings may have added (internal/autostart).
; User data in %AppData%\WorldKit is kept on purpose: chat logs are not replaceable.
Root: HKCU; Subkey: "Software\Microsoft\Windows\CurrentVersion\Run"; ValueType: none; ValueName: "WorldKit"; Flags: uninsdeletevalue dontcreatekey

[Code]
const
  SYNCHRONIZE = $00100000;
  WaitForExitMilliseconds = 30000;

function OpenProcess(DesiredAccess: DWORD; InheritHandle: BOOL; ProcessId: DWORD): THandle;
  external 'OpenProcess@kernel32.dll stdcall';
function WaitForSingleObject(Handle: THandle; Milliseconds: DWORD): DWORD;
  external 'WaitForSingleObject@kernel32.dll stdcall';
function CloseHandle(Handle: THandle): BOOL;
  external 'CloseHandle@kernel32.dll stdcall';

// The updater passes /WAITPID=<its process id> and quits right after starting this
// installer. Wait for it (up to 30 s) so its exe can be replaced; CloseApplications=force
// still closes it if it hangs.
procedure WaitForUpdatingWorldKit();
var
  ProcessId: Integer;
  Process: THandle;
begin
  ProcessId := StrToIntDef(ExpandConstant('{param:waitpid|0}'), 0);
  if ProcessId <= 0 then
    exit;
  Process := OpenProcess(SYNCHRONIZE, False, ProcessId);
  if Process = 0 then
    exit;
  WaitForSingleObject(Process, WaitForExitMilliseconds);
  CloseHandle(Process);
end;

function InitializeSetup(): Boolean;
begin
  WaitForUpdatingWorldKit();
  Result := True;
end;

// IsRelaunch is true for a self-update (/RELAUNCH=1), which restarts WorldKit when done.
function IsRelaunch(): Boolean;
begin
  Result := ExpandConstant('{param:relaunch|0}') = '1';
end;
