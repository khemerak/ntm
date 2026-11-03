$ErrorActionPreference = "Stop"
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12

$Repo = "khemerak/ntm"
Write-Host "Fetching latest release information for $Repo..." -ForegroundColor Cyan

$ApiUrl = "https://api.github.com/repos/$Repo/releases/latest"
$Release = Invoke-RestMethod -Uri $ApiUrl -Method Get
$LatestTag = $Release.tag_name

if (-not $LatestTag) {
    Write-Host "Error: Could not determine latest release tag." -ForegroundColor Red
    exit 1
}

if ($env:PROCESSOR_ARCHITECTURE -eq "AMD64" -or $env:PROCESSOR_ARCHITEW6432 -eq "AMD64") {
    $Arch = "amd64"
} elseif ($env:PROCESSOR_ARCHITECTURE -eq "ARM64") {
    # The release pipeline only publishes ntm-windows-amd64.exe.
    # Windows on ARM runs it through x64 emulation.
    $Arch = "amd64"
} else {
    Write-Host "Error: Unsupported Windows architecture: $env:PROCESSOR_ARCHITECTURE" -ForegroundColor Red
    exit 1
}

$BinaryName = "ntm-windows-${Arch}.exe"
$DownloadUrl = "https://github.com/$Repo/releases/download/$LatestTag/$BinaryName"

$InstallDir = "$env:LOCALAPPDATA\ntm\bin"
if (-not (Test-Path -Path $InstallDir)) {
    New-Item -ItemType Directory -Path $InstallDir | Out-Null
}

$ExePath = "$InstallDir\ntm.exe"

Write-Host "Downloading ntm $LatestTag for Windows ($Arch)..." -ForegroundColor Cyan
Invoke-WebRequest -Uri $DownloadUrl -OutFile $ExePath

$UserPath = [Environment]::GetEnvironmentVariable("PATH","User")
if ($UserPath -notmatch [regex]::Escape($InstallDir)) {
    Write-Host "Adding $InstallDir to User PATH..." -ForegroundColor Yellow
    
    if ($UserPath.EndsWith(";")) {
        $Suffix = ""
    } else {
        $Suffix = ";"
    }
    
    $NewPath = $UserPath + $Suffix + $InstallDir
    
    [Environment]::SetEnvironmentVariable("PATH", $NewPath, "User")
    $env:PATH = $NewPath
}

Write-Host "ntm installed successfully!" -ForegroundColor Green
Write-Host "Please restart your terminal to ensure PATH updates take effect, then run: ntm --help"