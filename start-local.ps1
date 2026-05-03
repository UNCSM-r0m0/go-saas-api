#!/usr/bin/env pwsh
# Levanta todo el stack go-saas-api + r3-chat frontend en localhost
# Puertos: gateway 3000, auth 3001, agent 3002, billing 3003, usage 3004, sandbox 3006

$ErrorActionPreference = "Stop"
$root = "D:\WORKSPACES\GO\go-saas-api"
$frontend = "$root\r3-chat"

# ===================================================================
# MATAR PROCESOS VIEJOS
# ===================================================================
Write-Host "Deteniendo servicios anteriores..." -ForegroundColor Cyan

$serviceNames = @("auth-service", "agent-service", "billing-service", "usage-service", "sandbox-service", "api-gateway")
foreach ($svc in $serviceNames) {
    $procs = Get-Process -Name $svc -ErrorAction SilentlyContinue
    if ($procs) {
        $procs | Stop-Process -Force
        Write-Host "  Parado $svc"
    }
}

$nodeProcs = Get-Process -Name "node" -ErrorAction SilentlyContinue | Where-Object {
    $_.CommandLine -like "*r3-chat*" -or $_.CommandLine -like "*vite*"
}
if ($nodeProcs) {
    $nodeProcs | Stop-Process -Force
    Write-Host "  Parado frontend (node/vite)"
}

Start-Sleep -Milliseconds 500

# ===================================================================
# CARGAR .env COMO VARIABLES DE ENTORNO
# ===================================================================
$envVars = @{}
$envFile = "$root\.env"
if (Test-Path $envFile) {
    $envCount = 0
    Get-Content $envFile | ForEach-Object {
        $line = $_.Trim()
        if ($line -and -not $line.StartsWith("#")) {
            $parts = $line -split "=", 2
            if ($parts.Length -eq 2) {
                $key = $parts[0].Trim()
                $val = $parts[1].Trim()
                # Strip surrounding quotes
                $val = $val.Trim('"').Trim("'")
                $envVars[$key] = $val
                [Environment]::SetEnvironmentVariable($key, $val, "Process")
                $envCount++
            }
        }
    }
    Write-Host "Variables cargadas desde .env ($envCount vars)" -ForegroundColor Green
} else {
    Write-Host "WARN: .env no encontrado en $envFile" -ForegroundColor Red
}

# ===================================================================
# CREAR CARPETAS
# ===================================================================
$logDir = "$root\logs"
$tmpDir = "$root\tmp"
New-Item -ItemType Directory -Force -Path $logDir | Out-Null
New-Item -ItemType Directory -Force -Path $tmpDir | Out-Null

# ===================================================================
# COMPILAR SERVICIOS
# ===================================================================
Write-Host "`nCompilando servicios Go..." -ForegroundColor Cyan

$services = @(
    @{ Name = "auth-service";    Path = "cmd/auth-service" },
    @{ Name = "agent-service";   Path = "cmd/agent-service" },
    @{ Name = "billing-service"; Path = "cmd/billing-service" },
    @{ Name = "usage-service";   Path = "cmd/usage-service" },
    @{ Name = "sandbox-service"; Path = "cmd/sandbox-service" },
    @{ Name = "api-gateway";     Path = "cmd/api-gateway" }
)

$buildErrors = @()
foreach ($svc in $services) {
    $out = "$tmpDir\$($svc.Name).exe"
    $src = "$root\$($svc.Path)"
    Write-Host "  BUILD $($svc.Name)..." -NoNewline
    $result = go build -o $out $src 2>&1
    if (Test-Path $out) {
        Write-Host " OK" -ForegroundColor Green
    } else {
        Write-Host " FAIL" -ForegroundColor Red
        $buildErrors += "  $($svc.Name): $result"
    }
}

if ($buildErrors.Count -gt 0) {
    Write-Host "`nErrores de compilacion:" -ForegroundColor Red
    $buildErrors | ForEach-Object { Write-Host $_ }
    exit 1
}

# ===================================================================
# VERIFICAR INFRAESTRUCTURA DOCKER
# ===================================================================
Write-Host "`nVerificando infraestructura..." -ForegroundColor Cyan

$pgRunning = docker ps --format "{{.Names}}" 2>$null | Select-String "saas-postgres"
if (-not $pgRunning) {
    Write-Host "  Levantando Postgres, Redis, NATS..." -ForegroundColor Yellow
    Push-Location $root
    docker-compose up -d postgres redis nats 2>$null
    Pop-Location
    Start-Sleep -Seconds 5
} else {
    Write-Host "  OK Postgres, Redis, NATS" -ForegroundColor Green
}

# ===================================================================
# LANZAR SERVICIOS GO (heredando env vars del proceso + PORT)
# ===================================================================
Write-Host "`nLevantando servicios Go..." -ForegroundColor Cyan

function Start-GoService($name, $port) {
    $exe = "$root\tmp\$name.exe"
    $log = "$logDir\$name.log"
    $errLog = "$logDir\$name.err.log"

    if (-not (Test-Path $exe)) {
        Write-Host "  ERROR $exe no encontrado" -ForegroundColor Red
        return
    }

    $svcEnv = $envVars.Clone()
    $svcEnv["PORT"] = "$port"

    Write-Host "  START $name (port $port)" -ForegroundColor Yellow
    Start-Process -FilePath $exe -WorkingDirectory $root -WindowStyle Hidden -RedirectStandardOutput $log -RedirectStandardError $errLog -Environment $svcEnv
}

Start-GoService "auth-service"    3001
Start-Sleep -Milliseconds 300
Start-GoService "agent-service"   3002
Start-Sleep -Milliseconds 300
Start-GoService "billing-service" 3003
Start-Sleep -Milliseconds 300
Start-GoService "usage-service"   3004
Start-Sleep -Milliseconds 300
Start-GoService "sandbox-service" 3006
Start-Sleep -Milliseconds 300
Start-GoService "api-gateway"     3000

Write-Host "`nEsperando 4 segundos a que el gateway inicie..." -ForegroundColor Cyan
Start-Sleep -Seconds 4

# Verificar gateway
try {
    $gw = Invoke-RestMethod -Uri "http://localhost:3000/health" -Method GET -ErrorAction Stop
    $gwSvc = if ($gw.service) { $gw.service } else { "gateway" }
    $gwPort = if ($gw.port) { $gw.port } else { "3000" }
    Write-Host "  Gateway OK ($gwSvc @ port $gwPort)" -ForegroundColor Green
} catch {
    Write-Host "  WARN: Gateway no responde (puede tardar unos segundos)" -ForegroundColor Yellow
}

# ===================================================================
# FRONTEND
# ===================================================================
Write-Host "`nLevantando frontend (r3-chat)..." -ForegroundColor Cyan
if (Test-Path "$frontend\package.json") {
    $frontendLog = "$logDir\frontend.log"
    $frontendErr = "$logDir\frontend.err.log"
    Start-Process -FilePath "cmd" -ArgumentList "/c","npm","run","dev" -WorkingDirectory $frontend -WindowStyle Hidden -RedirectStandardOutput $frontendLog -RedirectStandardError $frontendErr
    Write-Host "  Frontend iniciando en http://localhost:5173" -ForegroundColor Green
} else {
    Write-Host "  WARN: r3-chat no encontrado en $frontend" -ForegroundColor Yellow
}

Write-Host "`n========================================" -ForegroundColor Green
Write-Host "  TODO LEVANTADO!" -ForegroundColor Green
Write-Host "========================================" -ForegroundColor Green
Write-Host "  Frontend: http://localhost:5173"
Write-Host "  Gateway:  http://localhost:3000"
Write-Host "  Auth:     http://localhost:3001"
Write-Host "  Agent:    http://localhost:3002"
Write-Host "  Billing:  http://localhost:3003"
Write-Host "  Usage:    http://localhost:3004"
Write-Host "  Sandbox:  http://localhost:3006"
Write-Host ""
Write-Host "  Logs en: $logDir"
Write-Host "  Para detener: .\stop-local.ps1"