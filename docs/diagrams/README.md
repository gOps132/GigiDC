---
title: Diagram Guide
description: Diagram conventions for the GigiDC docs and architecture workflow.
---

# Diagrams

Store durable project diagrams here.

Use this folder for diagrams that explain system shape or change impact, not for throwaway scratch work.

## Recommended Contents

- architecture overviews
- feature or command flows
- retrieval and data-path diagrams
- deployment and release flow diagrams

## File Naming

Prefer explicit names:

- `system-overview.excalidraw`
- `assignment-publish-flow.excalidraw`
- `dm-retrieval-flow.excalidraw`
- `dm-tool-execution-flow.excalidraw`
- `dm-confirmation-and-user-memory-flow.excalidraw`
- `sensitive-data-dm-flow.excalidraw`
- `deploy-release-flow.excalidraw`

Markdown + Mermaid documents are also acceptable when they are easier to diff and maintain in-repo:

- `system-overview.md`
- `dm-retrieval-flow.md`
- `dm-tool-execution-flow.md`
- `dm-confirmation-and-user-memory-flow.md`
- `sensitive-data-dm-flow.md`
- `deploy-release-flow.md`

## Rules

- keep diagrams aligned with the current codebase
- avoid committing secrets, private DM content, or internal-only operational details
- update nearby docs when a diagram becomes the primary explanation for a workflow
- add attribution in `docs/credits.md` when a diagram is generated or heavily derived from an external tool or provider
