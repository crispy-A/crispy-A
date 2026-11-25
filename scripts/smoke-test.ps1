# 后端接口冒烟测试：需先启动后端（go run ./cmd/server）。
# 用法：powershell -File scripts/smoke-test.ps1
param(
    [string]$BaseUrl = 'http://127.0.0.1:8080',
    [string]$Email = $env:ADMIN_EMAIL,
    [string]$Password = $env:ADMIN_PASSWORD
)

$ErrorActionPreference = 'Stop'

function Write-Step([string]$Text) {
    Write-Output "`n== $Text =="
}

Write-Step 'GET /api/health'
$health = Invoke-RestMethod -Uri "$BaseUrl/api/health" -Method Get
$health | ConvertTo-Json -Depth 4

if (-not $Email -or -not $Password) {
    Write-Output '未提供 ADMIN_EMAIL / ADMIN_PASSWORD，跳过登录相关用例。'
    return
}

Write-Step 'POST /api/auth/login'
$session = New-Object Microsoft.PowerShell.Commands.WebRequestSession
$loginBody = @{ email = $Email; password = $Password } | ConvertTo-Json
$login = Invoke-RestMethod -Uri "$BaseUrl/api/auth/login" -Method Post -Body $loginBody `
    -ContentType 'application/json' -WebSession $session
$login | ConvertTo-Json -Depth 4

Write-Step 'GET /api/admin/me（带 Bearer token）'
$me = Invoke-RestMethod -Uri "$BaseUrl/api/admin/me" -Method Get -WebSession $session -Headers @{
    Authorization = "Bearer $($login.accessToken)"
}
$me | ConvertTo-Json -Depth 4

Write-Step 'POST /api/auth/refresh（轮换 refresh cookie）'
$refreshed = Invoke-RestMethod -Uri "$BaseUrl/api/auth/refresh" -Method Post -WebSession $session
$refreshed | ConvertTo-Json -Depth 4

Write-Step 'POST /api/auth/logout'
Invoke-RestMethod -Uri "$BaseUrl/api/auth/logout" -Method Post -WebSession $session | Out-Null
Write-Output 'logout: 204 No Content'
