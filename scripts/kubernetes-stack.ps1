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
    # Keep a hidden console parent so the WSL client retains an attached session.
    # Windows process management separates it from the caller's lifetime.
    # The existing unprivileged helper exits when normal stop ends forwarding.
    $holdArguments = @((Join-Path $env:WINDIR 'System32/wsl.exe'), '-d', $Distro, '-u', $ServiceUser, '--', 'python3', "$linuxRepo/deploy/kubernetes/stack.py", 'hold', '--user', $ServiceUser)
    foreach ($argument in $holdArguments) {
        if ($argument -match '["\r\n\x00%!&|<>^]' -or $argument.EndsWith('\')) { throw 'Unsupported keep-alive process argument' }
    }
    # WSL parses option flags from the raw command line; quoted flags become a
    # Linux command. Quote only values containing whitespace, not every token.
    $holdCommand = ($holdArguments | ForEach-Object { if ($_ -match '\s') { '"' + $_ + '"' } else { $_ } }) -join ' '
    $consoleCommand = '"' + (Join-Path $env:WINDIR 'System32/cmd.exe') + '" /d /v:off /s /c "' + $holdCommand + '"'
    $startup = New-CimInstance -ClassName Win32_ProcessStartup -ClientOnly -Property @{ ShowWindow = [uint16]0 }
    $launched = Invoke-CimMethod -ClassName Win32_Process -MethodName Create -Arguments @{ CommandLine = $consoleCommand; ProcessStartupInformation = $startup }
    if ($launched.ReturnValue -ne 0) { throw 'Cannot start detached WSL keep-alive' }
    $ready = Invoke-RestMethod 'http://127.0.0.1:8081/readyz' -TimeoutSec 5
    $discovery = Invoke-RestMethod 'http://127.0.0.1:8082/realms/forge/.well-known/openid-configuration' -TimeoutSec 5
    if ($ready.status -ne 'ok' -or $discovery.issuer -ne 'http://127.0.0.1:8082/realms/forge') { throw 'Windows loopback validation failed' }
    Write-Output 'Windows loopback connectivity verified.'
}
