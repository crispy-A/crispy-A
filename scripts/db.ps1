# 本地 PostgreSQL 17 启停脚本（便携二进制，不注册 Windows 服务）。
# 用法：powershell -File scripts/db.ps1 start|stop|status
param(
    [ValidateSet('start', 'stop', 'status')]
    [string]$Action = 'start'
)

$pgBin = if ($env:PG_BIN) { $env:PG_BIN } else { 'D:\crispy\tools\pgsql\bin' }
$pgData = if ($env:PGDATA) { $env:PGDATA } else { 'D:\crispy\pgdata' }

$pgCtl = Join-Path $pgBin 'pg_ctl.exe'
if (-not (Test-Path $pgCtl)) {
    throw "找不到 pg_ctl：$pgCtl。请通过 PG_BIN 环境变量指向 PostgreSQL bin 目录。"
}
if (-not (Test-Path (Join-Path $pgData 'PG_VERSION'))) {
    throw "PGDATA 尚未初始化：$pgData。"
}

switch ($Action) {
    'start' {
        & $pgCtl status -D $pgData *> $null
        if ($LASTEXITCODE -eq 0) {
            Write-Output "PostgreSQL 已在运行：$pgData"
            break
        }
        $logFile = Join-Path $pgData 'server.log'
        & $pgCtl start -D $pgData -l $logFile -w -t 60
        if ($LASTEXITCODE -ne 0) {
            throw "PostgreSQL 启动失败，请查看日志：$logFile"
        }
        Write-Output "PostgreSQL 已启动：$pgData"
    }
    'stop' {
        & $pgCtl stop -D $pgData -m fast -w -t 60
        if ($LASTEXITCODE -ne 0) {
            throw "PostgreSQL 停止失败。"
        }
        Write-Output "PostgreSQL 已停止。"
    }
    'status' {
        & $pgCtl status -D $pgData
    }
}
