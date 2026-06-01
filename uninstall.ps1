#Requires -Version 5.1
# Kizunax uninstaller for Windows (PowerShell)
$ErrorActionPreference = 'Stop'

$RepoDir   = (Resolve-Path -Path $PSScriptRoot).Path
$Settings  = Join-Path $env:USERPROFILE '.claude\settings.json'
$Binary    = Join-Path $RepoDir 'plugins\kizunax\bin\kizunax.exe'
$BackupDir = Join-Path $env:USERPROFILE '.claude\.kizunax-backups'

$script:BackedUpSettings = $null

function Info($msg) { Write-Host "==> $msg" }
function Fail($msg) { Write-Host "ERROR: $msg" -ForegroundColor Red; throw $msg }

function Invoke-Rollback {
    Write-Host "ERROR: Aborted - reverting settings..." -ForegroundColor Red
    if ($script:BackedUpSettings -and (Test-Path $script:BackedUpSettings)) {
        try { Copy-Item $script:BackedUpSettings $Settings -Force; Info "Restored $Settings" }
        catch { Write-Host "ERROR: FAILED to restore (original at $script:BackedUpSettings)" -ForegroundColor Red }
    }
    Remove-Item "$Settings.tmp" -Force -ErrorAction SilentlyContinue
}

try {
    if (Test-Path $Settings) {
        try { Get-Content -Raw $Settings | ConvertFrom-Json | Out-Null }
        catch { Fail "$Settings is not valid JSON." }
    }

    New-Item -ItemType Directory -Force -Path $BackupDir | Out-Null

    if (Test-Path $Settings) {
        $ts   = Get-Date -Format 'yyyyMMdd-HHmmss'
        $dest = Join-Path $BackupDir "settings.json.$ts"
        Copy-Item $Settings $dest
        $script:BackedUpSettings = $dest
        Info "Backed up settings -> $dest"

        $tmp  = "$Settings.tmp"
        $data = Get-Content -Raw $Settings | ConvertFrom-Json
        if ($data.PSObject.Properties['enabledPlugins'] -and $data.enabledPlugins) {
            $data.enabledPlugins.PSObject.Properties.Remove('kizunax@kizunax-local')
        }
        if ($data.PSObject.Properties['extraKnownMarketplaces'] -and $data.extraKnownMarketplaces) {
            $data.extraKnownMarketplaces.PSObject.Properties.Remove('kizunax-local')
        }
        $json = $data | ConvertTo-Json -Depth 20
        $json | Set-Content -Path $tmp
        Get-Content -Raw $tmp | ConvertFrom-Json | Out-Null
        Move-Item $tmp $Settings -Force
        Info "Removed kizunax keys from $Settings"
    } else {
        Info "No settings.json - nothing to clean up."
    }

    if (Test-Path $Binary) {
        try {
            Remove-Item $Binary -Force
            Info "Removed $Binary"
        } catch {
            Write-Host "WARN: Could not remove $Binary (continuing)" -ForegroundColor Yellow
        }
    }

    Write-Host ""
    Write-Host "Kizunax uninstalled."
    if ($script:BackedUpSettings) {
        Write-Host "  Settings backup: $script:BackedUpSettings"
    }
    Write-Host "  Config + state at $env:USERPROFILE\.kizunax\ preserved."
    Write-Host "  Run 'Remove-Item -Recurse $env:USERPROFILE\.kizunax' to delete those too."
    Write-Host "  Restart Claude Code if it is running."
    exit 0
} catch {
    Invoke-Rollback
    exit 1
}
