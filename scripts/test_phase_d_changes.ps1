param(
    [string]$RepoRoot = "D:\Vit_DAW"
)

$ErrorActionPreference = "Stop"

Write-Host "=== Testing Phase D Dimension Mapping Changes ===" -ForegroundColor Cyan

# Test 1: Build agent to verify code compiles
Write-Host "`n[1/3] Testing agent build..." -ForegroundColor Yellow
Push-Location "$RepoRoot\agent"
try {
    $buildOutput = go build -o test_agent.exe ./cmd/vitagent 2>&1
    if ($LASTEXITCODE -ne 0) {
        Write-Host "Build failed:" -ForegroundColor Red
        Write-Host $buildOutput
        exit 1
    }
    Write-Host "✓ Agent builds successfully" -ForegroundColor Green
    Remove-Item test_agent.exe -ErrorAction SilentlyContinue
} finally {
    Pop-Location
}

# Test 2: Run unit tests for dimension mapping
Write-Host "`n[2/3] Running dimension mapping tests..." -ForegroundColor Yellow
Push-Location "$RepoRoot\agent"
try {
    $testOutput = go test ./internal/capabilitycontext -run "TestCCBViewCatalogIncludesDimensionMapping|TestGetViewsForDimension" -v 2>&1
    Write-Host $testOutput
    if ($LASTEXITCODE -ne 0) {
        Write-Host "✗ Dimension mapping tests failed" -ForegroundColor Red
        exit 1
    }
    Write-Host "✓ Dimension mapping tests passed" -ForegroundColor Green
} finally {
    Pop-Location
}

# Test 3: Run existing CCB catalog tests to ensure no regression
Write-Host "`n[3/3] Running existing CCB tests for regression check..." -ForegroundColor Yellow
Push-Location "$RepoRoot\agent"
try {
    $testOutput = go test ./internal/capabilitycontext -run "TestFreeStateObservationCatalog" -v 2>&1
    Write-Host $testOutput
    if ($LASTEXITCODE -ne 0) {
        Write-Host "✗ Regression detected in CCB tests" -ForegroundColor Red
        exit 1
    }
    Write-Host "✓ No regression in existing tests" -ForegroundColor Green
} finally {
    Pop-Location
}

Write-Host "`n=== All Phase D validation tests passed ===" -ForegroundColor Green
Write-Host "Ready to run full D1 smoke test with: .\scripts\run_free_state_d1_smoke.ps1 -PublicCaseId spv1_p02" -ForegroundColor Cyan
