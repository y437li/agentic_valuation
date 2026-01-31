# Configuration
$baseUrl = "http://localhost:8080"
$outputDir = "batch_data"
$logFile = "batch_job.log"
$years = 2024..2020 

# Top 50 Companies (Diverse Sectors)
$targets = @(
    # --- Tech & Semi ---
    @{Ticker="AAPL"; CIK="0000320193"; Sector="Tech"}, @{Ticker="MSFT"; CIK="0000789019"; Sector="Tech"},
    @{Ticker="NVDA"; CIK="0001045810"; Sector="Semi"}, @{Ticker="GOOGL";CIK="0001652044"; Sector="Tech"},
    @{Ticker="AMZN"; CIK="0001018724"; Sector="Cons"}, @{Ticker="META"; CIK="0001326801"; Sector="Tech"},
    @{Ticker="AVGO"; CIK="0001730168"; Sector="Semi"}, @{Ticker="TSM";  CIK="0001046179"; Sector="Semi"},
    @{Ticker="ORCL"; CIK="0001341439"; Sector="Tech"}, @{Ticker="ADBE"; CIK="0000796343"; Sector="Tech"},
    @{Ticker="AMD";  CIK="0000002488"; Sector="Semi"}, @{Ticker="INTC"; CIK="0000050863"; Sector="Semi"},
    @{Ticker="CRM";  CIK="0001108524"; Sector="Tech"}, @{Ticker="CSCO"; CIK="0000858877"; Sector="Tech"},
    @{Ticker="IBM";  CIK="0000051143"; Sector="Tech"}, @{Ticker="QCOM"; CIK="0000804328"; Sector="Semi"},

    # --- Consumer ---
    @{Ticker="TSLA"; CIK="0001318605"; Sector="Auto"}, @{Ticker="WMT";  CIK="0000104169"; Sector="Retail"},
    @{Ticker="PG";   CIK="0000080424"; Sector="Cons"}, @{Ticker="COST"; CIK="0000909832"; Sector="Retail"},
    @{Ticker="HD";   CIK="0000354950"; Sector="Retail"},@{Ticker="KO";   CIK="0000021344"; Sector="Bev"},
    @{Ticker="PEP";  CIK="0000077476"; Sector="Bev"},  @{Ticker="MCD";  CIK="0000063908"; Sector="Rest"},
    @{Ticker="NKE";  CIK="0000320187"; Sector="Apparel"},@{Ticker="SBUX"; CIK="0000829224"; Sector="Rest"},
    @{Ticker="F";    CIK="0000037996"; Sector="Auto"}, @{Ticker="GM";   CIK="0001467858"; Sector="Auto"},

    # --- Finance ---
    @{Ticker="JPM";  CIK="0000019617"; Sector="Fin"},  @{Ticker="V";    CIK="0001403161"; Sector="Fin"},
    @{Ticker="MA";   CIK="0001141391"; Sector="Fin"},  @{Ticker="BAC";  CIK="0000070858"; Sector="Fin"},
    @{Ticker="WFC";  CIK="0000072971"; Sector="Fin"},  @{Ticker="GS";   CIK="0000886982"; Sector="Fin"},
    @{Ticker="MS";   CIK="0000895421"; Sector="Fin"},  @{Ticker="BLK";  CIK="0001364742"; Sector="Fin"},

    # --- Healthcare ---
    @{Ticker="LLY";  CIK="0000059478"; Sector="Pharma"},@{Ticker="UNH";  CIK="0000731766"; Sector="Health"},
    @{Ticker="JNJ";  CIK="0000200406"; Sector="Pharma"},@{Ticker="MRK";  CIK="0000310158"; Sector="Pharma"},
    @{Ticker="ABBV"; CIK="0001551152"; Sector="Pharma"},@{Ticker="PFE";  CIK="0000078003"; Sector="Pharma"},
    @{Ticker="TMO";  CIK="0000096943"; Sector="Health"},@{Ticker="AMGN"; CIK="0000318154"; Sector="Biotech"},

    # --- Other (Energy, Ind, Comm) ---
    @{Ticker="XOM";  CIK="0000034088"; Sector="Energy"},@{Ticker="CVX";  CIK="0000093410"; Sector="Energy"},
    @{Ticker="GE";   CIK="0000040545"; Sector="Ind"},   @{Ticker="CAT";  CIK="0000018230"; Sector="Ind"},
    @{Ticker="BA";   CIK="0000012927"; Sector="Aero"},  @{Ticker="HON";  CIK="0000773840"; Sector="Ind"},
    @{Ticker="NFLX"; CIK="0001065280"; Sector="Comm"},  @{Ticker="DIS";  CIK="0001744489"; Sector="Comm"},
    @{Ticker="VZ";   CIK="0000732717"; Sector="Telecom"},@{Ticker="T";    CIK="0000007327"; Sector="Telecom"}
)


if (!(Test-Path $outputDir)) { New-Item -ItemType Directory -Path $outputDir | Out-Null }
$global:startTime = Get-Date

function Log-Message($msg) {
    $timestamp = (Get-Date).ToString("yyyy-MM-dd HH:mm:ss")
    $logLine = "[$timestamp] $msg"
    Write-Host $logLine
    $logLine | Out-File $logFile -Append -Encoding utf8
}

function Validate-Financials($json) {
    if (-not $json) { return $false }
    $pass = $true
    
    # --- 1. Balance Sheet ---
    $bs = $json.balance_sheet
    if (-not $bs) { return $false }

    $ca = 0; if ($bs.current_assets -and $bs.current_assets.calculated_total) { $ca = $bs.current_assets.calculated_total }
    $nca = 0; if ($bs.noncurrent_assets -and $bs.noncurrent_assets.calculated_total) { $nca = $bs.noncurrent_assets.calculated_total }
    $A = $ca + $nca
    
    $cl = 0; if ($bs.current_liabilities -and $bs.current_liabilities.calculated_total) { $cl = $bs.current_liabilities.calculated_total }
    $ncl = 0; if ($bs.noncurrent_liabilities -and $bs.noncurrent_liabilities.calculated_total) { $ncl = $bs.noncurrent_liabilities.calculated_total }
    $L = $cl + $ncl
    
    $E = 0; if ($bs.equity -and $bs.equity.calculated_total) { $E = $bs.equity.calculated_total }
    
    $bsGap = $A - ($L + $E)
    $bsGapPct = 0; if ($A -ne 0) { $bsGapPct = ($bsGap / $A) * 100 }
    
    $gapDisplay = [math]::Round($bsGapPct, 2)

    if ([math]::Abs($bsGapPct) -gt 5) {
        Log-Message "      [FAIL] BS UNBALANCED: Gap=$gapDisplay% (A=$A, L+E=$($L+$E))"
        $pass = $false
    } else {
        Log-Message "      [PASS] BS Balanced (Gap: $gapDisplay%)"
    }

    # --- 2. Cash Flow Logic (Simplified) ---
    $cf = $json.cash_flow_statement
    if ($cf) {
        # Optional advanced checks here
    }
    
    return $pass
}

Log-Message "Starting S&P50 batch extraction (SMART SAVER MODE)..."

foreach ($target in $targets) {
    $ticker = $target.Ticker
    $companyDir = Join-Path $outputDir $ticker
    if (!(Test-Path $companyDir)) { New-Item -ItemType Directory -Path $companyDir | Out-Null }

    foreach ($year in $years) {
        $outputFile = Join-Path $companyDir "$($ticker)_FY$($year).json"
        
        # Check if exists AND VALID
        if (Test-Path $outputFile) {
            try {
                $content = Get-Content $outputFile -Raw | ConvertFrom-Json
                if (Validate-Financials $content) {
                    Log-Message "Skipping $ticker FY$year (Already Valid)"
                    continue
                } else {
                    Log-Message "Existing file $ticker FY$year failed validation. Re-extracting..."
                }
            } catch {
                Log-Message "Corrupt existing file. Re-extracting..."
            }
        }

        # STRATEGY 1: Try with Full LLM (use_fee=$false)
        # Smart Agent Mode - Most Intelligent
        
        $strategies = @(
            @{Name="LLM_AGENT"; UseFee=$false},
            @{Name="FEE_DETERM"; UseFee=$true}
        )
        
        $success = $false
        
        foreach ($strat in $strategies) {
            if ($success) { break }
            
            Log-Message "Processing $ticker FY$year using Strategy: $($strat.Name)..."
            
            $body = @{
                ticker = $ticker
                cik = $target.CIK
                form = "10-K"
                fiscal_year = $year
                use_fee = $strat.UseFee
            } | ConvertTo-Json
    
            try {
                $sw = [System.Diagnostics.Stopwatch]::StartNew()
                $response = Invoke-RestMethod -Uri "$baseUrl/api/edgar/fsap-map" -Method Post -Body $body -ContentType "application/json" -TimeoutSec 600
                $sw.Stop()

                $mapped = 0
                if ($response.metadata) { $mapped = $response.metadata.variables_mapped }
                
                # Check results - NO RETRIES to save tokens
                if (Validate-Financials $response) {
                    $timeStr = [math]::Round($sw.Elapsed.TotalSeconds, 1)
                    $response | ConvertTo-Json -Depth 10 | Out-File $outputFile -Encoding utf8
                    Log-Message "   [SUCCESS] $ticker FY$year via $($strat.Name) - $mapped mapped - ${timeStr}s"
                    $success = $true
                } else {
                    Log-Message "   [FAIL] Strategy $($strat.Name) invalid. Saving debug info and switching..."
                    # Save INVALID file for "Find out why" analysis
                    $debugFile = Join-Path $companyDir "$($ticker)_FY$($year)_FAILED_$($strat.Name).json"
                    $response | ConvertTo-Json -Depth 10 | Out-File $debugFile -Encoding utf8
                }

            } catch {
                $errMsg = $_.Exception.Message
                Log-Message "   [ERROR] Strategy $($strat.Name) Exception: $errMsg"
                Start-Sleep -Seconds 2
            }
        }
        
        if (-not $success) {
             Log-Message "   [STOP] All strategies failed for $ticker FY$year. Moving on."
        }
        
        # Rate limit
        Start-Sleep -Seconds 2
    }
}
