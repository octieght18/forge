#requires -Version 7.0
param(
    [int]$Port = 7233
)
$ErrorActionPreference = 'Stop'
$temporal = Get-Command temporal -ErrorAction SilentlyContinue
if (-not $temporal) {
    throw 'The Temporal CLI is not installed. Install the pinned dev-server CLI, then rerun this script.'
}
$state = Join-Path $env:TEMP 'forge-temporal-dev.db'
& temporal server start-dev --port $Port --headless --db-filename $state
if ($LASTEXITCODE -ne 0) { throw 'Temporal dev server exited' }
