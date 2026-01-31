Agentic Valuation Platform (FSAP Edition) - 产品需求文档

1. 项目愿景

构建一个基于 Agentic Workflow 的智能估值平台，自动化处理从“财报数据提取”到“多模型估值”的全流程。系统严格遵循 FSAP 模版的逻辑，利用 LLM 的语义理解能力处理数据清洗与归类，并确保数据的可追溯性与会计平衡。

2. 系统架构与职责划分

2.1 Backend (核心智能层 - Python/FastAPI)

Pipeline: Raw Data Ingestion -> OCR/Parsing -> Semantic Mapping -> Balance Check -> Valuation.

Agent A (The Accountant - 负责准):

任务: 自动读取财报，映射到 FSAP Data.csv。

会计恒等式守卫:

在映射每一行后，实时计算 Asset - (Liab + Equity)。

Auto-Plug 逻辑: 对于极小的误差（如舍入误差 < $1M），Agent 可自动计入 "Other Current Assets/Liabilities" 并标记 Note。

Red Flag: 对于大额不平衡，Agent 必须停止并请求人工介入 ("Human-in-the-loop")。

审计追踪: 每一个数字背后都有 Source (财报原图位置) 和 Logic (FSAP 归类规则)。

2.2 Frontend (交互层 - React)

Mapping Review UI: 允许用户通过对话或拖拽调整分类，调整后系统自动重算平衡性。

Valuation Dashboard: 展示四种模型的三角验证结果。

Dynamic Dependency Graph: (新增) 动态展示假设变化对估值的影响路径。

3. 核心功能：FSAP 四大估值模型

为了得到客观的估值，我们不再依赖单一模型，而是通过以下四种模型进行交叉验证。Agent 会自动根据数据运行所有模型。

3.1 股利折现模型 (Dividend Discount Model - DDM)

逻辑: 股票的价值等于未来所有股利的现值。

Agent 自动假设: Agent 会分析公司的历史股利支付率 (Payout Ratio) 和回购政策。

适用场景: 针对像福特这样成熟、分红稳定的公司。

3.2 自由现金流模型 (Free Cash Flow - DCF)

逻辑: 公司的价值等于未来产生的自由现金流 (FCF) 的现值。这是最主流的模型。

Agent 关键参数: 营收增长率、EBIT 利润率、资本支出 (Capex)。

联动: Agent 修改“营收增长”时，此模型波动最大。

3.3 剩余收益模型 (Residual Income Model - RIM)

逻辑: 价值 = 当前账面价值 (Book Value) + 未来预期“超额收益”的现值。

核心优势: 即使公司短期不分红或现金流为负，只要有盈利 (Net Income > 资本成本)，该模型依然有效。

Agent 监控: Agent 会特别关注“权益回报率 (ROE)”与“股权成本 (Cost of Equity)”的差额。

3.4 剩余收益市净率模型 (Residual Income Market-to-Book)

逻辑: 是 RIM 的一种变体，重点预测未来的市净率 (P/B Ratio) 走势。

Agent 视角: Agent 会搜索行业平均 P/B，以此作为终值计算的参考锚点。

4. 核心工作流 (The Agentic Loop)

阶段一：智能摄入与平衡 (Auto-Balance)

Agent: 读取 PDF -> 映射数据。

Check: 发现 Assets = 100亿, L+E = 98亿。差额 2亿。

Chat Action: Agent 主动询问用户：“我发现 2亿 的差额。经检查，Note 7 提到有一笔‘未决诉讼准备金’。应该归类为‘其他长期负债’吗？”

User: “是的。” -> Result: 平衡达成。

阶段二：假设推荐与调整 (Proactive Assumption)

Scenario: 用户不知道如何设定明年的利润率。

Agent: “基于福特过去 5 年的数据，平均净利率为 3.5%。但我搜索到分析师认为明年因 EV 竞争激烈，利润率可能承压。我为您生成了三种情景，您想应用哪一种？”

🟢 乐观 (Bull): 4.5% (新车型大卖)

🟡 中性 (Base): 3.5% (维持现状)

🔴 悲观 (Bear): 2.5% (价格战)

阶段三：结果追踪 (Traceability)

每一次参数修改（如从 3.5% 改为 4.0%），系统都会记录：

Who: User via Chat

When: 2023-10-27 10:00

Why: "Based on user input referencing new battery plant efficiency."

5. 可视化假设图谱 (Dynamic Diff Graph)

此模块旨在解决“黑盒”问题，清晰展示数据流向和改动点。

5.1 节点设计

每个节点代表一个关键变量（如 Revenue Growth, WACC, Base Cash），节点状态包括：

默认态 (Default): 显示当前值，背景为白色。

修改态 (Modified):

User Edited: 高亮为 蓝色，显示 Old Value -> New Value。

Agent Proposed: 高亮为 紫色，显示 Old Value -> New Value。

来源标记:

Data Ingestion: 标记来自财报原始数据的节点（如 Base Year Revenue）。

Assumption: 标记来自预测表的假设节点。

5.2 联动逻辑

当用户在 Ingestion 阶段修改了“Cash”分类 -> 图谱中的 Base Cash 节点高亮 -> Net Debt 节点高亮 -> Equity Value 节点高亮。

用户可以直观看到一次小的分类调整如何通过杠杆效应影响最终股价。

6. 技术栈

Logic: Python (pandas) 复现 FSAP 公式。

Frontend: React + Recharts (用于展示三角验证范围图)。