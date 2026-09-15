# Go source installation: go install github.com/ironpark/zapp/cmd/zapp@latest
# After installation: zapp init --app dist/MyApp.app; zapp config show; zapp build
# Install a published zapp release without requiring administrator access.
# Optional: $env:ZAPP_VERSION = 'v1.0.0-beta'; $env:ZAPP_INSTALL_DIR = 'C:\Tools\zapp'
& {
    $ErrorActionPreference = 'Stop'
    $ProgressPreference = 'SilentlyContinue'
    if ($env:OS -ne 'Windows_NT') { throw 'Use install.sh on macOS and Linux.' }
    # Prefer the native architecture when running under WOW64/emulation.
    $nativeArch = $env:PROCESSOR_ARCHITEW6432
    if (-not $nativeArch) { $nativeArch = $env:PROCESSOR_ARCHITECTURE }
    switch ($nativeArch) {
        'AMD64' { $arch = 'x86_64' }
        'ARM64' { $arch = 'arm64' }
        default { throw "Unsupported architecture: $nativeArch. Supported: AMD64, ARM64." }
    }
    $oldProtocol = [Net.ServicePointManager]::SecurityProtocol
    $work = $null
    $staged = $null
    try {
        [Net.ServicePointManager]::SecurityProtocol = $oldProtocol -bor [Net.SecurityProtocolType]::Tls12
        $repo = 'https://github.com/ironpark/zapp'
        $version = $env:ZAPP_VERSION
        if (-not $version) {
            $release = Invoke-RestMethod 'https://api.github.com/repos/ironpark/zapp/releases/latest'
            $version = $release.tag_name
        }
        if (-not $version -or $version -notmatch '^[A-Za-z0-9._-]+$') {
            throw 'Invalid release tag. Set ZAPP_VERSION to a tag such as v1.0.0-beta.'
        }
        $installDir = $env:ZAPP_INSTALL_DIR
        if (-not $installDir) { $installDir = Join-Path $env:LOCALAPPDATA 'zapp\bin' }
        if (-not [IO.Path]::IsPathRooted($installDir)) { throw 'ZAPP_INSTALL_DIR must be an absolute path.' }
        $archive = "zapp_Windows_$arch.zip"
        $checksums = 'zapp_' + ($version -creplace '^v', '') + '_checksums.txt'
        $work = Join-Path ([IO.Path]::GetTempPath()) ('zapp-install-' + [Guid]::NewGuid().ToString('N'))
        New-Item -ItemType Directory -Path $work | Out-Null
        $archivePath = Join-Path $work $archive
        Write-Host "Downloading zapp $version (Windows/$arch)..."
        Invoke-WebRequest -UseBasicParsing "$repo/releases/download/$version/$archive" -OutFile $archivePath
        $checksumPath = Join-Path $work 'checksums.txt'
        Invoke-WebRequest -UseBasicParsing "$repo/releases/download/$version/$checksums" -OutFile $checksumPath
        $pattern = '^([a-fA-F0-9]{64})\s+' + [Regex]::Escape($archive) + '$'
        $entries = @(Get-Content -LiteralPath $checksumPath | Where-Object { $_ -match $pattern })
        if ($entries.Count -ne 1) { throw 'Missing or invalid release checksum.' }
        $expected = [Regex]::Match($entries[0], $pattern).Groups[1].Value
        if ((Get-FileHash -LiteralPath $archivePath -Algorithm SHA256).Hash -ne $expected) {
            throw 'Checksum mismatch; installation aborted.'
        }
        Expand-Archive -LiteralPath $archivePath -DestinationPath (Join-Path $work 'extracted')
        $binary = Join-Path $work 'extracted\zapp.exe'
        if (-not (Test-Path -LiteralPath $binary -PathType Leaf)) { throw 'Archive does not contain zapp.exe.' }
        New-Item -ItemType Directory -Path $installDir -Force | Out-Null
        $staged = Join-Path $installDir ('.zapp-install-' + [Guid]::NewGuid().ToString('N'))
        Copy-Item -LiteralPath $binary -Destination $staged
        Move-Item -LiteralPath $staged -Destination (Join-Path $installDir 'zapp.exe') -Force
        $staged = $null
        $userPath = [string][Environment]::GetEnvironmentVariable('Path', 'User')
        if (($userPath -split ';') -notcontains $installDir) {
            [Environment]::SetEnvironmentVariable('Path', (($userPath.TrimEnd(';') + ';' + $installDir).TrimStart(';')), 'User')
        }
        if (($env:Path -split ';') -notcontains $installDir) { $env:Path = $installDir + ';' + $env:Path }
        Write-Host "Installed zapp $version to $installDir\zapp.exe"
        Write-Host 'Open a new terminal to use zapp in other sessions.'
    }
    finally {
        [Net.ServicePointManager]::SecurityProtocol = $oldProtocol
        if ($staged -and (Test-Path -LiteralPath $staged)) { Remove-Item -LiteralPath $staged -Force }
        if ($work -and (Test-Path -LiteralPath $work)) { Remove-Item -LiteralPath $work -Recurse -Force }
    }
}
