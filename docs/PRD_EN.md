# TIED Platform - Product Requirements Document

---

## Executive Summary

TIED (Transparent Investment Evidence & Discovery) is an AI-powered valuation platform that democratizes financial analysis by enabling anyone to build DCF models, generate assumptions through multi-agent debates, and participate in a transparent assumption consensus marketplace.

---

## Problem Statement

1. **High Barrier to Entry**: Traditional valuation requires Excel expertise, financial training, and hours of manual work
2. **Opaque Assumptions**: Sell-side analyst reports don't show the reasoning behind their assumptions
3. **No Assumption Marketplace**: Unlike stock prices, there's no transparent market for valuation assumptions
4. **Information Asymmetry**: Institutional investors have access to better analysis tools than retail

---

## Value Proposition

> **"Democratizing valuation, creating transparent assumption consensus"**

### Core Value

| Value | Description |
|-------|-------------|
| AI-assisted DCF modeling | Build complete valuation models in minutes |
| Multi-agent debate | 11 specialized agents generate high-quality assumptions |
| Full evidence tracing | Every number links to source document |
| Market consensus | Aggregated assumption consensus (new data type!) |
| Gap analysis | Discover divergence between assumptions and stock price |

---

## Target Users

| Persona | Description | Primary Need |
|---------|-------------|--------------|
| **Students** | MBA students, CFA candidates learning valuation | Easy-to-use tool |
| **Independent Analysts** | Analysts wanting to prove their track record | Reputation building |
| **Professional Investors** | Fund managers needing coverage expansion | Scale analysis with AI |
| **Institutions** | Hedge funds seeking alternative data | Consensus data feed |

---

## Features

### Phase 1: Core Platform

| Feature | Description | Priority |
|---------|-------------|----------|
| **10-K Parser** | Auto-extract financial data from SEC filings | P0 |
| **DCF Builder** | One-click DCF model generation | P0 |
| **Multi-Agent Debate** | 11 specialized agents debate assumptions | P0 |
| **Evidence Tracing** | Every number links to source document | P0 |
| **Assumption Editor** | Users can adjust AI-generated assumptions | P0 |

### Phase 2: Social & Verification

| Feature | Description | Priority |
|---------|-------------|----------|
| **Assumption Submission** | Users submit their assumptions publicly | P1 |
| **Earnings Settlement** | Auto-verify assumptions when earnings release | P1 |
| **Reputation System** | Track record scoring based on accuracy | P1 |
| **Leaderboard** | Rank users by prediction accuracy | P1 |
| **Consensus View** | Aggregated market consensus on assumptions | P1 |

### Phase 3: Marketplace

| Feature | Description | Priority |
|---------|-------------|----------|
| **Expert Packages** | Top analysts sell assumption packages | P2 |
| **Unlock Purchase** | Users pay to access expert assumptions | P2 |
| **Gap Analysis** | Show divergence between consensus and price | P2 |
| **Data API** | Institutional data feed for consensus | P2 |
| **Model Training** | Use verified assumptions to fine-tune models | P2 |

---

## Business Model

### Revenue Streams

```
┌─────────────────────────────────────────────────────────────────────┐
│                                                                     │
│  Layer 1: Tool Fees                                                 │
│  ────────────────────────────────────────────────────────────────── │
│  • BYOK (Bring Your Own Key): FREE                                  │
│  • Hosted LLM: ~$0.01-0.05 per assumption                           │
│                                                                     │
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│  Layer 2: Subscription                                              │
│  ────────────────────────────────────────────────────────────────── │
│  • Free: Basic features, limited usage                              │
│  • Pro ($29/mo): Private Agent config, unlimited debates            │
│                                                                     │
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│  Layer 3: Creator Economy (Core Monetization)                       │
│  ────────────────────────────────────────────────────────────────── │
│  • Expert sells assumption package: $10                             │
│  • Platform take rate: 30% ($3)                                     │
│  • Expert keeps: 70% ($7)                                           │
│                                                                     │
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│  Layer 4: Data Licensing                                            │
│  ────────────────────────────────────────────────────────────────── │
│  • Consensus data feed: $10,000+/year                               │
│  • Gap analysis signals: $50,000+/year                              │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘
```

---

## Go-to-Market Strategy

### Phase 1: Education & Community (Year 1)

| Channel | Target | Pricing |
|---------|--------|--------|
| **Schools** | Rotman → Canadian MBA → US Top MBA | $1000-5000/year |
| **Communities** | Investment clubs, CFA groups, Discord communities | $99-299/month |
| **Goal** | Build user base, collect data, refine product |
| **Revenue Target** | $0-50k (not priority) |

### Phase 2: Buy-Side (Year 2)

| Channel | Target | Pricing |
|---------|--------|--------|
| **Hedge Funds** | Consensus data API | $10k-50k/year |
| **Asset Managers** | Alternative data signals | $20k-100k/year |
| **Goal** | Validate institutional willingness to pay |
| **Revenue Target** | $100k-500k |

### Phase 3: Sell-Side (Year 3+)

| Channel | Target | Pricing |
|---------|--------|--------|
| **Investment Banks** | Research analyst tools | Custom |
| **Brokerages** | Value-added client services | Custom |
| **Goal** | Replace parts of FactSet/Bloomberg |
| **Revenue Target** | $1M+ |

---

## Verification Mechanism

### What Can Be Verified

| Assumption Type | Verification | Timeline | Included in Ranking |
|-----------------|--------------|----------|---------------------|
| Current Quarter (Q1) | ✅ Earnings release | ~3 months | ✅ Yes |
| Current Year (FY) | ✅ Annual report | ~12 months | ✅ Yes |
| Future Years (FY+1, +2...) | ❌ Not yet verifiable | Years | ❌ No (display only) |

### Reputation Scoring

```
Reputation = Σ (Accuracy × Time Weight × Difficulty)
```

---

## Gap Analysis Feature

### Assumption Consensus Heatmap

Users input assumptions across multiple dimensions. The platform aggregates all inputs to show where consensus forms:

```
┌────────────────────────────────────────────────────────────────────────────────────────────────┐
│                    FORD (F) - FY 2026 ASSUMPTION CONSENSUS HEATMAP                             │
├──────────┬──────────┬──────────┬──────────┬──────────┬──────────┬══════════════════════════════┤
│ Revenue  │ EBIT     │ CapEx    │  WACC    │ Terminal │ Tax      │                              │
│ ($B)     │ Margin % │   ($B)   │    %     │ Growth % │ Rate %   │   → IMPLIED PRICE            │
├──────────┼──────────┼──────────┼──────────┼──────────┼──────────┤                              │
│   $190   │   6.5%   │   $9.5   │   8.5%   │   3.0%   │   20%    │       $18.00  🟢             │
│   ░░░    │   ░░░    │   ░░░    │   ░░░    │   ░░░    │   ░░░    │                              │
│          │          │          │          │          │          │                              │
│   $185   │   6.0%   │   $10    │   9.0%   │   2.5%   │   21%    │       $15.50  🟢             │
│   ░░░    │   ░░░    │   ░░░    │   ░░░    │   ░░░    │   ░░░    │                              │
│          │          │          │          │          │          │                              │
│   $180   │   5.5%   │   $10.5  │   9.5%   │   2.0%   │   22%    │   ▓▓▓ $14.00  ◀ CONSENSUS   │
│   ▓▓▓    │   ▓▓▓    │   ▓▓▓    │   ▓▓▓    │   ▓▓▓    │   ▓▓▓    │                              │
│          │          │          │          │          │          │                              │
│   $175   │   5.0%   │   $11    │  10.0%   │   1.5%   │   23%    │       $12.00  🟡             │
│   ░░░    │   ░░░    │   ░░░    │   ░░░    │   ░░░    │   ░░░    │                              │
│          │          │          │          │          │          │                              │
│   $170   │   4.5%   │   $11.5  │  10.5%   │   1.0%   │   24%    │       $10.50  🔴             │
│   ░░░    │   ░░░    │   ░░░    │   ░░░    │   ░░░    │   ░░░    │                              │
├──────────┴──────────┴──────────┴──────────┴──────────┴──────────┼──────────────────────────────┤
│  ░░░ = Few users       ▓▓▓ = Consensus zone (most users)        │  Current Price: $10.00       │
│  All assumptions contribute to consensus calculation            │  Gap: +40% (Undervalued?)    │
└─────────────────────────────────────────────────────────────────┴──────────────────────────────┘
```

### Assumption Categories

| Type | Assumptions | Verifiable? |
|------|-------------|-------------|
| **Operating** | Revenue, EBIT Margin, CapEx, D&A | ✅ Earnings verify |
| **Valuation** | WACC, Terminal Growth, Tax Rate | ❌ Not verifiable (market judgment) |

### Key Insight

WACC and Terminal Growth cannot be "verified", but the consensus distribution itself is valuable information - it shows what discount rate the crowd believes is appropriate.

### Year 2+ Assumptions

| Year | User Input? | How Calculated |
|------|-------------|----------------|
| Year 1 (FY2026) | ✅ Heatmap selection | User submits |
| Year 2-5 | ❌ Auto-calculated | Apply consensus growth rate |

---

## Competitive Landscape

| Competitor | What They Do | What TIED Does Differently |
|------------|--------------|---------------------------|
| **Bloomberg** | Data terminal | AI-powered modeling, not just data display |
| **FactSet** | Financial data | Assumption generation, not just data retrieval |
| **Brightwave** | Document Q&A | Full DCF modeling, not just text summarization |
| **Visible Alpha** | Sell-side consensus | Crowdsourced consensus with verification |
| **Koyfin** | Data visualization | Valuation models, not just charts |

### TIED's Unique Moat

1. **Assumption Verification Data**: Historical accuracy records (unique dataset)
2. **Creator Network**: Expert analysts with proven track records
3. **Gap Analysis Signals**: Divergence between fundamentals and price

---

## Technical Architecture

### Current Implementation

| Component | Technology | Status |
|-----------|------------|--------|
| Frontend | Next.js + React | ✅ Implemented |
| State Management | Zustand | ✅ Implemented |
| Agent System | 11 Specialized Agents | ✅ Implemented |
| LLM Providers | OpenAI, Gemini, DeepSeek, Qwen | ✅ Implemented |
| Data Pipeline | SEC Edgar Integration | ✅ Implemented |
| Evidence Tracing | Full provenance tracking | ✅ Implemented |

### Data Granularity Roadmap

| Phase | Granularity | Target Users | Data Source |
|-------|-------------|--------------|-------------|
| **MVP** | Annual (10-K) | Students, Communities | SEC Edgar 10-K |
| **V2** | Quarterly (10-Q) | Professional Analysts | SEC Edgar 10-Q |
| **V3** | Real-time estimates | Institutions | Earnings calls, 8-K |

### Roadmap for Marketplace

| Component | Technology | Status |
|-----------|------------|--------|
| User Authentication | Supabase Auth | 🔲 Planned |
| Assumption Database | PostgreSQL | 🔲 Planned |
| Settlement Engine | Cron + SEC API | 🔲 Planned |
| Payment System | Stripe | 🔲 Planned |
| Data API | REST/GraphQL | 🔲 Planned |
| Training Pipeline | PyTorch + MLflow | 🔲 Planned |

---

## Backend Model Training / 后台模型训练

### Purpose

User-generated assumptions, once verified against actual earnings, become **high-quality training data**. This creates a flywheel:

```
Users submit assumptions → Earnings verify → Accurate data collected → Model fine-tuned → Better AI suggestions → More users
```

### Training Data Collection

| Data Type | Source | Value |
|-----------|--------|-------|
| Assumption + Actual | Verified predictions | Ground truth for fine-tuning |
| User adjustments | Parameter changes | Reveals expert intuition |
| Confidence levels | Stake amounts | Weighted training signal |
| Reasoning text | User explanations | Chain-of-thought training |

### Training Strategy

1. **Continuous Learning**: Fine-tune base models on verified assumptions quarterly
2. **Sector-Specific Models**: Train specialized models for different industries
3. **User-Personalized Agents**: Optional fine-tuning on individual user's history

### Privacy & Consent

- All training data is anonymized
- Users opt-in to contribute training data (with rewards)
- No PII included in training sets

---

## Success Metrics

### Phase 1 (0-6 months)

| Metric | Target |
|--------|--------|
| Registered Users | 10,000 |
| Models Created | 50,000 |
| Pro Conversion | 5% |

### Phase 2 (6-12 months)

| Metric | Target |
|--------|--------|
| Assumptions Submitted | 100,000 |
| Verified Predictions | 10,000 |
| Expert Creators | 100 |

### Phase 3 (12-24 months)

| Metric | Target |
|--------|--------|
| Revenue | $1M ARR |
| Data Clients | 10 institutions |
| Accuracy vs. Analysts | >60% |

---

## Risks & Mitigations

| Risk | Severity | Mitigation |
|------|----------|------------|
| LLM Homogenization | 🟡 Medium | Allow BYOK + custom Agent configs |
| Regulatory Issues | 🔴 High | Position as "game" not "financial product" initially |
| Cold Start | 🟡 Medium | Seed with AI-generated assumptions |
| Competition | 🟡 Medium | Focus on speed, open-source community |

---

*Document Version: 1.0*
*Last Updated: 2026-01-20*
*Author: TIED Team*
