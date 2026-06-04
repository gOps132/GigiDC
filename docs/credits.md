---
title: Credits
description: External resources, platforms, and tooling used by GigiDC.
---

# External Resources and Credits

## Runtime and Platform Dependencies

- `Go`
  - Use: Primary application runtime for the rebuilt Gigi service
  - Source: https://go.dev/
- `PostgreSQL`
  - Use: Local relational database for runtime state, jobs, external app integrations, and future history storage
  - Source: https://www.postgresql.org/
- `pgvector`
  - Use: PostgreSQL vector extension planned for semantic retrieval
  - Source: https://github.com/pgvector/pgvector
- `Docker`
  - Use: Local and production container build/runtime workflow
  - Source: https://www.docker.com/
- `Docker Compose`
  - Use: Local app/Postgres orchestration and deployable Compose shape
  - Source: https://docs.docker.com/compose/
- `Discord API`
  - Use: Planned Discord gateway, slash commands, interactions, messages, and external app surfaces
  - Source: https://discord.com/developers/docs/
- `discordgo`
  - Use: Go client library for the Discord gateway adapter
  - Source: https://github.com/bwmarrin/discordgo
- `pgx`
  - Use: Go PostgreSQL driver used by the local database-backed Discord permission grants
  - Source: https://github.com/jackc/pgx
- `OpenAI API`
  - Use: Planned LLM response, tool-planning, and embedding provider behind an adapter
  - Source: https://platform.openai.com/docs/
- `Coolify`
  - Use: Planned simple Docker deployment target for the soft-deploy foundation
  - Source: https://coolify.io/docs/
- `Space.h`
  - Use: Local sibling project used as reference for Coolify Docker Compose environment-variable ergonomics and pull request template structure
  - Source: local sibling repository reference
- `GitHub Actions`
  - Use: CI runner for Go validation and Docker Compose smoke tests
  - Source: https://github.com/features/actions
- `actions/checkout`
  - Use: Official GitHub Action used to fetch the repository in CI
  - Source: https://github.com/actions/checkout
- `actions/setup-go`
  - Use: Official GitHub Action used to provision Go in CI
  - Source: https://github.com/actions/setup-go
- `Shields.io`
  - Use: README badge strip for version and stack tags
  - Source: https://shields.io/
- `Mintlify`
  - Use: Documentation site framework and navigation format
  - Source: https://www.mintlify.com/
