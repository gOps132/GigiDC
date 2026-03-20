# Terraform Starter Guide

This repo includes a Terraform starter for the **new Discord bot EC2 only**.

It does **not** try to recreate or import your existing OpenClaw EC2 instance. Instead, it assumes:

- your OpenClaw EC2 already exists
- you know its VPC and security group
- you want Terraform to create the second EC2 for this Discord bot

## What Terraform Will Create

Inside [terraform/](/Users/giancedrick/dev/projects/gigi/terraform):

- one EC2 instance for the Discord bot
- one security group for the Discord bot
- optionally one ingress rule on the existing OpenClaw security group so only the Discord bot EC2 can reach the OpenClaw port

It also bootstraps the new instance with:

- Node.js 22
- git
- Nginx
- the `gigi` service user
- the expected app directories

## What You Need Before Running It

You need these AWS values:

- AWS region of your existing OpenClaw EC2
- VPC ID of the existing OpenClaw EC2
- public subnet ID where the new Discord bot EC2 should live
- your existing EC2 key pair name
- your admin public IP in CIDR form, for example `203.0.113.10/32`
- the security group ID currently attached to OpenClaw, if you want Terraform to add the allow-rule automatically

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
- `openclaw_security_group_id`

Important:

- Use the **same VPC** as the existing OpenClaw EC2
- Use a **public subnet** for the Discord bot EC2 if you want the callback endpoint reachable from the internet
- Keep `openclaw_port = 3000` unless your existing OpenClaw listens on a different port

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

Use the output public IP as the temporary `BOT_PUBLIC_BASE_URL`, for example:

```text
http://YOUR_DISCORD_BOT_PUBLIC_IP
```

Use the existing OpenClaw private address as `CLAWBOT_BASE_URL`, for example:

```text
http://OPENCLAW_PRIVATE_IP:3000
```

## After Terraform Apply

Terraform only creates the machine and baseline networking. You still need to deploy the app.

Follow [docs/deploy-ec2.md](/Users/giancedrick/dev/projects/gigi/docs/deploy-ec2.md):

1. SSH into the new EC2
2. Clone this repo into `/opt/gigi-discord-bot`
3. Create `/etc/gigi-discord-bot/gigi-discord-bot.env`
4. Fill in the bot env vars
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

That destroys only the resources managed by this Terraform folder. It will not delete your existing OpenClaw EC2 instance unless you later add that instance to this Terraform config.
