package templates

var FounderOSTemplate = StarterTemplate{
	Name:        "founder",
	Description: "Startup founder vault with decision records, investor tracking, and market research",
	Folders:     []string{"50-people", "60-companies"},
	Files: map[string]string{
		"10-notes/company-overview.md": `---
id: note_founder_001
type: note
title: Company Overview
status: active
tags: [company, overview]
created: 2026-01-15
updated: 2026-01-15
---

# Company Overview

## Mission

[Your mission statement here]

## Vision

[Your vision statement here]

## Key Metrics

| Metric | Value | Target |
|--------|-------|--------|
| MRR | $0 | $10K |
| Users | 0 | 100 |
| Team | 1 | 3 |

## Key Links

- Website: [url]
- GitHub: [url]
- Demo: [url]
`,
		"30-decisions/pricing-model.md": `---
id: dec_founder_001
type: decision
title: Adopt Usage-Based Pricing Over Seat-Based
status: active
confidence: medium
project: company
tags: [pricing, revenue, monetization]
created: 2026-02-10
updated: 2026-02-10
review_after: 2026-05-10
---

# Adopt Usage-Based Pricing Over Seat-Based

## Decision

Use a usage-based pricing model (per API call / per token) instead of per-seat pricing.

## Reasoning

- Better aligns cost with customer value
- Lowers barrier to entry for small teams
- Natural upsell as usage grows
- Matches how AI infrastructure is typically priced

## Tradeoffs

- More complex billing system
- Revenue harder to predict
- Customers may fear runaway costs

## Revisit when

- We have 10+ paying customers
- We understand actual usage patterns
- We hear consistent feedback on pricing
`,
		"30-decisions/tech-stack.md": `---
id: dec_founder_002
type: decision
title: Use Go and React for Initial Stack
status: active
confidence: high
project: company
tags: [architecture, tech-stack]
created: 2026-01-10
updated: 2026-01-10
review_after: 2026-07-10
---

# Use Go and React for Initial Stack

## Decision

Use Go for backend, React for frontend, SQLite for database, Ollama for local AI.

## Reasoning

- Go: fast, simple, single binary deployment
- React: large ecosystem, easy to hire for
- SQLite: zero-config, sufficient for early stage
- Ollama: free local AI, no API costs for development

## Tradeoffs

- Go is verbose compared to Python/Node
- SQLite does not scale horizontally (but we don't need that yet)
- React bundle size vs alternatives

## Revisit when

- We need real-time collaboration features
- SQLite becomes a bottleneck (>100GB data)
- We need multi-region deployment
`,
		"30-decisions/go-to-market.md": `---
id: dec_founder_003
type: decision
title: Developer-First GTM via Open Source
status: active
confidence: medium
project: company
tags: [gtm, marketing, strategy]
created: 2026-03-01
updated: 2026-03-01
review_after: 2026-06-01
---

# Developer-First GTM via Open Source

## Decision

Launch as open-source project on GitHub, build community first, then offer paid hosted version.

## Reasoning

- Developers discover tools through GitHub and HN
- Open source builds trust and viral growth
- Free tier generates usage data and feedback
- Clear upgrade path to paid (sync, teams, hosted AI)

## Tradeoffs

- Slower revenue ramp vs direct sales
- Need to manage community expectations
- Risk of forks/competition

## Revisit when

- We have 1K+ GitHub stars
- First enterprise inquiry comes in
- We need to hire a sales team
`,
		"30-decisions/hiring-first-engineer.md": `---
id: dec_founder_004
type: decision
title: Hire Full-Stack Engineer as First Employee
status: active
confidence: low
project: company
tags: [hiring, team]
created: 2026-03-15
updated: 2026-03-15
review_after: 2026-06-15
---

# Hire Full-Stack Engineer as First Employee

## Decision

Hire a senior full-stack engineer (Go/React) as the first employee, not a specialist.

## Reasoning

- Small team needs generalists
- Can own features end-to-end
- Reduces communication overhead
- Easier to evaluate in interview process

## Tradeoffs

- May lack deep expertise in specific areas
- Single point of failure for critical systems
- Harder to find true full-stack seniors

## Revisit when

- We have 5+ engineers (then hire specialists)
- Specific domain expertise becomes critical (AI/ML, security)
`,
		"50-people/investor-template.md": `---
id: person_founder_001
type: person
title: Investor Name
status: active
tags: [investor, fundraising]
created: 2026-01-20
updated: 2026-01-20
---

# [Investor Name]

## Role

[Partner at Firm / Angel Investor]

## Firm

[Firm name, AUM, stage focus]

## Contact

- Email: [email]
- LinkedIn: [url]
- Warm intro via: [name]

## Thesis Fit

[Why they might be interested in us]

## Meeting History

| Date | Type | Notes |
|------|------|-------|
| 2026-01-20 | Intro | [notes] |

## Next Steps

- [ ] Follow up with [specific action]
`,
		"60-companies/competitor-profile-template.md": `---
id: company_founder_001
type: company
title: Competitor Name
status: active
tags: [competitor, landscape]
created: 2026-01-25
updated: 2026-01-25
---

# [Competitor Name]

## Overview

[What they do, target market]

## Funding

- Total raised: $[amount]
- Last round: [Series/amount/date]
- Valuation: $[amount]

## Key Metrics

- Users: [estimate]
- Pricing: [model and price point]
- Team size: [estimate]

## Strengths

- [strength 1]
- [strength 2]

## Weaknesses

- [weakness 1]
- [weakness 2]

## Our Differentiation

[How we differ and why it matters]
`,
		"40-research/market-size.md": `---
id: research_founder_001
type: source
title: Market Size - Personal Knowledge Management
status: active
tags: [market, tam, research]
created: 2026-02-01
updated: 2026-02-01
---

# Market Size - Personal Knowledge Management

## TAM (Total Addressable Market)

Global knowledge management software: ~$XXB by 2028

## SAM (Serviceable Available Market)

Note-taking and PKM tools for developers/professionals: ~$X.XB

## SOM (Serviceable Obtainable Market)

AI-native, local-first PKM for developers and founders: ~$XXM

## Key Trends

- Growing demand for AI-personalized tools
- Privacy concerns driving local-first adoption
- Developer tools market expanding rapidly

## Sources

- [Gartner report]
- [Industry analysis]
`,
		"20-projects/product-v1.md": `---
id: prj_founder_001
type: project
title: Product v1.0 Launch
status: active
tags: [product, v1, launch]
created: 2026-01-05
updated: 2026-01-05
---

# Product v1.0 Launch

## Goals

- [ ] Core vault management (init, index, search)
- [ ] Desktop app with editor
- [ ] AI ask functionality
- [ ] MCP server for agent integration

## Key Decisions

- See: [[Use Go and React for Initial Stack]]
- See: [[Adopt Usage-Based Pricing Over Seat-Based]]

## Timeline

| Milestone | Target Date |
|-----------|-------------|
| Alpha | 2026-02 |
| Beta | 2026-03 |
| Public | 2026-04 |
`,
		"70-prompts/weekly-review.md": `---
id: prompt_founder_001
type: prompt
title: Weekly Review Prompt
status: active
tags: [prompt, review, founder]
created: 2026-01-15
updated: 2026-01-15
---

# Weekly Review Prompt

## Context

I am a startup founder building AgentVault, a local-first AI knowledge operating system. Help me review my week.

## Prompt

Review the decisions, tasks, and research notes from this week. Provide:

1. Key decisions made and their current status
2. Open tasks prioritized by impact
3. Any decisions that need revisiting based on new information
4. Recommended focus areas for next week
5. Any risks or blockers I should address

Use only the information from my vault. Do not invent facts.
`,
	},
}
