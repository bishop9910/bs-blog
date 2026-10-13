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

On Windows the executable also gets an icon and a version resource. Both come out
of winres/: versioninfo.json and icon.ico are ordinary tracked files you can edit
or replace, and goversioninfo compiles them into .syso objects that Go links into
the binary. Nothing in here redraws icon.ico -- tools/icongen only fills in a
missing one, so artwork you drop there survives a build. goversioninfo is an
external tool, so a machine without it still gets a working executable -- just
without the icon and the version tab.

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

Write-Output '=== windows resources ==='
# goversioninfo compiles winres/versioninfo.json and winres/icon.ico into a .syso
# -- a COFF object Go links straight into the executable. The .syso is rebuilt on
# every build because the version Explorer shows in the Properties dialog has to
# track the one baked in below by -ldflags; the files it is built from are not.
# icongen runs only to fill in a missing icon.ico and leaves an existing one
# alone, so replacing that file with your own artwork is safe.
$targetOS = if ($env:GOOS) { $env:GOOS } else { (& go env GOOS).Trim() }
$targetArch = if ($env:GOARCH) { $env:GOARCH } else { (& go env GOARCH).Trim() }
if ($targetOS -ne 'windows') {
    Write-Output "skipped: the target is $targetOS/$targetArch and .syso resources are Windows-only"
} elseif (-not (Get-Command goversioninfo -ErrorAction SilentlyContinue)) {
    Write-Output 'NOTE: goversioninfo is not on PATH, so the executable gets no icon and'
    Write-Output '      no version resource. Install it and rebuild:'
    Write-Output '        go install github.com/josephspurrier/goversioninfo/cmd/goversioninfo@latest'
} else {
    Push-Location $be
    try {
        # icongen is a fallback for a missing icon and nothing more. icon.ico is
        # a tracked asset, so an existing one is used as-is and the tool is not
        # even invoked -- which is what keeps a build from ever redrawing the
        # artwork you put there.
        if (Test-Path (Join-Path $be 'winres\icon.ico')) {
            Write-Output 'icon: reusing winres/icon.ico'
        } else {
            & go run ./tools/icongen
            if ($LASTEXITCODE -ne 0) { Write-Output "FAIL: icongen exit=$LASTEXITCODE"; exit 1 }
        }

        # Windows stores a version as four 16-bit numbers, so the git-derived
        # string only maps onto them when it opens with dotted numbers. A bare
        # commit hash or "dev" leaves the placeholders in winres/versioninfo.json
        # alone and overrides just the text fields.
        $verArgs = @('-file-version', $Version, '-product-version', $Version)
        if ($Version -match '^v?(\d+)\.(\d+)\.(\d+)(?:\.(\d+))?') {
            $nums = @([int]$Matches[1], [int]$Matches[2], [int]$Matches[3])
            $nums += if ($Matches[4]) { [int]$Matches[4] } else { 0 }
            $verArgs += @('-ver-major', $nums[0], '-ver-minor', $nums[1], '-ver-patch', $nums[2], '-ver-build', $nums[3])
            $verArgs += @('-product-ver-major', $nums[0], '-product-ver-minor', $nums[1], '-product-ver-patch', $nums[2], '-product-ver-build', $nums[3])
        }

        # goversioninfo resolves the JSON, its IconPath and its output against
        # the working directory, so run it from winres/ and move what it writes
        # into the package directory -- Go only links .syso from the directory it
        # is building.
        Push-Location (Join-Path $be 'winres')
        try {
            & goversioninfo @verArgs -platform-specific versioninfo.json
            $code = $LASTEXITCODE
        } finally {
            Pop-Location
        }
        if ($code -ne 0) { Write-Output "FAIL: goversioninfo exit=$code"; exit 1 }

        Move-Item (Join-Path $be 'winres\resource_windows_*.syso') (Join-Path $be 'cmd\server\') -Force
        Write-Output "wrote 4 resource objects for $targetArch (version $Version)"
    } finally {
        Pop-Location
    }
}

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
