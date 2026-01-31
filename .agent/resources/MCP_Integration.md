# MCP Integration: Standardizing Agentic Tools

The Agentic Valuation Platform leverages the **Model Context Protocol (MCP)** to provide a standardized interface between LLM agents and the system's core capabilities (database, calculation engine, web search).

## 1. Why MCP?
- **Interoperability**: Allows different LLMs (DeepSeek, OpenAI, Gemini) to use the same set of tools without model-specific glue code.
- **Security**: Granular control over what data (Resources) and actions (Tools) each agent can access.
- **Extensibility**: Easily plug in new financial data sources or third-party valuation APIs as MCP servers.

## 2. Platform MCP Servers

### 📊 FSAP Data Server (Resource & Tool)
- **Role**: Provides access to the PostgreSQL `fsap_data` and `project_snapshots`.
- **Tools**:
    - `get_period_data(ticker, year)`: Fetch standardized financial statements.
    - `get_segment_data(ticker, year)`: Fetch granular revenue and margin data by business unit (e.g., Ford Blue vs Ford Model e).
    - `log_user_correction`: Record human-in-the-loop adjustments (Context for the Learner agent).

### 🧮 Calc Engine Server (Tool)
- **Role**: Wraps the Go-based calculation engine.
- **Tools**:
    - `run_common_size_analysis(data)`: Converts figures to % of revenue.
    - `calculate_cagr(start_value, end_value, periods)`: Standardized growth calculations.
    - `validate_integrity(bs_data)`: Checks if Assets = L + E.

### 🔍 Market Research Server (Tool)
- **Role**: Wraps the Web Search and News APIs.
- **Tools**:
    - `search_industry_trends(industry)`: Fetch latest policy/tech news.
    - `get_commodity_prices(commodity)`: Fetch historical and spot prices (Steel, Lithium, etc.).

## 3. Agent-Tool Mapping (MCP)

| Agent | MCP Server Access | Key Tools Used |
| :--- | :--- | :--- |
| **Accountant** | FSAP Data, Calc Engine | `get_period_data`, `validate_integrity` |
| **Strategist** | FSAP Data, Calc Engine | `get_segment_data`, `run_common_size_analysis` |
| **Researcher** | Market Research | `search_industry_trends`, `get_commodity_prices` |
| **Analyst** | FSAP Data, Calc Engine | `get_period_data`, `calculate_cagr` |

## 4. Implementation (Go)
The Go backend (`cmd/api`) will implement the **MCP Host** role, managing agent sessions and routing tool calls. Future specialized "skills" can be deployed as standalone MCP servers running on local or remote containers.
