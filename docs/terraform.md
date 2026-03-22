# Terraform Starter Guide

This repo includes a Terraform starter for a **single Discord bot EC2 instance**.

## What Terraform Will Create

Inside [terraform/](/Users/giancedrick/dev/projects/gigi/terraform):

- one EC2 instance for the Discord bot
- one security group for the Discord bot

It also bootstraps the new instance with:

- Node.js 22
- git
- Nginx
- rsync
- the `gigi` service user
- the expected app directories

## What You Need Before Running It

You need these AWS values:

- AWS region for the new Discord bot EC2
- VPC ID where the bot should run
- public subnet ID where the Discord bot EC2 should live
- your existing EC2 key pair name
- your admin public IP in CIDR form, for example `203.0.113.10/32`

## Install Terraform

Terraform is not bundled in this repo. Install it on your machine first using the official HashiCorp instructions for your OS.

## Fill In Variables

1. Go to [terraform/terraform.tfvars.example](/Users/giancedrick/dev/projects/gigi/terraform/terraform.tfvars.example)
2. Copy it to a new local file named `terraform.tfvars`

Example:

```bash
cd terraform
cp terraform.tfvars.example terraform.tfvars
```

3. Edit `terraform.tfvars` and replace the placeholder values:

- `region`
- `vpc_id`
- `public_subnet_id`
- `key_name`
- `admin_cidr_blocks`

Important:

- Use a **public subnet** for the Discord bot EC2 if you want the health endpoint reachable from the internet
- Keep SSH restricted to your admin CIDR blocks

## Run Terraform

From the repo root:

```bash
cd terraform
terraform init
terraform plan -var-file=terraform.tfvars
terraform apply -var-file=terraform.tfvars
```

What these do:

- `terraform init`: downloads the AWS provider
- `terraform plan`: shows what Terraform is about to create
- `terraform apply`: actually creates it

## What You Get Back

Terraform outputs:

- the new Discord bot instance ID
- the public IP
- the private IP
- the Discord bot security group ID
- a short next-step summary

Use the output public IP as the temporary bot base URL, for example:

```text
http://YOUR_DISCORD_BOT_PUBLIC_IP
```

## After Terraform Apply

Terraform only creates the machine and baseline networking. You still need to deploy the app.

Follow [docs/deploy-ec2.md](/Users/giancedrick/dev/projects/gigi/docs/deploy-ec2.md):

1. SSH into the new EC2
2. Clone this repo into `/opt/gigi-discord-bot`
3. Create `/etc/gigi-discord-bot/gigi-discord-bot.env`
4. Fill in the Discord, Supabase, and OpenAI env vars
5. Enable the systemd unit
6. Enable the Nginx site
7. Start the service

## Safe First Workflow

If you are new to Terraform, use this exact pattern every time:

```bash
cd terraform
terraform plan -var-file=terraform.tfvars
terraform apply -var-file=terraform.tfvars
```

Do **not** edit AWS resources manually first and then guess what Terraform will do. Review the `plan` output before every `apply`.

## If You Want to Remove the Discord Bot EC2 Later

From the same `terraform/` directory:

```bash
terraform destroy -var-file=terraform.tfvars
```

That destroys only the resources managed by this Terraform folder. It will not delete any unrelated AWS resources unless you later add them to this Terraform config.
