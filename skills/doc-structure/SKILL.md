---
name: doc-structure
description: Extract and modify markdown document heading structure. Use when the user asks to analyze document hierarchy, reorganize headings, fix TOC structure, or mentions "heading structure", "readability", "document outline".
---

# Document Structure Tool

A utility skill for extracting and modifying markdown heading structure. **The LLM makes all semantic decisions** — this tool only provides raw structure data and executes changes.

## Tool Capabilities

| Command | Purpose | LLM Role |
|---------|---------|----------|
| `extract` | Output raw heading structure | Analyze and decide |
| `apply` | Execute heading level changes | Plan the changes |
| `demote` | Batch demote headings (H2→H3) | N/A |
| `promote` | Batch promote headings (H3→H2) | N/A |

## Workflow

### Step 1: Extract Structure

```bash
node restructure-toc.mjs <directory> extract
```

Output: JSON array with each file's heading structure:
```json
[
  {
    "file": "example.md",
    "headings": [
      { "level": 1, "text": "Document Title", "line": 2 },
      { "level": 2, "text": "Section A", "line": 5 },
      { "level": 2, "text": "Section B", "line": 20 }
    ]
  }
]
```

### Step 2: LLM Analysis

Based on the extracted structure, the LLM should analyze:

1. **Document type**: tutorial, reference, concept, troubleshooting?
2. **Grouping opportunities**: Should some H2s be H3s under a parent?
3. **Nesting depth**: Is the hierarchy appropriate?
4. **Title clarity**: Are section titles descriptive?

**Decision framework:**

| Situation | Recommendation |
|-----------|----------------|
| 5+ sequential steps at H2 | Consider grouping under phase headers |
| Independent items at H2 | Keep flat for scanability |
| H3 content without H2 parent | Add category header |
| Deep nesting (H5+) | Flatten to improve navigation |

### Step 3: Plan Changes

The LLM outputs a change plan in JSON format:
```json
{
  "changes": [
    { "file": "deployment-guide.md", "line": 15, "fromLevel": 2, "toLevel": 3 },
    { "file": "deployment-guide.md", "line": 25, "fromLevel": 2, "toLevel": 3 }
  ]
}
```

### Step 4: Apply Changes

```bash
node restructure-toc.mjs <directory> apply --plan plan.json
```

Or use the batch commands for simple operations:
```bash
# Promote all headings (H3→H2, H4→H3, etc.)
node restructure-toc.mjs <directory> promote --dry-run

# Demote all headings (H2→H3, H3→H4, etc.)
node restructure-toc.mjs <directory> demote --dry-run
```

## Analysis Guidelines (for LLM)

### Document Type Patterns

**Tutorial** (sequential steps):
```markdown
# Getting Started

## Prerequisites
## Step 1: Install
## Step 2: Configure
## Step 3: Verify
```
→ Keep flat or group under phases if 8+ steps

**Reference** (independent items):
```markdown
# API Reference

## Authentication
## Endpoints
## Error Codes
```
→ Flat structure is appropriate

**Concept** (hierarchical knowledge):
```markdown
# Architecture

## Components
### Gateway
### Registry
### Hosting
## Security
### Authentication
### Authorization
```
→ Nesting reflects relationships

### When to Nest

**Nest (H2→H3)** when items are:
- Sequential steps in a process
- Variations of the same concept
- Children of a clear parent topic

**Keep flat** when items are:
- Independent topics (read in any order)
- Substantial content (not just a paragraph)
- A list for quick scanning

### Example Analysis

**Before:**
```
H1: Deployment Guide
H2: Overview
H2: Step 1: Install Dependencies
H2: Step 2: Configure Environment
H2: Step 3: Build Application
H2: Step 4: Run Migrations
H2: Monitoring
H2: View Logs
H2: Health Checks
```

**LLM Analysis:**
- Document type: Tutorial
- Steps 1-4 are sequential, should nest under "Setup Steps"
- "Monitoring" section has sub-items that could nest

**Recommendation:**
```
H1: Deployment Guide
H2: Overview
H2: Setup Steps
  H3: Step 1: Install Dependencies
  H3: Step 2: Configure Environment
  H3: Step 3: Build Application
  H3: Step 4: Run Migrations
H2: Monitoring
  H3: View Logs
  H3: Health Checks
```

## Checklist

Before completing:
- [ ] Extracted structure using the tool
- [ ] Analyzed document types and patterns
- [ ] Identified grouping opportunities
- [ ] Planned specific heading changes
- [ ] Applied changes
- [ ] Re-extracted to verify
