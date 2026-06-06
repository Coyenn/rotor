# Regenerates the differential-test goldens by running the REAL rbxtsc 3.0.0
# over testdata/diff/project. Requires Node/npm. Run from repo root:
#   powershell -File tools/oracle/oracle.ps1
$ErrorActionPreference = "Stop"
$root = git rev-parse --show-toplevel
$proj = Join-Path $root "testdata/diff/project"
$golden = Join-Path $root "testdata/diff/golden"

if (-not (Test-Path (Join-Path $proj "node_modules"))) {
	Push-Location $proj
	npm install --no-audit --no-fund
	Pop-Location
}

# clean output so removed fixtures don't leave stale goldens
Remove-Item -Recurse -Force (Join-Path $proj "out") -ErrorAction SilentlyContinue
Push-Location $proj
npx rbxtsc --type model
if ($LASTEXITCODE -ne 0) { Pop-Location; throw "rbxtsc failed with exit code $LASTEXITCODE" }
Pop-Location

Remove-Item -Recurse -Force $golden -ErrorAction SilentlyContinue
New-Item -ItemType Directory -Force $golden | Out-Null
Get-ChildItem (Join-Path $proj "out") -Filter *.luau | Copy-Item -Destination $golden
Write-Host "goldens regenerated: $((Get-ChildItem $golden).Count) files"
