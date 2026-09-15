$ErrorActionPreference = "Stop"

$repositoryRoot = git rev-parse --show-toplevel
if (-not $repositoryRoot) {
    throw "Run this script from inside the Plex Song Grabber repository."
}

git -C $repositoryRoot config --local core.hooksPath .githooks
Write-Host "Automatic commit messages enabled for this checkout."
