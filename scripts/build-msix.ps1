# Builds the Microsoft Store package (.msix) around a release build of pdf-unseal.exe.
# Runs on Windows with the Windows SDK installed (makeappx.exe and makepri.exe).
# The package is left unsigned: the Store signs it after certification.
param(
    [Parameter(Mandatory)] [string] $Exe,      # release build, with the embedded manifest
    [Parameter(Mandatory)] [string] $Version,  # Major.Minor.Patch with a nonzero major, for example 1.0.0
    [Parameter(Mandatory)] [string] $Out       # output .msix path
)
$ErrorActionPreference = 'Stop'

if ($Version -notmatch '^\d+\.\d+\.\d+$') { throw "Version must be Major.Minor.Patch, got '$Version'" }
# Store rules: each part is 0-65535 and the first part cannot be 0.
$parts = $Version.Split('.') | ForEach-Object { [int]$_ }
if ($parts[0] -eq 0) { throw "The Store rejects versions with major 0, got '$Version'; release 1.0.0 or later" }
if ($parts | Where-Object { $_ -gt 65535 }) { throw "Version parts must be at most 65535, got '$Version'" }

# Use the newest x64 SDK tools.
$sdkBin = Get-ChildItem "${env:ProgramFiles(x86)}\Windows Kits\10\bin\10.*\x64" -Directory |
    Sort-Object { [version]$_.Parent.Name } |
    Select-Object -Last 1
if (-not $sdkBin) { throw 'Windows SDK not found' }
$makeappx = Join-Path $sdkBin.FullName 'makeappx.exe'
$makepri = Join-Path $sdkBin.FullName 'makepri.exe'

$src = Join-Path $PSScriptRoot '..\packaging\msix'
$stage = Join-Path ([IO.Path]::GetTempPath()) ("pdf-unseal-msix-" + [guid]::NewGuid())
New-Item -ItemType Directory -Path $stage | Out-Null
try {
    Copy-Item $Exe (Join-Path $stage 'pdf-unseal.exe')
    Copy-Item (Join-Path $src 'Assets') (Join-Path $stage 'Assets') -Recurse
    # The Store requires the fourth version part to be 0.
    (Get-Content (Join-Path $src 'AppxManifest.xml') -Raw).Replace('__VERSION__', "$Version.0") |
        Set-Content (Join-Path $stage 'AppxManifest.xml') -Encoding utf8

    # resources.pri lets Windows pick the scale- and targetsize- variants of the logos.
    # Keep the config outside the staging folder so it is not packaged or indexed.
    $priConfig = "$stage.priconfig.xml"
    & $makepri createconfig /cf $priConfig /dq en-US /pv 10.0.0 /o
    if ($LASTEXITCODE) { throw 'makepri createconfig failed' }
    # The default config splits resources per scale and language, which only bundles use;
    # a single package needs everything in resources.pri.
    $xml = [xml](Get-Content $priConfig -Raw)
    foreach ($node in @($xml.SelectNodes('//packaging'))) { [void]$node.ParentNode.RemoveChild($node) }
    $xml.Save($priConfig)
    & $makepri new /pr $stage /cf $priConfig /of (Join-Path $stage 'resources.pri') /o
    if ($LASTEXITCODE) { throw 'makepri new failed' }
    $split = Get-ChildItem $stage -Filter 'resources.*.pri'
    if ($split) { throw "makepri split resources into $($split.Name -join ', ')" }

    & $makeappx pack /d $stage /p $Out /o
    if ($LASTEXITCODE) { throw 'makeappx pack failed' }
} finally {
    Remove-Item $stage -Recurse -Force
    Remove-Item "$stage.priconfig.xml" -ErrorAction SilentlyContinue
}
