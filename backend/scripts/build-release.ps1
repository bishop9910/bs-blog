<#
.SYNOPSIS
Builds everything into backend/release/: a single bs-blog.exe with the frontend
embedded in it, alongside configs/ and data/.

.DESCRIPTION
The frontend cannot simply be referenced: go:embed is not allowed to reach
outside the Go module, and the Vite output lives at the repository root. So this
script runs the Vite build, stages dist/ into internal/web/dist (the directory
the embed directive names), and only then invokes go build.

Pass -SkipFrontend to reuse the dist/ already on disk, which is all you need
after changing Go code alone.

The release folder is assembled in place and data/ is never cleared, so an
existing database there survives a rebuild.

.EXAMPLE
.\scripts\build-release.ps1
.EXAMPLE
.\scripts\build-release.ps1 -Version 1.0.0
#>
param(
    [string]$Version = '',
    [switch]$SkipFrontend
)

$ErrorActionPreference = 'Stop'

$be = Split-Path -Parent $PSScriptRoot
$root = Split-Path -Parent $be
$dist = Join-Path $root 'dist'
$embedDir = Join-Path $be 'internal\web\dist'
$release = Join-Path $be 'release'

if (-not $Version) {
    $Version = (& git -C $root describe --tags --always --dirty 2>$null | Out-String).Trim()
    if (-not $Version -or $Version -match '^(fatal|usage)') { $Version = 'dev' }
}
Write-Output "version: $Version"

Write-Output '=== frontend build ==='
if ($SkipFrontend) {
    Write-Output 'skipped (-SkipFrontend)'
} else {
    # pnpm echoes its script banner to stderr, and under an ErrorActionPreference
    # of Stop a native command that writes to stderr aborts the script. Relax it
    # for this one call, funnel the banner back through the success stream as
    # plain text (otherwise a perfectly good build prints alarming red lines),
    # and check the exit code by hand.
    $previous = $ErrorActionPreference
    $ErrorActionPreference = 'Continue'
    Push-Location $root
    try {
        & pnpm build 2>&1 | ForEach-Object { "$_" }
        $code = $LASTEXITCODE
    } finally {
        Pop-Location
        $ErrorActionPreference = $previous
    }
    if ($code -ne 0) { Write-Output "FAIL: pnpm build exit=$code"; exit 1 }
}
if (-not (Test-Path (Join-Path $dist 'index.html'))) {
    Write-Output "FAIL: $dist\index.html is missing -- run once without -SkipFrontend"
    exit 1
}

Write-Output '=== staging the frontend for embedding ==='
New-Item -ItemType Directory -Force -Path $embedDir | Out-Null
# Clear the previous build but keep .gitkeep: it is the file that makes the
# embed pattern match on a fresh clone, so deleting it would break the build.
Get-ChildItem -Path $embedDir -Force |
    Where-Object { $_.Name -ne '.gitkeep' } |
    Remove-Item -Recurse -Force
& robocopy $dist $embedDir /E /NFL /NDL /NJH /NJS /NP | Out-Null
# robocopy reports success as 0..7 (1 simply means "files were copied").
if ($LASTEXITCODE -ge 8) { Write-Output "FAIL: robocopy exit=$LASTEXITCODE"; exit 1 }
$staged = Get-ChildItem -Path $embedDir -Recurse -File |
    Where-Object { $_.Name -ne '.gitkeep' } |
    Measure-Object -Property Length -Sum
Write-Output ("staged {0} files, {1:N1} KB" -f $staged.Count, (($staged.Sum / 1KB)))

Write-Output '=== go build ==='
New-Item -ItemType Directory -Force -Path $release | Out-Null
$exe = Join-Path $release 'bs-blog.exe'
# Pure Go throughout (glebarez/sqlite): CGO off keeps the output a single
# static executable with no C runtime to ship alongside it.
$env:CGO_ENABLED = '0'
Push-Location $be
& go build -trimpath -ldflags "-s -w -X main.version=$Version" -o $exe ./cmd/server
$code = $LASTEXITCODE
Pop-Location
if ($code -ne 0) { Write-Output "FAIL: go build exit=$code"; exit 1 }

Write-Output '=== assembling the release folder ==='
New-Item -ItemType Directory -Force -Path (Join-Path $release 'configs') | Out-Null
New-Item -ItemType Directory -Force -Path (Join-Path $release 'data\uploads') | Out-Null
Copy-Item (Join-Path $be 'configs\config.yaml') (Join-Path $release 'configs\config.yaml') -Force

$cfg = Get-Content (Join-Path $release 'configs\config.yaml') -Raw
if ($cfg -notmatch '(?m)^\s*staticDir:\s*""\s*$') {
    Write-Output 'NOTE: the released config.yaml sets a non-empty staticDir, so the'
    Write-Output '      embedded frontend is used only while that path is missing.'
}

# The database is deliberately not copied: overwriting a live one would be much
# worse than starting empty, so the choice stays with the operator.
$liveDB = Join-Path $be 'data\blog.db'
if ((Test-Path $liveDB) -and -not (Test-Path (Join-Path $release 'data\blog.db'))) {
    Write-Output ''
    Write-Output 'NOTE: backend\data holds a database but release\data does not.'
    Write-Output '      Copy backend\data\* into release\data\ to keep those posts.'
}

Write-Output ''
Write-Output '=== release ==='
$exeInfo = Get-Item $exe
Write-Output ("{0,-30} {1,10:N0} KB" -f $exeInfo.Name, ($exeInfo.Length / 1KB))
Get-ChildItem $release -Directory | Sort-Object Name | ForEach-Object {
    Write-Output ("{0,-30} {1,10}" -f ($_.Name + '\'), '<dir>')
}
Write-Output ''
Write-Output "Deploy by copying $release anywhere: configs/ and data/ are resolved"
Write-Output 'against the working directory, so start the exe from the folder itself.'
