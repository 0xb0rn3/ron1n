[CmdletBinding()]
param()

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

$ExpectedVersion = "0.0.1zoro"
$ExpectedGoldHENSHA256 = "c6329401d1810e16c84e6474ac30977dbdc951987c10cdb559370de7d59db0b0"
$ExpectedCacheMIME = "text/cache-manifest; charset=utf-8"

function Assert-Smoke {
    param(
        [Parameter(Mandatory = $true)]
        [bool]$Condition,
        [Parameter(Mandatory = $true)]
        [string]$Message
    )

    if (-not $Condition) {
        throw $Message
    }
}

function Write-SmokeStep {
    param([Parameter(Mandatory = $true)][string]$Message)
    Write-Host ("[ron1n-smoke] " + $Message)
}

function Resolve-SmokeBinary {
    param([Parameter(Mandatory = $true)][string]$Name)

    $command = Get-Command -Name $Name -CommandType Application -ErrorAction SilentlyContinue |
        Select-Object -First 1
    if ($null -ne $command) {
        return $command.Source
    }

    $candidates = @(
        (Join-Path $env:LOCALAPPDATA ("ron1n\bin\" + $Name + ".exe")),
        (Join-Path $env:ProgramFiles ("ron1n\" + $Name + ".exe"))
    )
    foreach ($candidate in $candidates) {
        if (Test-Path -LiteralPath $candidate -PathType Leaf) {
            return (Resolve-Path -LiteralPath $candidate).Path
        }
    }

    throw "$Name is not installed or is absent from PATH. Run the documented GitHub installer first."
}

function Invoke-SmokeNative {
    param(
        [Parameter(Mandatory = $true)][string]$FilePath,
        [string[]]$Arguments = @()
    )

    $lines = @(& $FilePath @Arguments 2>&1)
    $exitCode = $LASTEXITCODE
    $text = ($lines | ForEach-Object { $_.ToString() }) -join [Environment]::NewLine
    if ($exitCode -ne 0) {
        throw ("Native command failed ({0}): {1} {2}`n{3}" -f $exitCode, $FilePath, ($Arguments -join " "), $text)
    }
    return $text.Trim()
}

function Get-SmokeFreePort {
    $listener = [Net.Sockets.TcpListener]::new([Net.IPAddress]::Loopback, 0)
    try {
        $listener.Start()
        return ([Net.IPEndPoint]$listener.LocalEndpoint).Port
    } finally {
        $listener.Stop()
    }
}

function Start-SmokeProcess {
    param(
        [Parameter(Mandatory = $true)][string]$Label,
        [Parameter(Mandatory = $true)][string]$FilePath,
        [Parameter(Mandatory = $true)][string[]]$Arguments,
        [Parameter(Mandatory = $true)][string]$TemporaryRoot
    )

    $stdout = Join-Path $TemporaryRoot ($Label + ".stdout.log")
    $stderr = Join-Path $TemporaryRoot ($Label + ".stderr.log")
    $process = Start-Process -FilePath $FilePath -ArgumentList $Arguments -PassThru `
        -WindowStyle Hidden -RedirectStandardOutput $stdout -RedirectStandardError $stderr
    return [PSCustomObject]@{
        Label = $Label
        Process = $process
        Stdout = $stdout
        Stderr = $stderr
    }
}

function Get-SmokeProcessDiagnostics {
    param([Parameter(Mandatory = $true)]$TrackedProcess)

    $parts = @()
    foreach ($name in @($TrackedProcess.Stdout, $TrackedProcess.Stderr)) {
        if (Test-Path -LiteralPath $name -PathType Leaf) {
            $rawBody = Get-Content -LiteralPath $name -Raw -ErrorAction SilentlyContinue
            if ($null -ne $rawBody) {
                $body = $rawBody.Trim()
                if ($body) {
                    $parts += $body
                }
            }
        }
    }
    return ($parts -join [Environment]::NewLine)
}

function Invoke-SmokeHttp {
    param(
        [Parameter(Mandatory = $true)][string]$Uri,
        [int]$RangeStart = -1,
        [int]$RangeEnd = -1,
        [int]$TimeoutSeconds = 30
    )

    $request = [Net.HttpWebRequest]::Create($Uri)
    $request.Method = "GET"
    $request.Proxy = $null
    $request.AllowAutoRedirect = $false
    $request.Timeout = $TimeoutSeconds * 1000
    $request.ReadWriteTimeout = $TimeoutSeconds * 1000
    $request.UserAgent = "ron1n-windows-smoke/$ExpectedVersion"
    if ($RangeStart -ge 0 -or $RangeEnd -ge 0) {
        if ($RangeStart -lt 0 -or $RangeEnd -lt $RangeStart) {
            throw "Invalid smoke-test byte range."
        }
        $request.AddRange($RangeStart, $RangeEnd)
    }

    $response = $null
    $stream = $null
    $memory = $null
    try {
        $response = [Net.HttpWebResponse]$request.GetResponse()
        $stream = $response.GetResponseStream()
        $memory = New-Object IO.MemoryStream
        $stream.CopyTo($memory)
        return [PSCustomObject]@{
            StatusCode = [int]$response.StatusCode
            ContentType = [string]$response.Headers["Content-Type"]
            ContentRange = [string]$response.Headers["Content-Range"]
            Body = [byte[]]$memory.ToArray()
        }
    } catch {
        throw ("HTTP GET failed for {0}: {1}" -f $Uri, $_.Exception.Message)
    } finally {
        if ($null -ne $memory) { $memory.Dispose() }
        if ($null -ne $stream) { $stream.Dispose() }
        if ($null -ne $response) { $response.Dispose() }
    }
}

function Wait-SmokeEndpoint {
    param(
        [Parameter(Mandatory = $true)][string]$Uri,
        [Parameter(Mandatory = $true)]$TrackedProcess,
        [int]$TimeoutSeconds = 20
    )

    $deadline = [DateTime]::UtcNow.AddSeconds($TimeoutSeconds)
    $lastError = "endpoint did not answer"
    while ([DateTime]::UtcNow -lt $deadline) {
        $TrackedProcess.Process.Refresh()
        if ($TrackedProcess.Process.HasExited) {
            $diagnostics = Get-SmokeProcessDiagnostics -TrackedProcess $TrackedProcess
            throw ("{0} exited before becoming ready (exit {1}).`n{2}" -f $TrackedProcess.Label, $TrackedProcess.Process.ExitCode, $diagnostics)
        }
        try {
            $response = Invoke-SmokeHttp -Uri $Uri -TimeoutSeconds 2
            if ($response.StatusCode -eq 200) {
                return $response
            }
            $lastError = "HTTP status $($response.StatusCode)"
        } catch {
            $lastError = $_.Exception.Message
        }
        Start-Sleep -Milliseconds 250
    }

    $diagnostics = Get-SmokeProcessDiagnostics -TrackedProcess $TrackedProcess
    throw ("Timed out waiting for {0}: {1}.`n{2}" -f $Uri, $lastError, $diagnostics)
}

function Get-SmokeHttpStatus {
    param(
        [Parameter(Mandatory = $true)][string]$Uri,
        [int]$TimeoutSeconds = 30
    )

    $request = [Net.HttpWebRequest]::Create($Uri)
    $request.Method = "GET"
    $request.Proxy = $null
    $request.AllowAutoRedirect = $false
    $request.Timeout = $TimeoutSeconds * 1000
    $request.ReadWriteTimeout = $TimeoutSeconds * 1000
    $response = $null
    try {
        $response = [Net.HttpWebResponse]$request.GetResponse()
        return [int]$response.StatusCode
    } catch [Net.WebException] {
        if ($null -eq $_.Exception.Response) {
            throw
        }
        $response = [Net.HttpWebResponse]$_.Exception.Response
        return [int]$response.StatusCode
    } finally {
        if ($null -ne $response) { $response.Dispose() }
    }
}

function Get-SmokePolicySnapshot {
    return ((Get-ExecutionPolicy -List | ForEach-Object {
        "{0}={1}" -f $_.Scope, $_.ExecutionPolicy
    }) -join ";")
}

function Restore-SmokeEnvironment {
    param([Parameter(Mandatory = $true)][hashtable]$Snapshot)

    foreach ($name in $Snapshot.Keys) {
        $entry = $Snapshot[$name]
        if ($entry.Exists) {
            [Environment]::SetEnvironmentVariable($name, $entry.Value, [EnvironmentVariableTarget]::Process)
        } else {
            [Environment]::SetEnvironmentVariable($name, $null, [EnvironmentVariableTarget]::Process)
        }
    }
}

$ron1nPath = Resolve-SmokeBinary -Name "ron1n"
$relayPath = Resolve-SmokeBinary -Name "ron1n-relay"
$policyBefore = Get-SmokePolicySnapshot
$temporaryRoot = Join-Path ([IO.Path]::GetTempPath()) ("ron1n-windows-smoke-" + [Guid]::NewGuid().ToString("N"))
$trackedProcesses = @()
$failure = $null
$testPassed = $false
$cleanupFailure = $null

$environmentNames = @(
    "APPDATA",
    "LOCALAPPDATA",
    "RON1N_PORT",
    "RON1N_REPO_URL",
    "RON1N_CONTENT_DIR",
    "RON1N_RELAY_URL",
    "RON1N_RECENT_HTTP_SECONDS"
)
$environmentBefore = @{}
foreach ($name in $environmentNames) {
    $value = [Environment]::GetEnvironmentVariable($name, [EnvironmentVariableTarget]::Process)
    $environmentBefore[$name] = [PSCustomObject]@{
        Exists = ($null -ne $value)
        Value = $value
    }
}

try {
    New-Item -ItemType Directory -Path $temporaryRoot | Out-Null
    $sandboxRoaming = Join-Path $temporaryRoot "Roaming"
    $sandboxLocal = Join-Path $temporaryRoot "Local"
    New-Item -ItemType Directory -Path $sandboxRoaming | Out-Null
    New-Item -ItemType Directory -Path $sandboxLocal | Out-Null

    [Environment]::SetEnvironmentVariable("APPDATA", $sandboxRoaming, [EnvironmentVariableTarget]::Process)
    [Environment]::SetEnvironmentVariable("LOCALAPPDATA", $sandboxLocal, [EnvironmentVariableTarget]::Process)
    foreach ($name in @("RON1N_REPO_URL", "RON1N_CONTENT_DIR", "RON1N_RELAY_URL", "RON1N_RECENT_HTTP_SECONDS")) {
        [Environment]::SetEnvironmentVariable($name, $null, [EnvironmentVariableTarget]::Process)
    }

    $hostPort = Get-SmokeFreePort
    do {
        $relayPort = Get-SmokeFreePort
    } while ($relayPort -eq $hostPort)
    [Environment]::SetEnvironmentVariable("RON1N_PORT", [string]$hostPort, [EnvironmentVariableTarget]::Process)

    Write-SmokeStep "Verifying installed native binary versions"
    $hostVersion = Invoke-SmokeNative -FilePath $ron1nPath -Arguments @("version")
    $relayVersion = Invoke-SmokeNative -FilePath $relayPath -Arguments @("version")
    Assert-Smoke ($hostVersion -ceq $ExpectedVersion) "ron1n version was '$hostVersion'; expected '$ExpectedVersion'."
    Assert-Smoke ($relayVersion -ceq $ExpectedVersion) "ron1n-relay version was '$relayVersion'; expected '$ExpectedVersion'."

    Write-SmokeStep "Importing the pinned firmware-9.00 content with ron1n install"
    $installOutput = Invoke-SmokeNative -FilePath $ron1nPath -Arguments @("install")
    Assert-Smoke ($installOutput -match [Regex]::Escape("ron1n $ExpectedVersion initialized")) "ron1n install did not confirm the expected version."

    Write-SmokeStep "Starting the loopback-only local host"
    $hostProcess = Start-SmokeProcess -Label "host" -FilePath $ron1nPath `
        -Arguments @("serve", "--listen", "127.0.0.1:$hostPort") -TemporaryRoot $temporaryRoot
    $trackedProcesses += $hostProcess
    $localBase = "http://127.0.0.1:$hostPort"
    $healthResponse = Wait-SmokeEndpoint -Uri "$localBase/_ron1n/health" -TrackedProcess $hostProcess
    $healthText = [Text.Encoding]::UTF8.GetString([byte[]]$healthResponse.Body)
    $health = ConvertFrom-Json -InputObject $healthText
    Assert-Smoke ($healthResponse.StatusCode -eq 200) "Local health did not return HTTP 200."
    Assert-Smoke ([string]$health.status -ceq "ok") "Local health status was not ok."
    Assert-Smoke ([string]$health.version -ceq $ExpectedVersion) "Local health version was not $ExpectedVersion."
    Assert-Smoke (-not [string]::IsNullOrWhiteSpace([string]$health.bundle_id)) "Local health omitted the active bundle ID."

    Write-SmokeStep "Checking cache MIME, full GoldHEN digest, and bytes 0-31"
    $cacheResponse = Invoke-SmokeHttp -Uri "$localBase/psfree_lapse.cache"
    Assert-Smoke ($cacheResponse.StatusCode -eq 200) "psfree_lapse.cache did not return HTTP 200."
    Assert-Smoke ($cacheResponse.ContentType -ceq $ExpectedCacheMIME) "psfree_lapse.cache MIME was '$($cacheResponse.ContentType)'."

    $localGoldHEN = Invoke-SmokeHttp -Uri "$localBase/goldhen.bin" -TimeoutSeconds 60
    Assert-Smoke ($localGoldHEN.StatusCode -eq 200) "Full local goldhen.bin did not return HTTP 200."
    $localGoldHENFile = Join-Path $temporaryRoot "local-goldhen.bin"
    [IO.File]::WriteAllBytes($localGoldHENFile, [byte[]]$localGoldHEN.Body)
    $localDigest = (Get-FileHash -Algorithm SHA256 -LiteralPath $localGoldHENFile).Hash.ToLowerInvariant()
    Assert-Smoke ($localDigest -ceq $ExpectedGoldHENSHA256) "Full local goldhen.bin SHA-256 was $localDigest."

    $rangeResponse = Invoke-SmokeHttp -Uri "$localBase/goldhen.bin" -RangeStart 0 -RangeEnd 31
    Assert-Smoke ($rangeResponse.StatusCode -eq 206) "GoldHEN range request did not return HTTP 206."
    Assert-Smoke ($rangeResponse.Body.Length -eq 32) "GoldHEN bytes=0-31 returned $($rangeResponse.Body.Length) bytes."
    Assert-Smoke ($rangeResponse.ContentRange -match '^bytes 0-31/[1-9][0-9]*$') "GoldHEN Content-Range was '$($rangeResponse.ContentRange)'."

    Write-SmokeStep "Provisioning and starting the loopback relay"
    $hostID = "windows-vm-smoke"
    $tokenFile = Join-Path $temporaryRoot "relay.token"
    [void](Invoke-SmokeNative -FilePath $relayPath -Arguments @(
        "provision", "--host", $hostID, "--token-out", $tokenFile
    ))
    $relayBase = "http://127.0.0.1:$relayPort"
    $relayProcess = Start-SmokeProcess -Label "relay" -FilePath $relayPath -Arguments @(
        "serve", "--listen", "127.0.0.1:$relayPort", "--external-url", $relayBase,
        "--allow-insecure-public"
    ) -TemporaryRoot $temporaryRoot
    $trackedProcesses += $relayProcess
    $relayHealthResponse = Wait-SmokeEndpoint -Uri "$relayBase/_ron1n/health" -TrackedProcess $relayProcess
    $relayHealthText = [Text.Encoding]::UTF8.GetString([byte[]]$relayHealthResponse.Body)
    $relayHealth = ConvertFrom-Json -InputObject $relayHealthText
    Assert-Smoke ([string]$relayHealth.status -ceq "ok") "Relay health status was not ok."
    Assert-Smoke ([string]$relayHealth.version -ceq $ExpectedVersion) "Relay health version was not $ExpectedVersion."

    Write-SmokeStep "Starting the authenticated outbound agent"
    [void](Invoke-SmokeNative -FilePath $ron1nPath -Arguments @(
        "relay", "configure", "--url", $relayBase, "--host-id", $hostID,
        "--token-file", $tokenFile, "--allow-http"
    ))
    $agentProcess = Start-SmokeProcess -Label "agent" -FilePath $ron1nPath `
        -Arguments @("relay", "connect", "--allow-http") -TemporaryRoot $temporaryRoot
    $trackedProcesses += $agentProcess
    Start-Sleep -Milliseconds 500
    $agentProcess.Process.Refresh()
    if ($agentProcess.Process.HasExited) {
        throw ("Outbound agent exited early.`n" + (Get-SmokeProcessDiagnostics $agentProcess))
    }

    Write-SmokeStep "Creating an expiring capability and fetching GoldHEN through the relay"
    $sessionOutput = Invoke-SmokeNative -FilePath $ron1nPath -Arguments @(
        "relay", "session", "--ttl", "1m", "--allow-http"
    )
    $sessionLines = @($sessionOutput -split "`r?`n")
    $idLine = @($sessionLines | Where-Object { $_ -match '^Session ID:\s+(\S+)\s*$' })
    $rootLine = @($sessionLines | Where-Object { $_ -match '^Root:\s+(\S+)\s*$' })
    $expiryLine = @($sessionLines | Where-Object { $_ -match '^Session expires:\s+(\S+)\s*$' })
    Assert-Smoke ($idLine.Count -eq 1) "Relay session output did not contain exactly one session ID."
    Assert-Smoke ($rootLine.Count -eq 1) "Relay session output did not contain exactly one capability root."
    Assert-Smoke ($expiryLine.Count -eq 1) "Relay session output did not contain exactly one expiry."

    $idMatch = [Regex]::Match($idLine[0], '^Session ID:\s+(\S+)\s*$')
    $rootMatch = [Regex]::Match($rootLine[0], '^Root:\s+(\S+)\s*$')
    $expiryMatch = [Regex]::Match($expiryLine[0], '^Session expires:\s+(\S+)\s*$')
    $sessionID = $idMatch.Groups[1].Value
    Assert-Smoke ($sessionID -match '^[A-Za-z0-9_-]{16,}$') "Relay session ID was malformed."
    $capabilityRoot = $rootMatch.Groups[1].Value
    $capabilityUri = [Uri]$capabilityRoot
    Assert-Smoke ($capabilityUri.Scheme -ceq "http") "Loopback capability did not use the expected development HTTP scheme."
    Assert-Smoke ($capabilityUri.Host -ceq "127.0.0.1") "Capability URL was not loopback-only."
    Assert-Smoke ($capabilityUri.Port -eq $relayPort) "Capability URL used the wrong relay port."
    Assert-Smoke ($capabilityUri.AbsolutePath -match '^/s/[A-Za-z0-9_-]{32,}/$') "Capability URL path was malformed."

    $expiresAt = [DateTimeOffset]::Parse($expiryMatch.Groups[1].Value, [Globalization.CultureInfo]::InvariantCulture)
    $now = [DateTimeOffset]::UtcNow
    Assert-Smoke ($expiresAt -gt $now) "Capability was already expired."
    Assert-Smoke ($expiresAt -le $now.AddMinutes(2)) "Capability expiry exceeded the requested one-minute test TTL."

    $relayGoldHEN = Invoke-SmokeHttp -Uri ($capabilityRoot + "goldhen.bin") -TimeoutSeconds 90
    Assert-Smoke ($relayGoldHEN.StatusCode -eq 200) "Relayed goldhen.bin did not return HTTP 200."
    $relayGoldHENFile = Join-Path $temporaryRoot "relay-goldhen.bin"
    [IO.File]::WriteAllBytes($relayGoldHENFile, [byte[]]$relayGoldHEN.Body)
    $relayDigest = (Get-FileHash -Algorithm SHA256 -LiteralPath $relayGoldHENFile).Hash.ToLowerInvariant()
    Assert-Smoke ($relayDigest -ceq $ExpectedGoldHENSHA256) "Relayed goldhen.bin SHA-256 was $relayDigest."

    Write-SmokeStep "Revoking the capability and confirming it fails closed"
    [void](Invoke-SmokeNative -FilePath $ron1nPath -Arguments @(
        "relay", "revoke", "--session", $sessionID, "--allow-http"
    ))
    $revokedStatus = Get-SmokeHttpStatus -Uri ($capabilityRoot + "goldhen.bin")
    Assert-Smoke ($revokedStatus -eq 404) "Revoked capability returned HTTP $revokedStatus instead of 404."

    $policyAfter = Get-SmokePolicySnapshot
    Assert-Smoke ($policyAfter -ceq $policyBefore) "PowerShell execution-policy state changed during the smoke test."
    $testPassed = $true
} catch {
    $failure = $_.Exception
} finally {
    for ($index = $trackedProcesses.Count - 1; $index -ge 0; $index--) {
        $tracked = $trackedProcesses[$index]
        try {
            $tracked.Process.Refresh()
            if (-not $tracked.Process.HasExited) {
                Stop-Process -Id $tracked.Process.Id -Force -ErrorAction Stop
                [void]$tracked.Process.WaitForExit(5000)
            }
            $tracked.Process.Dispose()
        } catch {
            if ($null -eq $cleanupFailure) {
                $cleanupFailure = $_.Exception
            }
        }
    }

    try {
        Restore-SmokeEnvironment -Snapshot $environmentBefore
    } catch {
        if ($null -eq $cleanupFailure) {
            $cleanupFailure = $_.Exception
        }
    }

    for ($attempt = 1; $attempt -le 3 -and (Test-Path -LiteralPath $temporaryRoot); $attempt++) {
        try {
            Remove-Item -Recurse -Force -LiteralPath $temporaryRoot -ErrorAction Stop
        } catch {
            if ($attempt -eq 3) {
                $cleanupFailure = $_.Exception
            } else {
                Start-Sleep -Milliseconds 300
            }
        }
    }
}

if ($null -ne $cleanupFailure -and $null -eq $failure) {
    $failure = $cleanupFailure
}
if ($null -ne $failure -or -not $testPassed) {
    $message = if ($null -ne $failure) { $failure.Message } else { "Smoke test did not complete." }
    Write-Host ("FAIL: ron1n Windows VM smoke test: " + $message) -ForegroundColor Red
    throw "ron1n Windows VM smoke test failed."
}

Write-Host "PASS: ron1n Windows VM smoke test" -ForegroundColor Green
Write-Host "  - ron1n and ron1n-relay report exactly 0.0.1zoro"
Write-Host "  - pinned content installed in an isolated temporary profile"
Write-Host "  - local health, cache MIME, full GoldHEN SHA-256, and 32-byte range passed"
Write-Host "  - expiring capability delivered identical bytes and failed closed after authenticated revocation"
Write-Host "  - tracked processes stopped; temporary state removed; execution policy unchanged"
