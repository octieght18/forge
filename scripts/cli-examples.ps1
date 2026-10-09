#requires -Version 7.0
param(
    [string]$TokenFile = '\\wsl.localhost\Ubuntu\home\owner\.local\share\forge-native\tokens\access.json'
)
$ErrorActionPreference = 'Stop'
$PSNativeCommandUseErrorActionPreference = $false
$repo = Split-Path -Parent $PSScriptRoot
$suffix = ''
if ($IsWindows) { $suffix = '.exe' }
$bin = Join-Path ([System.IO.Path]::GetTempPath()) ('forge-cli-' + [guid]::NewGuid().ToString('N') + $suffix)
$spec = Join-Path $repo 'internal/contract/examples/create-version.json'

function Invoke-Forge {
    param([Parameter(Mandatory = $true)][int]$ExpectedExit, [Parameter(Mandatory = $true)][string[]]$Arguments)
    $stdout = Join-Path ([System.IO.Path]::GetTempPath()) ([guid]::NewGuid().ToString('N') + '.out')
    $stderr = Join-Path ([System.IO.Path]::GetTempPath()) ([guid]::NewGuid().ToString('N') + '.err')
    try {
        & $bin @Arguments 1> $stdout 2> $stderr
        if ($LASTEXITCODE -ne $ExpectedExit) {
            throw "Unexpected CLI exit $LASTEXITCODE for $($Arguments[0]); expected $ExpectedExit"
        }
        $payload = if ($ExpectedExit -eq 0) { Get-Content -LiteralPath $stdout -Raw } else { Get-Content -LiteralPath $stderr -Raw }
        if ([string]::IsNullOrWhiteSpace($payload)) { throw "CLI command $($Arguments[0]) produced no JSON" }
        return ($payload | ConvertFrom-Json)
    } finally {
        Remove-Item -LiteralPath $stdout, $stderr -Force -ErrorAction SilentlyContinue
    }
}

Push-Location $repo
try {
    & go build -o $bin ./cmd/forge
    if ($LASTEXITCODE -ne 0) { throw 'Developer CLI build failed' }
    $name = 'cli-' + [guid]::NewGuid().ToString('N')
    $registered = Invoke-Forge 0 @('register', '--token-file', $TokenFile, '--name', $name, '--description', 'Developer CLI example')
    $workloadId = [string]$registered.workload.workload_id
    if ($registered.operation -ne 'register' -or $registered.status -ne 201 -or $registered.request_id.Length -ne 32 -or !$workloadId) {
        throw 'Register did not return an operation ID and workload'
    }
    $deployed = Invoke-Forge 0 @('deploy', '--token-file', $TokenFile, '--workload', $workloadId, '--spec', $spec)
    $versionId = [string]$deployed.version.version_id
    if ($deployed.operation -ne 'deploy' -or $deployed.status -ne 201 -or $deployed.request_id.Length -ne 32 -or !$versionId -or $deployed.execution -ne 'not_started' -or $deployed.environment -ne 'not_requested') {
        throw 'Deploy did not record an immutable version without starting execution'
    }
    $version = Invoke-Forge 0 @('status', '--token-file', $TokenFile, '--workload', $workloadId, '--version', $versionId)
    if ($version.operation -ne 'status' -or $version.status -ne 200 -or $version.request_id.Length -ne 32 -or [string]$version.version.version_id -ne $versionId) {
        throw 'Status did not read the deployed version'
    }
    $removed = Invoke-Forge 1 @('delete', '--token-file', $TokenFile, '--workload', $workloadId)
    if ($removed.operation -ne 'delete' -or $removed.status -ne 405 -or $removed.error.code -ne 'method_not_allowed' -or $removed.request_id.Length -ne 32 -or !$removed.hint) {
        throw 'Delete did not report the API rejection and operation ID'
    }
    $stillThere = Invoke-Forge 0 @('status', '--token-file', $TokenFile, '--workload', $workloadId)
    if ([string]$stillThere.workload.workload_id -ne $workloadId) {
        throw 'Rejected delete removed or hid the workload'
    }
    [pscustomobject]@{ workload_id = $workloadId; version_id = $versionId; delete_rejected = $true; cli_examples_verified = $true }
} finally {
    Pop-Location
    if (Test-Path -LiteralPath $bin) { Remove-Item -LiteralPath $bin -Force }
}
