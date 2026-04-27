#!/usr/bin/env pwsh
# Detiene todos los servicios go-saas-api y el frontend

Write-Host "🛑 Deteniendo servicios Go..." -ForegroundColor Yellow

$nombres = @(
    "api-gateway.exe",
    "auth-service.exe",
    "agent-service.exe",
    "billing-service.exe",
    "usage-service.exe",
    "sandbox-service.exe"
)

foreach ($n in $nombres) {
    Get-Process | Where-Object { $_.ProcessName -eq $n.Replace(".exe","") } | Stop-Process -Force -ErrorAction SilentlyContinue
}

# Detener proceso npm (frontend)
Get-Process | Where-Object { $_.ProcessName -eq "node" -and $_.CommandLine -like "*r3-chat*" } | Stop-Process -Force -ErrorAction SilentlyContinue

Write-Host "✅ Todo detenido" -ForegroundColor Green
