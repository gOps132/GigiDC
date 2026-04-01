# Workflow Rules

- Rule: Store reusable insights under `knowledge/<domain>/` using `knowledge.md`, `hypotheses.md`, and `rules.md`.
  Basis: This repo now standardizes the domain-folder knowledge workflow for cross-task reusable learnings.
  Scope: Maintainership and post-task documentation.
  Last confirmed: 2026-04-01

- Rule: Maintain `knowledge/INDEX.md` as the routing table for all active knowledge domains.
  Basis: A central index keeps the domain structure discoverable and prevents orphaned folders.
  Scope: Knowledge organization.
  Last confirmed: 2026-04-01

- Rule: Promote a hypothesis after 5 independent confirmations, and demote a contradicted rule back to `hypotheses.md`.
  Basis: This is the adopted default evidence threshold for the repo knowledge workflow.
  Scope: Knowledge promotion and correction.
  Last confirmed: 2026-04-01

- Rule: Keep project-wide, high-cost, security-relevant, or easy-to-repeat lessons in `AGENTS.md` instead of relying only on `knowledge/`.
  Basis: `AGENTS.md` remains the repo's highest-priority standing guidance for future agents.
  Scope: Durable project memory.
  Last confirmed: 2026-04-01
