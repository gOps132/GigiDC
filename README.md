<p align="center">
  <img src="assets/branding/gigidc-logo-square.png" width="128" alt="GigiDC logo" />
</p>

<h1 align="center">GigiDC</h1>

<p align="center">
  <a href="https://gigi-f9937525.mintlify.app/">
    <img alt="Docs" src="https://img.shields.io/badge/docs-live-0f766e?style=for-the-badge&logo=readthedocs&logoColor=white" />
  </a>
  <img alt="Version" src="https://img.shields.io/badge/version-0.1.0-0f766e?style=for-the-badge" />
  <img alt="Node.js" src="https://img.shields.io/badge/node-22.12%2B-2563eb?style=for-the-badge&logo=node.js&logoColor=white" />
  <img alt="TypeScript" src="https://img.shields.io/badge/typescript-5.8-3178C6?style=for-the-badge&logo=typescript&logoColor=white" />
  <img alt="discord.js" src="https://img.shields.io/badge/discord.js-14.19-5865F2?style=for-the-badge&logo=discord&logoColor=white" />
  <img alt="Supabase" src="https://img.shields.io/badge/supabase-2.45-3ECF8E?style=for-the-badge&logo=supabase&logoColor=white" />
</p>

<p align="center">
  Gigi is a personalized Discord bot for CS/IT Archive.
</p>

<p align="center">
  <a href="https://gigi-f9937525.mintlify.app/"><strong>Read the official docs</strong></a>
</p>

Gigi gives CS/IT Archive members one consistent bot experience across DMs and server workflows.

```mermaid
flowchart LR
    User["Discord User"] --> Surface["DM / Mentions / Slash / Buttons"]
    Surface --> Gigi["GigiDC"]
    Gigi --> Router["Intent + Permission Router"]
    Router --> Memory["History, Tasks, User Memory"]
    Router --> Admin["Guild Admin Actions"]
    Router --> Secrets["Sensitive Data Vault"]
    classDef person fill:#fff7ed,stroke:#f97316,color:#7c2d12;
    classDef surface fill:#eff6ff,stroke:#2563eb,color:#1e3a8a;
    classDef core fill:#ecfeff,stroke:#0891b2,color:#164e63;
    classDef state fill:#f0fdf4,stroke:#16a34a,color:#14532d;
    classDef secure fill:#fdf2f8,stroke:#db2777,color:#831843;
    class User person;
    class Surface surface;
    class Gigi,Router core;
    class Memory,Admin state;
    class Secrets secure;
```

## What GigiDC Offers

- Personalized DM-first interaction with mention-based channel chat
- Shared memory across supported workflows
- Permission-aware guild actions
- Sensitive-data disclosure in DM only

```mermaid
flowchart LR
    Ask["DM request"] --> Analyze["Analyze intent"]
    Analyze --> Sensitive{"Sensitive data?"}
    Sensitive -- Yes --> Auth["Check owner + capability"]
    Auth --> Retrieve["Fetch encrypted record"]
    Retrieve --> Reply["Return in DM only"]
    Sensitive -- No --> Execute["Route to retrieval or bounded tools"]
    Execute --> Reply
    classDef request fill:#fff7ed,stroke:#f97316,color:#7c2d12;
    classDef decision fill:#fef3c7,stroke:#d97706,color:#78350f;
    classDef secure fill:#fdf2f8,stroke:#db2777,color:#831843;
    classDef action fill:#eff6ff,stroke:#2563eb,color:#1e3a8a;
    classDef reply fill:#f0fdf4,stroke:#16a34a,color:#14532d;
    class Ask request;
    class Sensitive decision;
    class Auth,Retrieve secure;
    class Analyze,Execute action;
    class Reply reply;
```

## Docs

- [Official docs](https://gigi-f9937525.mintlify.app/)
- [User guide](https://gigi-f9937525.mintlify.app/user-guide)
- [Using Gigi in Discord](https://gigi-f9937525.mintlify.app/discord-usage)
- [Permissions](https://gigi-f9937525.mintlify.app/permissions)
- [Architecture](https://gigi-f9937525.mintlify.app/architecture-v1)
