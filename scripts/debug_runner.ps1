
$baseUrl = "http://localhost:8080"
$ticker = "AAPL"
$year = 2024
$body = @{
    ticker      = $ticker
    cik         = "0000320193"
    form        = "10-K"
    fiscal_year = $year
    use_fee     = $false
} | ConvertTo-Json

Write-Host "Requesting $ticker FY$year..."
$response = Invoke-RestMethod -Uri "$baseUrl/api/edgar/fsap-map" -Method Post -Body $body -ContentType "application/json" -TimeoutSec 600

# Save output even if invalid
$response | ConvertTo-Json -Depth 20 | Out-File "debug_aapl_2024.json" -Encoding utf8
Write-Host "Saved to debug_aapl_2024.json"
