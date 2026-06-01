#Requires -Version 5.1
# Kizunax installer for Windows (PowerShell)
$ErrorActionPreference = 'Stop'

$RepoDir     = (Resolve-Path -Path $PSScriptRoot).Path
$VersionFile = Join-Path $RepoDir 'internal\cli\cli.go'
$Settings    = Join-Path $env:USERPROFILE '.claude\settings.json'
$Binary      = Join-Path $RepoDir 'plugins\kizunax\bin\kizunax.exe'
$BackupDir   = Join-Path $env:USERPROFILE '.claude\.kizunax-backups'
$ReleaseBase = 'https://github.com/thanhhaudev/kizunax-plugin-cc/releases/download'

$script:BackedUpSettings    = $null
$script:BackedUpBinary      = $null
$script:NewBinaryWritten    = $false
$script:SettingsPreexisted  = $false

function Info($msg) { Write-Host "==> $msg" }
function Fail($msg) { Write-Host "ERROR: $msg" -ForegroundColor Red; throw $msg }

function Read-Version {
    if (-not (Test-Path $VersionFile)) {
        Fail "Cannot find $VersionFile. Run install.ps1 from inside the kizunax-plugin-cc repo."
    }
    $match = Select-String -Path $VersionFile -Pattern '^const Version = "([^"]+)"' | Select-Object -First 1
    if (-not $match) { Fail "Failed to parse Version from $VersionFile" }
    return $match.Matches[0].Groups[1].Value
}

function Detect-Arch {
    $arch = $env:PROCESSOR_ARCHITECTURE
    switch ($arch) {
        'AMD64' { return 'amd64' }
        'ARM64' { return 'arm64' }
        default { Fail "Unsupported architecture: $arch" }
    }
}

function Backup-State {
    $ts = Get-Date -Format 'yyyyMMdd-HHmmss'
    if (Test-Path $Settings) {
        $dest = Join-Path $BackupDir "settings.json.$ts"
        Copy-Item $Settings $dest
        $script:BackedUpSettings = $dest
        Info "Backed up settings -> $dest"
    }
    if (Test-Path $Binary) {
        $dest = Join-Path $BackupDir "kizunax.bin.$ts"
        Copy-Item $Binary $dest
        $script:BackedUpBinary = $dest
        Info "Backed up binary  -> $dest"
    }
}

function Invoke-Rollback {
    Write-Host "ERROR: Aborted - reverting changes..." -ForegroundColor Red
    if ($script:BackedUpSettings -and (Test-Path $script:BackedUpSettings)) {
        try { Copy-Item $script:BackedUpSettings $Settings -Force; Info "Restored $Settings" }
        catch { Write-Host "ERROR: FAILED to restore settings (original at $script:BackedUpSettings)" -ForegroundColor Red }
    }
    if ($script:NewBinaryWritten) {
        if ($script:BackedUpBinary -and (Test-Path $script:BackedUpBinary)) {
            try { Copy-Item $script:BackedUpBinary $Binary -Force; Info "Restored $Binary" }
            catch { Write-Host "ERROR: FAILED to restore binary (original at $script:BackedUpBinary)" -ForegroundColor Red }
        } else {
            Remove-Item $Binary -Force -ErrorAction SilentlyContinue
            Info "Removed partially-written binary"
        }
    }
    Remove-Item "$Binary.download.tmp" -Force -ErrorAction SilentlyContinue
    Remove-Item "$Settings.tmp"        -Force -ErrorAction SilentlyContinue
    if (-not $script:SettingsPreexisted -and (Test-Path $Settings)) {
        Remove-Item $Settings -Force -ErrorAction SilentlyContinue
        Info "Removed $Settings (did not exist before install)"
    }
    Write-Host "ERROR: Rollback complete. Backups retained at $BackupDir\" -ForegroundColor Red
}

function Download-Binary {
    param([string]$Version, [string]$Platform)
    $url    = "$ReleaseBase/v$Version/kizunax-$Platform.exe"
    $shaUrl = "$url.sha256"
    $tmp    = "$Binary.download.tmp"

    Info "Trying $url"
    try {
        Invoke-WebRequest -Uri $url -OutFile $tmp -UseBasicParsing -ErrorAction Stop
    } catch {
        Remove-Item $tmp -Force -ErrorAction SilentlyContinue
        return $false
    }

    $shaContent = $null
    try {
        $shaContent = (Invoke-WebRequest -Uri $shaUrl -UseBasicParsing -ErrorAction Stop).Content
    } catch {
        Info "No .sha256 published - skipping checksum check"
    }

    if ($shaContent) {
        $expected = ($shaContent -split '\s+')[0].ToLower()
        $actual   = (Get-FileHash -Path $tmp -Algorithm SHA256).Hash.ToLower()
        if ($expected -ne $actual) {
            Remove-Item $tmp -Force -ErrorAction SilentlyContinue
            Write-Host "ERROR: SHA256 mismatch on downloaded binary (expected $expected, got $actual)" -ForegroundColor Red
            Write-Host "ERROR: Aborting install - possible corrupted download or tampering. Not falling back to local build." -ForegroundColor Red
            exit 1
        }
        Info "SHA256 verified"
    }

    Move-Item $tmp $Binary -Force
    $script:NewBinaryWritten = $true
    return $true
}

function Test-GoVersionOk {
    $goCmd = Get-Command go -ErrorAction SilentlyContinue
    if (-not $goCmd) { return $false }
    try {
        $line = & go version
        if ($line -match 'go(\d+)\.(\d+)') {
            $major = [int]$matches[1]
            $minor = [int]$matches[2]
            return ($major -gt 1) -or ($major -eq 1 -and $minor -ge 21)
        }
    } catch {}
    return $false
}

function Build-BinaryLocal {
    param([string]$Version, [string]$Platform)
    if (-not (Test-GoVersionOk)) {
        Fail "No release binary for v$Version on $Platform, and Go >=1.21 not found. Install Go from https://go.dev/dl/ or wait for the v$Version release."
    }
    Info "Building locally with $(go version)"
    Push-Location $RepoDir
    try {
        $env:GOOS         = 'windows'
        $env:GOARCH       = (Detect-Arch)
        $env:CGO_ENABLED  = '0'
        & go build -trimpath -ldflags='-s -w' -o $Binary ./cmd/kizunax
        if ($LASTEXITCODE -ne 0) { Fail "go build failed" }
    } finally {
        Pop-Location
    }
    $script:NewBinaryWritten = $true
}

function Invoke-SmokeTest {
    param([string]$Version)
    $out = (& $Binary --version 2>&1) -join "`n"
    if ($out.Trim() -ne "kizunax $Version") {
        Fail "Smoke test failed: expected 'kizunax $Version', got '$out'"
    }
    Info "Smoke test passed: $out"
}

function Patch-Settings {
    $tmp = "$Settings.tmp"
    if (-not (Test-Path $Settings)) {
        '{}' | Set-Content -Path $Settings -NoNewline
    }
    $data = Get-Content -Raw $Settings | ConvertFrom-Json
    if (-not $data.PSObject.Properties['enabledPlugins']) {
        $data | Add-Member -NotePropertyName enabledPlugins -NotePropertyValue ([pscustomobject]@{}) -Force
    }
    if (-not $data.PSObject.Properties['extraKnownMarketplaces']) {
        $data | Add-Member -NotePropertyName extraKnownMarketplaces -NotePropertyValue ([pscustomobject]@{}) -Force
    }
    $data.enabledPlugins | Add-Member -NotePropertyName 'kizunax@kizunax-local' -NotePropertyValue $true -Force
    $entry = [pscustomobject]@{
        source = [pscustomobject]@{
            source = 'directory'
            path   = $RepoDir
        }
    }
    $data.extraKnownMarketplaces | Add-Member -NotePropertyName 'kizunax-local' -NotePropertyValue $entry -Force

    $json = $data | ConvertTo-Json -Depth 20
    $json | Set-Content -Path $tmp
    Get-Content -Raw $tmp | ConvertFrom-Json | Out-Null
    Move-Item $tmp $Settings -Force
    Info "Patched $Settings"
}

# ---------- Main ----------

try {
    if (-not (Test-Path $VersionFile)) {
        Fail "Run install.ps1 from inside the kizunax-plugin-cc repo."
    }

    $platform = "windows-$(Detect-Arch)"
    $version  = Read-Version

    if (Test-Path $Settings) {
        try { Get-Content -Raw $Settings | ConvertFrom-Json | Out-Null }
        catch { Fail "$Settings is not valid JSON. Fix it manually, then re-run." }
        $script:SettingsPreexisted = $true
    }

    New-Item -ItemType Directory -Force -Path $BackupDir              | Out-Null
    New-Item -ItemType Directory -Force -Path (Split-Path $Binary)    | Out-Null

    Info "Platform: $platform"
    Info "Version:  $version"
    Info "Settings: $Settings"

    Backup-State

    if (-not (Download-Binary -Version $version -Platform $platform)) {
        Info "Falling back to local go build"
        Build-BinaryLocal -Version $version -Platform $platform
    } else {
        Info "Downloaded pre-built binary"
    }

    Invoke-SmokeTest -Version $version
    Patch-Settings

    Write-Host ""
    Write-Host "Kizunax v$version installed."
    Write-Host "  Binary:   $Binary"
    Write-Host "  Settings: $Settings"
    if ($script:BackedUpSettings -or $script:BackedUpBinary) {
        Write-Host "  Backups:  $BackupDir\   (delete after verifying things work)"
    }
    Write-Host ""
    Write-Host "Next steps:"
    Write-Host "  1. Restart Claude Code if it is running."
    Write-Host "  2. Run /kizunax:setup to configure provider, model, and API key."
    exit 0
} catch {
    Invoke-Rollback
    exit 1
}
