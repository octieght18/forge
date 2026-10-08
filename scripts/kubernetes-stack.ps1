param(
    [ValidateSet('import-native', 'up', 'stop', 'status', 'login')][string]$Action = 'up',
    [string]$Distro = 'Ubuntu',
    [string]$ServiceUser = 'owner'
)
$ErrorActionPreference = 'Stop'
$repoPath = Split-Path -Parent $PSScriptRoot
$linuxRepo = (& wsl.exe -d $Distro -- wslpath -a -u $repoPath.Replace('\', '/')).Trim()
if ($LASTEXITCODE -ne 0) { throw 'Cannot resolve repository in WSL' }
if ($Action -eq 'login') {
    & wsl.exe -d $Distro -u $ServiceUser -- bash -c 'exec "$HOME/.local/share/forge-kubernetes/bin/forge-login" --token-file "$HOME/.local/share/forge-kubernetes/tokens/access.json"'
} else {
    & wsl.exe -d $Distro -u root -- python3 "$linuxRepo/deploy/kubernetes/stack.py" $Action --user $ServiceUser
}
if ($LASTEXITCODE -ne 0) { throw 'Kubernetes Forge operation failed' }
if ($Action -in @('up', 'import-native')) {
    Start-Process -FilePath wsl.exe -ArgumentList @('-d', $Distro, '-u', $ServiceUser, '--', 'python3', "$linuxRepo/deploy/kubernetes/stack.py", 'hold', '--user', $ServiceUser) -WindowStyle Hidden | Out-Null
    $ready = Invoke-RestMethod 'http://127.0.0.1:8081/readyz' -TimeoutSec 5
    $discovery = Invoke-RestMethod 'http://127.0.0.1:8082/realms/forge/.well-known/openid-configuration' -TimeoutSec 5
    if ($ready.status -ne 'ok' -or $discovery.issuer -ne 'http://127.0.0.1:8082/realms/forge') { throw 'Windows loopback validation failed' }
    Write-Output 'Windows loopback connectivity verified.'
}
