#requires -Version 7.0
param(
    [string]$TokenFile = '\\wsl.localhost\Ubuntu\home\owner\.local\share\forge-native\tokens\access.json'
)
$ErrorActionPreference = 'Stop'
$repo = Split-Path -Parent $PSScriptRoot
$login = Get-Content -LiteralPath $TokenFile -Raw | ConvertFrom-Json
if (!$login.access_token -or !$login.expires_at -or [DateTimeOffset]::Parse($login.expires_at) -le [DateTimeOffset]::UtcNow) {
    throw 'Access token missing or expired; complete a fresh local-stack.ps1 login first'
}
$headers = @{ Authorization = 'Bearer ' + $login.access_token }
$session = [Microsoft.PowerShell.Commands.WebRequestSession]::new()

function Request($method, $path, $expected, $body = $null, $etag = $null) {
    $displayPath = $path.Split('?')[0]
    $requestHeaders = @{} + $headers
    if ($etag) { $requestHeaders['If-Match'] = $etag }
    $parameters = @{ Uri=('http://127.0.0.1:8081' + $path); Method=$method; Headers=$requestHeaders;
        WebSession=$session; TimeoutSec=10; SkipHttpErrorCheck=$true }
    if ($null -ne $body) {
        $parameters.Body = [Text.Encoding]::UTF8.GetBytes(($body | ConvertTo-Json -Depth 20 -Compress))
        $parameters.ContentType = 'application/json'
    }
    $response = Invoke-WebRequest @parameters
    if ([int]$response.StatusCode -ne $expected -or !$response.Headers['X-Request-ID']) {
        throw "Unexpected status for $method $displayPath; expected $expected, received $($response.StatusCode). No token or response body printed."
    }
    Write-Host "$method $displayPath -> $expected"
    return [pscustomobject]@{ body=($response.Content | ConvertFrom-Json); etag=([string]($response.Headers['ETag'] | Select-Object -First 1)) }
}

try {
    # Each invocation creates one retained synthetic workload and version.
    # Names satisfy the actual lowercase-slug contract; do not retry POSTs blindly.
    $name = 'quickstart-' + [Guid]::NewGuid().ToString('N')
    $created = Request 'POST' '/api/v1/workloads' 201 @{name=$name; description='Quickstart API example'}
    $workloadId = $created.body.workload_id
    $path = '/api/v1/workloads/' + $workloadId
    $read = Request 'GET' $path 200
    if ($read.body.workload_id -ne $workloadId -or !$read.etag) { throw 'Workload readback or ETag missing' }
    $updated = Request 'PATCH' $path 200 @{description='Quickstart metadata updated using If-Match'} $read.etag
    if ($updated.body.revision -ne ($read.body.revision + 1)) { throw 'Metadata revision did not advance' }

    $page = Request 'GET' '/api/v1/workloads?limit=1' 200
    if ($page.body.next_cursor) {
        $cursor = [Uri]::EscapeDataString($page.body.next_cursor)
        $null = Request 'GET' ('/api/v1/workloads?limit=1&cursor=' + $cursor) 200
    }

    $spec = Get-Content -LiteralPath (Join-Path $repo 'internal/contract/examples/create-version.json') -Raw | ConvertFrom-Json
    $version = Request 'POST' ($path + '/versions') 201 $spec
    $versionId = $version.body.version_id
    $readVersion = Request 'GET' ($path + '/versions/' + $versionId) 200
    if ($readVersion.body.version_id -ne $versionId) { throw 'Version readback mismatch' }
    $versions = Request 'GET' ($path + '/versions?limit=1') 200
    if (@($versions.body.items | Where-Object version_id -eq $versionId).Count -ne 1) { throw 'Registered version missing from its collection' }
    [pscustomobject]@{ workload_id=$workloadId; version_id=$versionId; revision=$updated.body.revision; seven_operations_verified=$true }
} finally {
    Remove-Variable login,headers -ErrorAction SilentlyContinue
}
