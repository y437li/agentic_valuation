
# 滚动年份更新与重述校准策略 (Decoupled Architecture)

## 1. 核心设计哲学 (Core Philosophy)

我们采用 **“提取 (Extraction) 与 合成 (Synthesis) 解耦”** 的模式。
*   **Extraction (Immutable)**: 一次性的、观察性质的。LLM "看到"了 2024年 10-K 里 2023年的收入是 $100M。这是一个不可变的事实（Snapshot）。
*   **Synthesis (Mutable)**: 构建性质的。我们基于多个不可变的 Snapshots，构建出一个当前认为最接近真相的 "Golden Timeline"。

---

## 2. 核心数据结构 (Data Structures)

为了支持复杂的重述和审计，我们需要比简单的 JSON 更严谨的结构。

### `GoldenRecord` (最终输出)
前端直接消费的数据结构，代表“当前最佳认知”。

```go
type GoldenRecord struct {
    Ticker        string
    LastUpdated   time.Time
    // 主时间轴：Key 是年份 (e.g., 2023)
    Timeline      map[int]YearlyData 
    // 审计日志：记录了所有的“冲突”和“修正”
    Restatements  []RestatementLog
}

type YearlyData struct {
    Revenue       float64
    NetIncome     float64
    // ... 其他字段
    SourceFiling  string // e.g., "ACC-2024-001" (数据来源的具体文件 Accession)
    OriginalValue float64 // 如果发生了重述，这里记录被覆盖的旧值
}
```

### `RestatementLog` (差异审计)
```go
type RestatementLog struct {
    Year          int
    Item          string  // e.g., "Revenue"
    OldValue      float64 // 来自 2023 10-K
    NewValue      float64 // 来自 2024 10-K
    DeltaPercent  float64 // e.g., -5.2%
    DetectedAt    time.Time
    Source        string  // "Found in 2024 10-K comparison column"
}
```

---

## 3. "The Zipper" 合并算法 (Synthesis Logic)

这是本策略的核心。我们需要像拉链一样，将不同年份的文件无缝拼接，并优先信任最新数据。

**输入**: 一个 Ticker 的所有 `RawExtraction` 列表 (按 `FilingDate` 倒序排列: 2024_10K, 2023_10K, 2022_10K...)

**算法步骤**:

1.  **初始化**: 创建空的 `MasterTimeline`。
2.  **文件遍历 (File Iteration)**: 
    *   从 **最新** 的文件开始遍历 (e.g., 2024 10-K)。
    *   该文件通常包含 [2024, 2023, 2022] 三列数据。
3.  **年份遍历 (Year Processing)**:
    *   对于文件中的每一年 `Y` (e.g., 2023):
        *   **Check 1: 填补空白 (Fill Gap)**
            *   如果 `MasterTimeline[Y]` 为空 -> **直接写入**。
            *   标记 `Source = 2024_10K`。
        *   **Check 2: 发现重述 (Detect Restatement)**
            *   如果 `MasterTimeline[Y]` 已存在 (意味着我们已经从更旧的文件，或者同一年更晚的 10-K/A 中读到了数据？*注：由于我们是倒序遍历，所以已存在的数据一定是由“更新或同级”的文件写入的。这里逻辑需要精准。*)
            *   **修正逻辑**: 
                *   正确的遍历顺序应该是：**Time Descending (Latest Filing First)**。
                *   当我们第一次遇到 2023年的数据（在 2024 10-K 中），我们写入 Master。
                *   当我们第二次遇到 2023年的数据（在 2023 10-K 中），我们发现 Master 里已经有了（来自 2024 10-K）。
                *   **此时进行对比**:
                    *   `Value_In_2023_File` (Old) vs `Master_Value_from_2024` (New)。
                    *   如果不一致 -> **记录 Restatement Log**。
                    *   **不覆盖**: 因为 Master 里的数据来自更新的文件 (2024)，所以 Master 保持不变。
4.  **异常保护 (Outlier Guard)**:
    *   如果在最新文件 (2024) 中，某个关键字段 (Revenue) 为 0 或 null，但旧文件 (2023) 中有值。
    *   **策略**: 默认信任最新文件（假设是公司倒闭或业务变更），但生成 `CRITICAL_ALERT`。除非是明确的 `Extraction Error` (需结合 LLM Metadata 判断)。

---

## 4. 智能干预层 (Agentic Intervention Layer)

虽然 "Zipper" 算法能解决 95% 的数学问题，但它无法解决 **"为什么"** 的问题。我们引入 **Layer 2 Agent** 来补充纯代码逻辑。

### 角色 1: 异常法官 (The Judge)
*   **触发**: 当 `Outlier Guard` 检测到剧烈的数据变化 (e.g., Revenue Drop > 50%)。
*   **任务**:
    *   阅读最新 Filing 的 "Discontinued Operations" 或 "Basis of Presentation" 章节。
    *   **判断**: 这是真实的业务崩盘，还是数据提取错误？
    *   **输出**: `VERDICT_OVERRIDE` (强制覆盖) 或 `VERDICT_REJECT` (拒绝脏数据)。

### 角色 2: 重述侦探 (The Detective) - *Institution-Grade Feature*
*   **触发**: 当 `Zipper` 成功合并并生成 `RestatementLog` 后。
*   **痛点**: 只有数字变化的图表会让用户困惑。
*   **任务**:
    *   拿着差异点 (e.g., "2023 Revenue down by $5M") 去最新 Filing 中搜索。
    *   寻找关键词: "Restatement", "Accounting Standard Update", "Reclassification".
    *   **输出**: 一句简短的解释 (Human-Readable Reason)。
    *   **UI表现**: 用户鼠标悬停在重述数据上时，显示 Tooltip: *"Restated due to adoption of ASU 2016-13 (Revenue Recognition)."*

---

## 5. 边界情况处理策略 (Edge Case Policies)

| 情景 | 策略 | 解释 |
| :--- | :--- | :--- |
| **Case 1: 10-K/A (修正案)** | **Amendment Dominance** | 如果同一年有 10-K 和 10-K/A，按 Filing Date 排序，10-K/A 自然排在前面，其数据会被优先写入 Master。 |
| **Case 2: 会计准则变更** | **Recency Bias** | 如 Case 4 (GMV -> Net Revenue)。2024文件里的2023数据是按新准则算的。我们保留这个新值，抛弃 2023文件里的旧值。这保证了 Trend Analysis (24 vs 23) 是 Apple-to-Apple 的。 |
| **Case 3: 缺失年份** | **Gap Filling** | 如果 2024文件只写了 [24, 23]，而 2022文件写了 [22, 21]。算法会自动保留 24/23 (来自 2024文件) 并填补 22/21 (来自 2022文件)。 |
| **Case 4: 货币单位变更** | **Metadata Check** | (需实施) 提取层必须提取 Unit (Millions/Thousands)。合成层在比较前必须 Normalize 到统一单位。 |

---

## 6. 实施路线图 (Implementation Roadmap)

### Phase 1: 基础架构 (当前)
- [x] `fsap_extractions` 表 (存储 Raw JSON)。
- [ ] `pkg/core/synthesis` 包结构搭建。

### Phase 2: 核心引擎实现 (Zipper)
- [ ] 实现 `Merge(ticker string)` 函数。
- [ ] 实现 `DetectRestatement(old, new)` 逻辑。
- [ ] 单元测试覆盖 5 个核心 Case (见 `test_cases_rolling_update_cn.md`)。

### Phase 3: 智能层接入 (Agents)
- [ ] 实现 `JudgeAgent` 接口：当 Zipper 抛出 `ErrSignificantDeviation` 时调用。
- [ ] 实现 `DetectiveAgent` 接口：异步消费 `RestatementLog` 队列。

### Phase 4: API 与缓存
- [ ] `GET /api/valuation/consolidated` 接口。
- [ ] 内存缓存：因为 Synthesis 是纯 CPU 计算，对于没新文件的公司，应该缓存结果避免每次重复计算。

---

## 7. 附录：为什么这是“机构级”的？
Bloomberg 和 FactSet 的核心价值不在于 OCR，而在于清洗和对齐 (Alignment)。
通过 **"Latest Disclosure is Truth"** 原则，我们确保了用户在做 Growth Rate 分析时，分子分母总是基于同一口径（Restated Basis），避免了因为会计变更导致的虚假高增长或暴跌。
结合 **Agentic Intervention**，我们不仅给出了正确的数字，还给出了数字背后的 *原因*，这是传统 Quant 数据库做不到的。
