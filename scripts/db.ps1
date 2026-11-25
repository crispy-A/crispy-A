# Start / stop the local PostgreSQL 17 instance (portable binaries, no Windows service).
# Keep this file ASCII-only: Windows PowerShell 5.1 decodes BOM-less UTF-8 as ANSI,
# and non-ASCII bytes can corrupt the parser.
# Usage: powershell -File scripts/db.ps1 start|stop|status
param(
    [ValidateSet('start', 'stop', 'status')]
    [string]$Action = 'start'
)

$pgBin = if ($env:PG_BIN) { $env:PG_BIN } else { 'D:\crispy\tools\pgsql\bin' }
$pgData = if ($env:PGDATA) { $env:PGDATA } else { 'D:\crispy\pgdata' }

$pgCtl = Join-Path $pgBin 'pg_ctl.exe'
if (-not (Test-Path $pgCtl)) {
    throw "pg_ctl not found: $pgCtl. Set PG_BIN to the PostgreSQL bin directory."
}
if (-not (Test-Path (Join-Path $pgData 'PG_VERSION'))) {
    throw "PGDATA is not initialized: $pgData."
}

switch ($Action) {
    'start' {
        & $pgCtl status -D $pgData *> $null
        if ($LASTEXITCODE -eq 0) {
            Write-Output "PostgreSQL already running: $pgData"
            break
        }

        $logFile = Join-Path $pgData 'server.log'
        # postgres inherits its parent stdio. Calling pg_ctl directly keeps the caller's
        # pipe open, so a captured run looks hung. Start it detached, then poll status.
        $argumentLine = 'start -D "{0}" -l "{1}" -w -t 60' -f $pgData, $logFile
        Start-Process -FilePath $pgCtl -ArgumentList $argumentLine -WindowStyle Hidden | Out-Null

        $ready = $false
        for ($attempt = 0; $attempt -lt 120; $attempt++) {
            Start-Sleep -Milliseconds 500
            & $pgCtl status -D $pgData *> $null
            if ($LASTEXITCODE -eq 0) {
                $ready = $true
                break
            }
        }
        if (-not $ready) {
            throw "PostgreSQL failed to start within 60 seconds. Check log: $logFile"
        }
        Write-Output "PostgreSQL started: $pgData"
    }
    'stop' {
        & $pgCtl stop -D $pgData -m fast -w -t 60
        if ($LASTEXITCODE -ne 0) {
            throw 'PostgreSQL failed to stop.'
        }
        Write-Output 'PostgreSQL stopped.'
    }
    'status' {
        & $pgCtl status -D $pgData
    }
}
