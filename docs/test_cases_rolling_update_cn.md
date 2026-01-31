
# 滚动更新与数据合成测试用例 (Test Cases: Rolling Update & Synthesis)

## 目标 (Objective)
验证 `SynthesisEngine` (数据合成引擎) 能否正确处理重述 (Restatement)、会计变更和异常数据，准确生成 "Golden Data"。

## 测试数据集 (Test Scenarios)

### Case 1: 正常的重述覆盖 (Normal Restatement)
*   **输入**:
    *   `Filing_2023.json`: { 2023: 100, 2022: **50** } (原始值)
    *   `Filing_2024.json`: { 2024: 120, 2023: **102**, 2022: **52** } (修正值)
*   **预期输出 (Golden)**:
    *   `2024`: 120 (最新)
    *   `2023`: **102** (覆盖旧值，差额 +2%)
    *   `2022`: **52** (覆盖旧值，差额 +4%)
*   **断言 (Assert)**: 
    *   `Master[2023] == 102`
    *   `Alerts` 包含: "2023 Restated (+2%)", "2022 Restated (+4%)"

### Case 2: 无新数据时的回退 (Fallback to Older)
*   **输入**:
    *   `Filing_2024.json`: { 2024: 120, 2023: 102 } (只披露了两年)
    *   `Filing_2022.json`: { 2022: 50, 2021: 40 }
*   **预期输出 (Golden)**:
    *   `2024`: 120 (来自 2024 Filing)
    *   `2023`: 102 (来自 2024 Filing)
    *   `2022`: **50** (来自 2022 Filing，因为 2024 Filing 没包含它)
    *   `2021`: 40  (来自 2022 Filing)
*   **断言**: 时间序列不中断。

### Case 3: 10-KA 优先于 10-K (Amendment Priority)
*   **输入**:
    *   `Filing_2023_10K.json`: { 2023: 100 } (Date: 2024-03-01)
    *   `Filing_2023_10KA.json`: { 2023: **95** } (Date: 2024-04-15)
*   **预期输出 (Golden)**:
    *   `2023`: **95**
*   **断言**: 必须选用 `FilingDate` 更晚的文件作为 source of truth。

### Case 4: 剧烈会计变更 (Radical Accounting Change)
*   **输入**:
    *   `Filing_2023.json`: { 2023: **1000** } (旧准则：把 GMV 算作收入)
    *   `Filing_2024.json`: { 2024: 200, 2023: **200** } (新准则：只算净佣金)
*   **预期输出 (Golden)**:
    *   `2024`: 200
    *   `2023`: **200** (覆盖)
*   **衍生计算**:
    *   `Growth_2024` = (200 - 200) / 200 = 0% (正确)
    *   *错误对照*: 如果不覆盖，Growth = (200 - 1000) / 1000 = -80% (虚假崩盘)。
*   **断言**: `Master[2023] == 200` 且 `Alert` 为 "Significant Restatement (-80%)".

### Case 5: 脏数据过滤 (Outlier/Glitch Protection)
*   **输入**:
    *   `Filing_2024.json`: { 2024: 0, 2023: 100 } (LLM 提取错误，把 2024 漏了填成 0)
*   **逻辑**: 
    *   我们在 Synthesis 阶段可以加入简单规则：如果最新一年的 Revenue 为 0 且公司未退市，标记为 `DATA_CORRUPTION`，拒绝更新或回退到上一版提取。
*   **预期**: 报警并提示人工介入。

## 测试执行 (Test Runner)

✅ **已实现 / Implemented**: `pkg/core/synthesis/zipper_test.go`

运行测试:
```bash
go test -v ./pkg/core/synthesis/...
```

所有 5 个 Case 均已通过自动化测试验证。

