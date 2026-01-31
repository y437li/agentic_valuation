# User Guide for TIED Valuation Platform

Welcome to the TIED (Transparent, Integrated, Evidence-Driven) Valuation Platform. This guide will help you set up the environment, run the demo valuation engine, and interpret the financial analysis reports.

## Prerequisites

Before running the platform, ensure you have the following installed:

1.  **Go**: Version 1.25.5 or higher.
    -   Download from [go.dev](https://go.dev/dl/).
    -   Verify installation: `go version`

2.  **DeepSeek API Key**: The platform uses DeepSeek LLM for data extraction and analysis.
    -   Obtain an API key from the DeepSeek platform.

## Installation

1.  **Clone the Repository**:
    ```bash
    git clone https://github.com/your-org/tied-valuation-platform.git
    cd tied-valuation-platform
    ```

2.  **Install Dependencies**:
    ```bash
    go mod tidy
    ```

## Configuration

Create a `.env` file in the root directory of the project to store your sensitive configuration.

1.  Create the file:
    ```bash
    touch .env
    ```

2.  Add your DeepSeek API key:
    ```env
    DEEPSEEK_API_KEY=your_actual_api_key_here
    ```

> **Note**: Do not commit your `.env` file to version control. It is already included in `.gitignore`.

## Running the Demo

The current version of the platform includes a demonstration mode that runs on cached financial data (Apple Inc. FY2024 10-K). This allows you to test the pipeline without incurring API costs for downloading huge files, though the extraction and analysis steps still use the LLM.

To run the demo:

```bash
go run cmd/pipeline/main.go
```

### What Happens When You Run It?

1.  **Data Loading**: The engine loads a cached Markdown version of Apple's 10-K filing from `pkg/core/edgar/testdata/cache/apple_10k_fy2024.md`.
2.  **Parallel Extraction**: The system uses multiple AI agents to extract key financial data in parallel.
3.  **Segment Analysis**: A specialized agent analyzes the "Segment Information" note to breakdown revenue and operating income by geography.
4.  **Financial Analysis**: The engine calculates key ratios and performs forensic checks.
5.  **Report Generation**: A structured report is printed to your console.

## Understanding the Output

The output is divided into several sections:

### [1] Core Financials
Displays the fundamental numbers extracted from the Income Statement.
-   **Revenue**: Total sales for the fiscal year.
-   **Operating Margin**: Operating Income divided by Revenue.
-   **Net Income**: The bottom line profit.

### [2] Penman Decomposition
Advanced financial ratios based on Stephen Penman's framework.
-   **RNOA (Return on Net Operating Assets)**: Measures the efficiency of the core business operations, independent of financing.
-   **ROCE (Return on Common Equity)**: The total return to shareholders.

### [3] Segment Analysis
Breakdown of performance by business unit or geography.
-   **Revenue & OpIncome**: Shows contribution of each segment.
-   Useful for understanding where the money actually comes from.

### [4] Data Validation & Aggregation Check
A self-check mechanism to ensure data integrity.
-   **Segment Sum vs. Consolidated**: Compares the sum of all segments against the reported total in the Income Statement.
-   **Combined Overhead**: The difference often represents corporate overhead not allocated to specific segments.

### [5] Forensic Risk Screening
Automated checks for potential accounting irregularities.
-   **Benford's Law**: Checks if the distribution of first digits in the financial numbers follows the natural law. A high "MAD" (Mean Absolute Deviation) might indicate manipulation.
-   **Beneish M-Score**: A statistical model to detect earnings manipulation. A score less than -1.78 usually suggests "Safe".

## Troubleshooting

### "DEEPSEEK_API_KEY is not set"
**Cause**: The `.env` file is missing or the variable is not named correctly.
**Fix**: Ensure `.env` exists in the root and contains `DEEPSEEK_API_KEY=...`.

### "Cache file not found"
**Cause**: You might be running the command from the wrong directory.
**Fix**: Ensure you run the command from the project root (`./`), not from inside `cmd/pipeline/`.

### "Extraction failed"
**Cause**: Network issues or invalid API key.
**Fix**: Check your internet connection and verify your API key limits.
