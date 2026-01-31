$baseUrl = "http://localhost:8080"
$testCases = @(
    @{Ticker = "AAPL"; CIK = "0000320193"; Year = 2024; Industry = "Tech" },
    @{Ticker = "INTC"; CIK = "0000050863"; Year = 2024; Industry = "Semi (Dirty)" },
    @{Ticker = "F"; CIK = "0000037996"; Year = 2023; Industry = "Auto" },
    @{Ticker = "PFE"; CIK = "0000078003"; Year = 2023; Industry = "Pharma" }
)

Write-Host "Starting FSAP Extraction Batch Tests..." -ForegroundColor Cyan

foreach ($tc in $testCases) {
    Write-Host "`n----------------------------------------"
    Write-Host "Testing $($tc.Ticker) ($($tc.Industry)) - FY$($tc.Year)" -ForegroundColor Yellow
    
    $body = @{
        ticker      = $tc.Ticker
        cik         = $tc.CIK
        form        = "10-K"
        fiscal_year = $tc.Year
        use_fee     = $false # Use LLM
    } | ConvertTo-Json

    try {
        $sw = [System.Diagnostics.Stopwatch]::StartNew()
        $response = Invoke-RestMethod -Uri "$baseUrl/api/edgar/fsap-map" -Method Post -Body $body -ContentType "application/json" -TimeoutSec 600
        $sw.Stop()
        
        if ($response.metadata) {
            $mapped = $response.metadata.variables_mapped
        }
        else {
            $mapped = 0
        }

        $status = "❌ FAIL"
        if ($mapped -gt 20) { $status = "✅ PASS" }
        
        if ($status -eq "✅ PASS") {
            Write-Host "Status: $status" -ForegroundColor Green
        }
        else {
            Write-Host "Status: $status" -ForegroundColor Red
        }
        Write-Host "Variables Mapped: $mapped"
        Write-Host "Time: $([math]::Round($sw.Elapsed.TotalSeconds, 1))s"
        
        if ($response.balance_sheet.current_assets.cash_and_cash_equivalents.value) {
            $val = $response.balance_sheet.current_assets.cash_and_cash_equivalents.value
            Write-Host "   Cash: $val" -ForegroundColor Gray
        }
        
    }
    catch {
        Write-Host "Error: $($_.Exception.Message)" -ForegroundColor Red
        if ($_.Exception.Response) {
            # Try to read body
            try {
                $reader = New-Object System.IO.StreamReader $_.Exception.Response.GetResponseStream()
                $errBody = $reader.ReadToEnd()
                Write-Host "   Server Details: $errBody" -ForegroundColor DarkRed
            }
            catch {}
        }
    }
}
