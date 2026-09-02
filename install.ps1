[CmdletBinding()]
param(
    [ValidateSet("User", "Machine")]
    [string]$Scope = $(if ($env:RON1N_INSTALL_SCOPE) { $env:RON1N_INSTALL_SCOPE } else { "User" }),
    [string]$Version = $(if ($env:RON1N_VERSION) { $env:RON1N_VERSION } else { "0.0.1zoro" }),
    [string]$ReleaseTag = $(if ($env:RON1N_RELEASE_TAG) { $env:RON1N_RELEASE_TAG } else { "0.0.1zoro-r1" }),
    [string]$Repository = $(if ($env:RON1N_REPOSITORY) { $env:RON1N_REPOSITORY } else { "0xb0rn3/ron1n" })
)

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

function Test-Administrator {
    $identity = [Security.Principal.WindowsIdentity]::GetCurrent()
    $principal = [Security.Principal.WindowsPrincipal]::new($identity)
    return $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
}

if ($Scope -eq "Machine" -and -not (Test-Administrator)) {
    throw "Machine scope requires an elevated PowerShell. Use the documented Start-Process -Verb RunAs command."
}

function Get-Ron1nArchitecture {
    $detectedArchitecture = $null
    $runtimeType = "System.Runtime.InteropServices.RuntimeInformation" -as [type]
    if ($null -ne $runtimeType) {
        $architectureProperty = $runtimeType.GetProperty("OSArchitecture")
        if ($null -ne $architectureProperty) {
            $detectedArchitecture = $architectureProperty.GetValue($null, $null).ToString()
        }
    }
    if ([string]::IsNullOrWhiteSpace($detectedArchitecture)) {
        if (-not [string]::IsNullOrWhiteSpace($env:PROCESSOR_ARCHITEW6432)) {
            $detectedArchitecture = $env:PROCESSOR_ARCHITEW6432
        } else {
            $detectedArchitecture = $env:PROCESSOR_ARCHITECTURE
        }
    }
    if ([string]::IsNullOrWhiteSpace($detectedArchitecture)) {
        throw "Unable to determine the Windows operating-system architecture."
    }
    return $detectedArchitecture.ToLowerInvariant()
}

$architecture = Get-Ron1nArchitecture
switch ($architecture) {
    "x64" { $targetArch = "amd64"; break }
    "amd64" { $targetArch = "amd64"; break }
    "arm64" { $targetArch = "arm64"; break }
    default { throw "Unsupported Windows architecture: $architecture" }
}

$releaseBase = "https://github.com/$Repository/releases/download/$ReleaseTag"
$hostAsset = "ron1n-windows-$targetArch.exe"
$relayAsset = "ron1n-relay-windows-$targetArch.exe"
$temporary = Join-Path ([IO.Path]::GetTempPath()) ("ron1n-install-" + [Guid]::NewGuid().ToString("N"))
New-Item -ItemType Directory -Path $temporary | Out-Null

try {
    Invoke-WebRequest -UseBasicParsing -Uri "$releaseBase/SHA256SUMS" -OutFile (Join-Path $temporary "SHA256SUMS")
    Invoke-WebRequest -UseBasicParsing -Uri "$releaseBase/$hostAsset" -OutFile (Join-Path $temporary $hostAsset)
    Invoke-WebRequest -UseBasicParsing -Uri "$releaseBase/$relayAsset" -OutFile (Join-Path $temporary $relayAsset)

    $checksumLines = Get-Content -LiteralPath (Join-Path $temporary "SHA256SUMS")
    foreach ($asset in @($hostAsset, $relayAsset)) {
        $escaped = [Regex]::Escape($asset)
        $line = $checksumLines | Where-Object { $_ -match "^[0-9a-fA-F]{64}\s+$escaped$" } | Select-Object -First 1
        if (-not $line) { throw "Checksum for $asset is absent or invalid." }
        $expected = ($line -split "\s+")[0].ToLowerInvariant()
        $actual = (Get-FileHash -Algorithm SHA256 -LiteralPath (Join-Path $temporary $asset)).Hash.ToLowerInvariant()
        if ($actual -ne $expected) { throw "SHA-256 mismatch for $asset." }
    }

    if ($Scope -eq "Machine") {
        $destination = Join-Path $env:ProgramFiles "ron1n"
        $pathTarget = [EnvironmentVariableTarget]::Machine
    } else {
        $destination = Join-Path $env:LOCALAPPDATA "ron1n\bin"
        $pathTarget = [EnvironmentVariableTarget]::User
    }
    New-Item -ItemType Directory -Force -Path $destination | Out-Null
    Copy-Item -Force -LiteralPath (Join-Path $temporary $hostAsset) -Destination (Join-Path $destination "ron1n.exe")
    Copy-Item -Force -LiteralPath (Join-Path $temporary $relayAsset) -Destination (Join-Path $destination "ron1n-relay.exe")

    $currentPath = [Environment]::GetEnvironmentVariable("Path", $pathTarget)
    $parts = @($currentPath -split ";" | Where-Object { $_ })
    if ($parts -notcontains $destination) {
        [Environment]::SetEnvironmentVariable("Path", (($destination) + ";" + $currentPath).TrimEnd(";"), $pathTarget)
    }
    if (($env:Path -split ";") -notcontains $destination) {
        $env:Path = "$destination;$env:Path"
    }

    Write-Host "Installed ron1n $Version from release $ReleaseTag and ron1n-relay to $destination"
    Write-Host "Release checksums verified. Start with: ron1n install"
} finally {
    Remove-Item -Recurse -Force -LiteralPath $temporary -ErrorAction SilentlyContinue
}
