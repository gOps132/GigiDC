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
  - Use: Local relational database for runtime state, jobs, plugins, and future history storage
  - Source: https://www.postgresql.org/
- `pgvector`
  - Use: PostgreSQL vector extension planned for semantic retrieval
  - Source: https://github.com/pgvector/pgvector
- `Docker`
  - Use: Local and production container build/runtime workflow
  - Source: https://www.docker.com/
- `Docker Compose`
  - Use: Local and EC2 app/Postgres orchestration
  - Source: https://docs.docker.com/compose/
- `Discord API`
  - Use: Planned Discord gateway, slash commands, interactions, messages, and voice/plugin surfaces
  - Source: https://discord.com/developers/docs/
- `OpenAI API`
  - Use: Planned LLM response, tool-planning, and embedding provider behind an adapter
  - Source: https://platform.openai.com/docs/
- `AWS`
  - Use: EC2 hosting target
  - Source: https://aws.amazon.com/
- `Nginx`
  - Use: Optional reverse proxy for health endpoints
  - Source: https://nginx.org/
- `GitHub Actions`
  - Use: CI/CD runner for Go validation, Docker Compose smoke tests, and EC2 deploy
  - Source: https://github.com/features/actions
- `actions/checkout`
  - Use: Official GitHub Action used to fetch the repository in CI
  - Source: https://github.com/actions/checkout
- `actions/setup-go`
  - Use: Official GitHub Action used to provision Go in CI
  - Source: https://github.com/actions/setup-go
- `actions/upload-artifact`
  - Use: Official GitHub Action used to store the built Docker image and deploy files between CI and deploy jobs
  - Source: https://github.com/actions/upload-artifact
- `actions/download-artifact`
  - Use: Official GitHub Action used to retrieve Docker image and deploy files in the deploy job
  - Source: https://github.com/actions/download-artifact
- `Shields.io`
  - Use: README badge strip for version and stack tags
  - Source: https://shields.io/
- `Mintlify`
  - Use: Documentation site framework and navigation format
  - Source: https://www.mintlify.com/
- `YouTube Data API`
  - Use: Possible future media plugin search provider; any actual plugin must document API and license/terms requirements before implementation
  - Source: https://developers.google.com/youtube
