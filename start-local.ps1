#!/usr/bin/env pwsh
# Levanta todo el stack go-saas-api + r3-chat frontend en localhost
# Puertos: gateway 3000, auth 3001, agent 3002, billing 3003, usage 3004, sandbox 3005

$ErrorActionPreference = "Stop"
$root = "D:\WORKSPACES\GO\go-saas-api"
$frontend = "D:\WORKSPACES\NESTJS\chat\r3-chat"

# Cargar variables de entorno desde .env
$envFile = "$root\.env"
if (Test-Path $envFile) {
    Get-Content $envFile | ForEach-Object {
        $line = $_.Trim()
        if ($line -and -not $line.StartsWith("#")) {
            $parts = $line -split "=", 2
            if ($parts.Length -eq 2) {
                $key = $parts[0].Trim()
                $val = $parts[1].Trim()
                [Environment]::SetEnvironmentVariable($key, $val, "Process")
            }
        }
    }
    Write-Host "Variables cargadas desde .env"
} else {
    Write-Host "WARN: .env no encontrado en $envFile"
}

# Crear carpeta de logs
$logDir = "$root\logs"
New-Item -ItemType Directory -Force -Path $logDir | Out-Null

# Verificar que infraestructura este corriendo
Write-Host "Verificando infraestructura..."
try {
    $pg = docker ps --format "{{.Names}}" | Select-String "saas-postgres"
    if (-not $pg) { throw "Postgres no esta corriendo" }
    Write-Host "  OK Postgres"
} catch {
    Write-Host "  Levantando infraestructura con docker-compose..."
    docker-compose up -d postgres redis nats
    Start-Sleep -Seconds 5
}

# Funcion para lanzar servicio Go con PORT especifico
function Start-GoService($name, $port) {
    $exe = "$root\tmp\$name.exe"
    $log = "$logDir\$name.log"
    if (Test-Path $exe) {
        Write-Host "  START $name (port $port)"
        $errLog = "$logDir\$name.err.log"
        $env:PORT = $port
        Start-Process -FilePath $exe -WorkingDirectory $root -WindowStyle Hidden -RedirectStandardOutput $log -RedirectStandardError $errLog
        Remove-Item Env:\PORT -ErrorAction SilentlyContinue
    } else {
        Write-Host "  ERROR $exe no encontrado"
    }
}

Write-Host ""
Write-Host "Levantando servicios Go..."
Start-GoService "auth-service" 3001
Start-Sleep -Milliseconds 500
Start-GoService "agent-service" 3002
Start-Sleep -Milliseconds 500
Start-GoService "billing-service" 3003
Start-Sleep -Milliseconds 500
Start-GoService "usage-service" 3004
Start-Sleep -Milliseconds 500
Start-GoService "sandbox-service" 3005
Start-Sleep -Milliseconds 500
Start-GoService "api-gateway" 3000

Write-Host ""
Write-Host "Esperando 3 segundos a que el gateway inicie..."
Start-Sleep -Seconds 3

# Verificar gateway
try {
    $gw = Invoke-RestMethod -Uri "http://localhost:3000/health" -Method GET -ErrorAction Stop
    if ($gw.status -eq "healthy") {
        Write-Host "  OK Gateway responde"
    }
} catch {
    Write-Host "  WARN Gateway no responde todavia (puede tardar unos segundos mas)"
}

# Frontend
Write-Host ""
Write-Host "Levantando frontend (r3-chat)..."
$frontendLog = "$logDir\frontend.log"
$frontendErr = "$logDir\frontend.err.log"
Start-Process -FilePath "cmd" -ArgumentList "/c","npm","run","dev" -WorkingDirectory $frontend -WindowStyle Hidden -RedirectStandardOutput $frontendLog -RedirectStandardError $frontendErr

Write-Host ""
Write-Host "TODO LEVANTADO!"
Write-Host "   Frontend: http://localhost:5173"
Write-Host "   Gateway:  http://localhost:3000"
Write-Host "   Swagger:  http://localhost:3000/swagger/index.html"
Write-Host ""
Write-Host "Logs en: $logDir"
Write-Host "Para detener todo: ejecuta .\stop-local.ps1"
