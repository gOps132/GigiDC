output "discord_bot_instance_id" {
  description = "ID of the Discord bot EC2 instance."
  value       = aws_instance.discord_bot.id
}

output "discord_bot_public_ip" {
  description = "Public IPv4 address of the Discord bot EC2 instance."
  value       = aws_instance.discord_bot.public_ip
}

output "discord_bot_private_ip" {
  description = "Private IPv4 address of the Discord bot EC2 instance."
  value       = aws_instance.discord_bot.private_ip
}

output "discord_bot_security_group_id" {
  description = "Security group ID attached to the Discord bot EC2 instance."
  value       = aws_security_group.discord_bot.id
}

output "discord_bot_callback_base_url" {
  description = "Temporary callback base URL if using the instance public IP directly."
  value       = aws_instance.discord_bot.public_ip != "" ? "http://${aws_instance.discord_bot.public_ip}" : null
}

output "next_step_summary" {
  description = "Human-readable next steps after Terraform apply."
  value = join("\n", [
    "1. SSH into the new EC2 using the configured key pair.",
    "2. Clone this repo into ${var.app_dir}.",
    "3. Create /etc/gigi-discord-bot/gigi-discord-bot.env from .env.example.",
    "4. Set CLAWBOT_BASE_URL to the existing OpenClaw private IP or private DNS.",
    "5. Follow docs/deploy-ec2.md to enable the systemd service and Nginx."
  ])
}
