# External Resources and Credits

This project requires explicit attribution for external resources used in implementation or project workflow.

## Runtime and Platform Dependencies

- `discord.js`
  - Use: Discord gateway client, slash command registration, and interaction handling
  - Source: https://discord.js.org/
- `Supabase`
  - Use: Postgres-backed storage for Discord control-plane state, role policy persistence, and local job references
  - Source: https://supabase.com/
- `OpenAI`
  - Use: DM reasoning and semantic retrieval embeddings in the V1 architecture
  - Source: https://platform.openai.com/docs
- `pgvector`
  - Use: Vector storage and similarity search over embedded Discord messages in Postgres
  - Source: https://github.com/pgvector/pgvector
- `AWS`
  - Use: Planned bot runtime hosting on a dedicated instance
  - Source: https://aws.amazon.com/
- `Terraform`
  - Use: Planned infrastructure provisioning and repeatable environment setup
  - Source: https://developer.hashicorp.com/terraform
- `Nginx`
  - Use: Reverse proxy for the Discord bot health endpoint in the EC2 deployment setup
  - Source: https://nginx.org/
- `NodeSource`
  - Use: Node.js 22 installation source in the EC2 bootstrap workflow
  - Source: https://github.com/nodesource/distributions
- `Canonical Ubuntu Server AMI`
  - Use: Base EC2 image selected by the Terraform starter for the Discord bot host
  - Source: https://cloud-images.ubuntu.com/
- `GitHub Actions`
  - Use: Continuous integration and deployment runner for app validation, Terraform validation, and release deployment to EC2
  - Source: https://github.com/features/actions
- `actions/checkout`
  - Use: Official GitHub Action used to fetch the repository in CI
  - Source: https://github.com/actions/checkout
- `actions/setup-node`
  - Use: Official GitHub Action used to provision Node.js 22 in CI
  - Source: https://github.com/actions/setup-node
- `hashicorp/setup-terraform`
  - Use: Official HashiCorp GitHub Action used to install Terraform in CI
  - Source: https://github.com/hashicorp/setup-terraform
- `actions/upload-artifact`
  - Use: Official GitHub Action used to store the built release bundle between CI and deploy steps
  - Source: https://github.com/actions/upload-artifact
- `actions/download-artifact`
  - Use: Official GitHub Action used to retrieve the built release bundle in the deploy step
  - Source: https://github.com/actions/download-artifact

## Workflow and Planning References

- `claude-skills / discord-bot`
  - Use: Local Codex skill used to guide Discord bot scaffolding workflow
  - Source: https://github.com/inbharatai/claude-skills/tree/main/skills/discord-bot
