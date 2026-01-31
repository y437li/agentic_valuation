
1. 我的目标是做一个工具可以通过agentic来自动从财报里把这些信息总结并填进这个对应的文件里。然后我可以跟另一个agent对话，可以列我的假设调整关键的参数。然后在网上可以给我对应的reference，而且他还可以帮我看研报，告诉我分析师的假设已经通过搜索互联网给我一个我的假设的confidence interval。你懂我的意思吗？我把这个价agentic valuation，我有fmp的金融数据，我还可以把分析师的研报和往上的相关信息关联过来。然后这个流程需要可视化，还有我假设的联系性所以应该是有个可视化的图。我可以点每个模块，可以根据对话来，像ant-gravity一样接受或者拒绝假设。然后这个对话记录是需要可追踪的，是可以之后被人工validate。然后我也可以点进去认为的调整参数。你还可以想想其他的一些功能。先做一个文档。然后再做一个demo。我把这个项目叫agentic_valuation

2.让agents来提取并根据note来划分数据的归属。data部分

3.复杂的“语义映射”过程（读取 Note -> 理解上下文 -> 分类数据）绝对是 Backend (后台) 的核心任务。前端只是用来展示后台处理的结果，并提供人工审核（Human-in-the-loop）的接口。前端有专门界面来展示“映射逻辑”供用户审核。

4. Financial Modeling Prep (FMP):

强项: 你已经有了这个。它最强的地方在于提供结构化的 JSON 数据。

适用场景: 直接填充 Data.csv 的核心数字（Revenue, Net Income, Cash, etc.）。

API Endpoints: /income-statement, /balance-sheet-statement, /cash-flow-statement。

Agent 策略: 直接调用，不需要太复杂的解析，准确率高。

SEC-API.io (强烈推荐配合):

强项: 专门做 SEC EDGAR 的解析。它不是只给你数字，而是把 10-K/10-Q 拆解成 JSON。

杀手级功能: 它可以提取特定的 Section，比如 Item 7 (MD&A) 或 Item 8 (Financial Statements and Supplementary Data)。

适用场景: 你的 Agent 需要读取 "Notes to Financial Statements" 来做决策（比如区分 Restricted Cash）。SEC-API 可以直接把 Notes 部分提取为文本发给 LLM。

5.你需要把为什么把哪笔钱归到哪里写好note，可查。然后保证最后是平衡的

6.我的目的是去除繁琐的收集数据和分类，而是自动的让agent去分好。当然允许人去调整回来。但最后需要保证平衡。每一步改变需要可以被追踪。支持聊天方式，直接修改对应部分。你还需要可以提供可能的假设，如果使用者想不到的话。使用者可以通过跟你聊天修改相应的假设参数。从而有心的估值结果。

7.核心功能：FSAP 四大估值模型

为了得到客观的估值，我们不再依赖单一模型，而是通过以下四种模型进行交叉验证。Agent 会自动根据数据运行所有模型。

股利折现模型 (Dividend Discount Model - DDM)

逻辑: 股票的价值等于未来所有股利的现值。

Agent 自动假设: Agent 会分析公司的历史股利支付率 (Payout Ratio) 和回购政策。

适用场景: 针对像福特这样成熟、分红稳定的公司。

自由现金流模型 (Free Cash Flow - DCF)

逻辑: 公司的价值等于未来产生的自由现金流 (FCF) 的现值。这是最主流的模型。

关键参数: 营收增长率、EBIT 利润率、资本支出 (Capex)。

Agent 修改“营收增长”时，此模型波动最大。

剩余收益模型 (Residual Income Model - RIM)

逻辑: 价值 = 当前账面价值 (Book Value) + 未来预期“超额收益”的现值。

核心优势: 即使公司短期不分红或现金流为负，只要有盈利 (Net Income > 资本成本)，该模型依然有效。

Agent 监控: Agent 会特别关注“权益回报率 (ROE)”与“股权成本 (Cost of Equity)”的差额。

剩余收益市净率模型 (Residual Income Market-to-Book)

逻辑: 是 RIM 的一种变体，重点预测未来的市净率 (P/B Ratio) 走势。

Agent 视角: Agent 会搜索行业平均 P/B，以此作为终值计算的参考锚点。

8.可视化假设图谱需要更加详细一些，每一个部分要显示我们做的关键改变。比如改了改了agentic的默认分类，已经一些关键假设需要能看见。

9.

