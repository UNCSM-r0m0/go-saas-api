#!/usr/bin/env pwsh
# Detiene todos los servicios go-saas-api y el frontend

Write-Host "Deteniendo servicios Go..." -ForegroundColor Yellow

$processNames = @(
    "api-gateway",
    "auth-service",
    "agent-service",
    "billing-service",
    "usage-service",
    "sandbox-service"
)

foreach ($name in $processNames) {
    $procs = Get-Process -Name $name -ErrorAction SilentlyContinue
    if ($procs) {
        $procs | Stop-Process -Force
        Write-Host "  Parado $name"
    }
}

# Detener proceso npm (frontend)
$nodeProcs = Get-Process -Name "node" -ErrorAction SilentlyContinue | Where-Object {
    $_.CommandLine -like "*r3-chat*" -or $_.CommandLine -like "*vite*"
}
if ($nodeProcs) {
    $nodeProcs | Stop-Process -Force
    Write-Host "  Parado frontend (node/vite)"
}

Write-Host "Todo detenido" -ForegroundColor Green
