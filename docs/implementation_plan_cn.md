
# 财务报表分析引擎扩展 (FSA)

## 目标描述
将当前的“提取与预测”系统转型为全面的**财务报表分析 (FSA)** 平台。这包括实施高级比率分析、通用共同比逻辑（Common-size）、基于 Penman/Lundholm & Sloan 框架的分部数据处理，以及用于可视化的 **Web UI**。

## 需要用户审核
> [!IMPORTANT]
> 这是一个重大的架构扩展，涉及多个新的 Agent 和通用的反射逻辑。

## 提议的变更

### 第 0 阶段：持久化与数据完整性 (关键 / CRITICAL)
#### [修改] `pkg/core/edgar/parser.go`
-   **支持修订版 (10-KA)**:
    -   更新 `GetFilingMetadataByYear` 以同时搜索 `10-K` and `10-KA`。
    -   解析 `filingDate` 以确定目标财政年度的绝对最新提交。
    -   确保最新的修订版（Amendment）覆盖原始版本。

#### [修改] `pkg/api/valuation/handler.go` & `pkg/core/store/fsap_cache.go`
-   **"混合金库" 持久化策略 (一次性提取 / Extract-Once)**:
    -   **摄取 (Ingest)**: 下载 HTML -> 转换为 Markdown "切片" -> 保存到文件缓存 (例如 `.cache/edgar/markdown/{cik}_{acc}.md`)。
    -   **提取 (Extract)**: 运行多智能体系统（财务 + 分部）**仅一次**。
    -   **持久化 (Persist)**: 将**完整的结构化 JSON** (包括分部表) 存储到 Postgres 逻辑 (`fsap_store`)。
    -   **加载 (Load)**: 前端从数据库获取 JSON (<100ms)。
    -   **审计 (Audit)**: "跳转至原文 (Jump to Source)" 使用文件缓存中的 `FullMarkdown` 路径/内容来展示上下文。

#### [新增] 校验与信任 ("拉链校验 / The Zipper")
-   **数学完整性**: 在后端直接运行 `资产 = 负债 + 权益` (Assets = L + E) 检查。
-   **溯源性**: 确保 Postgres 中的每个数字都有对应的 `LineNumber` 指针，指向缓存的 Markdown 文件。

### 第 1 阶段：基础 (通用共同比与增长)
#### [修改] `pkg/core/calc/common_size.go`
-   **通用垂直分析**: 使用反射遍历 `IncomeStatement` 和 `BalanceSheet` 中的 *所有* 字段。
    -   计算每个利润表 (IS) 项目的 `占收入百分比 (% of Revenue)`。
    -   计算每个资产负债表 (BS) 项目的 `占总资产百分比 (% of Total Assets)`。
-   **水平分析**:
    -   实现 `CalculateGrowth(current, previous)` 计算每个项目的同比 (YoY) 增长。
    -   实现 `CalculateCAGR(history []Financials)` 计算 3 年和 5 年的复合年均增长率 (CAGR)。

### 第 2 阶段：高级比率 (风险与盈利能力)
#### [修改] `pkg/core/calc/analysis.go`
-   **盈利能力分解 (DuPont / Penman)**:
    -   **NOA (净经营资产)**: 分离 经营性资产 vs 金融性资产。
    -   **RNOA (净经营资产回报率)**: `NOPAT / 平均 NOA`。
    -   **NBC (净借款成本)**: `税后利息 / 平均净债务`。
    -   **ROCE 分解**: `RNOA + (FLEV * Spread)`。
-   **流动性与偿债能力**:
    -   流动比率 (Current Ratio), 速动比率 (Quick Ratio), 经营现金流比率。
    -   债务/资本比 (Debt/Capital), 利息保障倍数 (EBIT / Interest)。
-   **风险模型**:
    -   **Altman Z-Score**: `1.2A + 1.4B + 3.3C + 0.6D + 1.0E` (区分 制造/非制造 变体)。
    -   **Beneish M-Score**: 8-variable model for earnings manipulation detection。

### 第 3 阶段：分部分析 (Level 3)
#### [新增] `pkg/core/analysis/segment_agent.go`
-   **附注分析 Agent**:
    -   针对 "分部信息 (Segment Information)" 附注 (通常是 Note 25)。
    -   按 分部/地区 提取 `收入`, `营业利润`, `资产`, `折旧`。
-   **集成**:
    -   将分部数据输入 `ProjectionEngine` 用于 "分部加总 (Sum-of-Parts)" 预测。

### 第 4 阶段：整合
-   **统一报告**: 重构集成测试以生成统一的 JSON 报告。
-   **CLI**: 创建 `cmd/pipeline` 以便于执行。

### 第 5 阶段：Web UI 可视化与集成
#### [修改] `web-ui/src/app/valuation-report/page.tsx`
-   **可视化改进**:
    -   **共同比分析**: 添加 `占收入%` (IS) 和 `占资产%` (BS) 列。
    -   **趋势分析**: 显示带颜色编码 (绿/红) 的同比/环比增长。
    -   **营运资本**: DSO, DSI, DPO, CCC 的面板与可视化。
    -   **利润率仪表板**: 毛利, 营业利润, 净利, EBITDA 利润率的专用卡片。
-   **后端集成**:
    -   确保 `/api/valuation/report` 返回所有必要的衍生指标。

## 验证计划 (Verification Plan)
### 自动化测试
-   `TestCommonSize_Generic`: 验证所有结构体字段的 100% 覆盖率。
-   `TestRatios_DuPont`: 验证 RNOA + 杠杆计算是否符合 Excel 布局。
-   `TestRiskModels`: 验证 Z-Score 计算是否与手动控制一致。

### 手动验证 (第 5 阶段)
-   **前端检查**:
    -   启动后端 (`go run cmd/api/main.go`)。
    -   启动前端 (`npm run dev`)。
    -   导航至 `localhost:3000/valuation-report`。
    -   点击 AAPL 的 "运行分析 (Run Analysis)"。
    -   验证所有新部分 (垂直分析, 利润率, 效率) 是否正确渲染。
