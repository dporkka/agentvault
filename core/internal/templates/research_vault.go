package templates

var ResearchVaultTemplate = StarterTemplate{
	Name:        "research",
	Description: "Research vault with source management, research briefs, and literature review tools",
	Folders:     []string{},
	Files: map[string]string{
		"10-notes/research-methodology.md": `---
id: note_research_001
type: note
title: Research Methodology Framework
status: active
tags: [methodology, framework, research]
created: 2026-01-15
updated: 2026-01-15
---

# Research Methodology Framework

## Process

1. **Define Question**: What exactly are we trying to understand?
2. **Source Search**: Find relevant papers, reports, experts
3. **Source Evaluation**: Assess credibility and relevance
4. **Extraction**: Pull key findings, data, quotes
5. **Synthesis**: Combine findings across sources
6. **Conclusion**: Answer the original question

## Source Credibility Scale

| Level | Description | Example |
|-------|-------------|---------|
| A | Peer-reviewed, primary data | Nature/Science paper |
| B | Expert analysis, reputable org | McKinsey, Gartner |
| C | Industry publication | Tech blogs, newsletters |
| D | Unverified, anecdotal | Forum posts, tweets |

## Note Types

- **Sources**: Individual papers/reports with full metadata
- **Briefs**: Synthesized answers to specific questions
- **Decisions**: Choices made based on research findings
`,
		"40-research/source-template.md": `---
id: src_research_001
type: source
title: Source Title
status: active
tags: [source, research]
created: 2026-01-20
updated: 2026-01-20
---

# [Source Title]

## Metadata

| Field | Value |
|-------|-------|
| Author(s) | [names] |
| Date | [YYYY-MM-DD] |
| Publisher | [journal/org] |
| URL | [link] |
| Credibility | [A/B/C/D] |
| Type | [paper/report/article/book] |

## Key Findings

- [Finding 1 with page/section reference]
- [Finding 2 with page/section reference]

## Relevant Quotes

> "[Direct quote]" — [page/section]

## Personal Notes

[Your analysis, connections to other sources, questions raised]

## Related Sources

- [[Related Source 1]]
- [[Related Source 2]]
`,
		"40-research/research-brief-template.md": `---
id: brief_research_001
type: note
title: Research Brief - [Question]
status: active
tags: [brief, synthesis, research]
created: 2026-01-25
updated: 2026-01-25
---

# Research Brief - [Question]

## Question

[The specific question this brief answers]

## Answer (Executive Summary)

[2-3 sentence answer]

## Key Evidence

1. [Finding from Source A]
2. [Finding from Source B]
3. [Finding from Source C]

## Sources

| Source | Relevance | Key Finding |
|--------|-----------|-------------|
| [Title] | High | [finding] |
| [Title] | Medium | [finding] |

## Confidence

[High / Medium / Low] — [explanation]

## Gaps

[What we still don't know]

## Recommendations

[Actionable recommendations based on findings]
`,
		"40-research/literature-review-outline.md": `---
id: note_research_002
type: note
title: Literature Review - [Topic]
status: active
tags: [literature-review, research]
created: 2026-02-01
updated: 2026-02-01
---

# Literature Review - [Topic]

## Scope

[What is included and excluded]

## Search Strategy

[Databases, keywords, date range]

## Themes

### Theme 1: [Name]
- [Source 1 finding]
- [Source 2 finding]
- Gap: [what's missing]

### Theme 2: [Name]
- [Source 3 finding]
- [Source 4 finding]
- Gap: [what's missing]

## Synthesis

[How themes connect, overall picture]

## Research Gaps

[Opportunities for original contribution]

## Bibliography

- [Full citation 1]
- [Full citation 2]
`,
		"60-companies/competitive-intelligence-template.md": `---
id: company_research_001
type: company
title: "[Company] - Competitive Intelligence"
status: active
tags: [competitor, intelligence, research]
created: 2026-02-10
updated: 2026-02-10
---

# [Company] - Competitive Intelligence

## Overview

[Company description, market position]

## Products

| Product | Description | Pricing | Our Alternative |
|---------|-------------|---------|-----------------|
| [Name] | [Desc] | [Price] | [Comparison] |

## Strengths

- [Strength 1 with evidence]
- [Strength 2 with evidence]

## Weaknesses

- [Weakness 1 with evidence]
- [Weakness 2 with evidence]

## Strategy Signals

[Hiring patterns, patent filings, partnerships, pricing changes]

## Threat Assessment

[High/Medium/Low] — [justification]
`,
		"30-decisions/research-tools.md": `---
id: dec_research_001
type: decision
title: Research Tool Stack
status: active
confidence: high
project: research
tags: [tools, research, workflow]
created: 2026-01-10
updated: 2026-01-10
review_after: 2026-07-10
---

# Research Tool Stack

## Decision

Use AgentVault for knowledge storage, Zotero for citation management, Perplexity for initial search.

## Stack

| Purpose | Tool | Why |
|---------|------|-----|
| Knowledge base | AgentVault | Local-first, AI-native |
| Citations | Zotero | Academic standard, BibTeX export |
| Quick search | Perplexity | Fast overview, source links |
| Deep search | Google Scholar | Academic papers |
| Analysis | Python/Jupyter | Data processing |

## Workflow

1. Search with Perplexity for initial landscape
2. Deep dive with Google Scholar
3. Store sources in AgentVault with full metadata
4. Manage citations in Zotero
5. Write briefs and synthesis in AgentVault
6. Export citations when publishing

## Revisit when

- Team grows and needs shared tools
- Publishing workflow changes
- Better AI research tools emerge
`,
		"70-prompts/synthesize-sources.md": `---
id: prompt_research_001
type: prompt
title: Synthesize Sources
status: active
tags: [prompt, synthesis, research]
created: 2026-01-15
updated: 2026-01-15
---

# Synthesize Sources

## Context

You are a research analyst synthesizing multiple sources to answer a specific question.

## Prompt

Given the following sources, answer the question: {{question}}

Sources:
{{sources}}

Provide:
1. A direct answer to the question
2. Key supporting evidence from sources (with citations)
3. Any conflicting findings and how to resolve them
4. Confidence level (High/Medium/Low) with justification
5. What additional information would strengthen the answer

Use only the provided sources. Do not invent facts. Note when sources are outdated or have conflicts of interest.
`,
	},
}
