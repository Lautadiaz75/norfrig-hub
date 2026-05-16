# Instala Norfrig Hub como tarea programada que arranca al iniciar sesión.
# Ejecutar como Administrador: clic derecho → "Ejecutar con PowerShell"

$taskName  = "NorfrigHub"
$hubDir    = Split-Path -Parent $MyInvocation.MyCommand.Path
$hubExe    = Join-Path $hubDir "hub.exe"

if (-not (Test-Path $hubExe)) {
    Write-Error "No se encontró hub.exe en $hubDir. Ejecutá primero build.bat."
    exit 1
}

$action   = New-ScheduledTaskAction -Execute $hubExe -Argument "serve" -WorkingDirectory $hubDir
$trigger  = New-ScheduledTaskTrigger -AtLogOn
$settings = New-ScheduledTaskSettingsSet -ExecutionTimeLimit 0 -RestartCount 3 `
              -RestartInterval (New-TimeSpan -Minutes 2)
$principal = New-ScheduledTaskPrincipal -UserId $env:USERNAME -LogonType Interactive -RunLevel Highest

Register-ScheduledTask `
    -TaskName  $taskName `
    -Action    $action `
    -Trigger   $trigger `
    -Settings  $settings `
    -Principal $principal `
    -Force | Out-Null

Write-Host "✓ Tarea '$taskName' instalada. El Hub arrancará automáticamente al iniciar sesión."
Write-Host "  Para iniciarlo ahora sin reiniciar:"
Write-Host "  Start-ScheduledTask -TaskName '$taskName'"
