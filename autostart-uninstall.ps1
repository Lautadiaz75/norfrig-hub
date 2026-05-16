# Desinstala la tarea programada de Norfrig Hub.

$taskName = "NorfrigHub"
Stop-ScheduledTask  -TaskName $taskName -ErrorAction SilentlyContinue
Unregister-ScheduledTask -TaskName $taskName -Confirm:$false -ErrorAction SilentlyContinue
Write-Host "✓ Tarea '$taskName' eliminada."
