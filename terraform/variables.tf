variable "region" {
  description = "AWS region for the Discord bot EC2 instance."
  type        = string
}

variable "name_prefix" {
  description = "Name prefix used for AWS tags and resource names."
  type        = string
  default     = "gigi-discord-bot"
}

variable "vpc_id" {
  description = "ID of the existing VPC where the Discord bot EC2 instance will run."
  type        = string
}

variable "public_subnet_id" {
  description = "ID of the public subnet where the Discord bot EC2 instance will be launched."
  type        = string
}

variable "key_name" {
  description = "Existing EC2 key pair name for SSH access."
  type        = string
}

variable "admin_cidr_blocks" {
  description = "CIDR blocks allowed to SSH into the Discord bot EC2 instance."
  type        = list(string)
}

variable "callback_ingress_cidr_blocks" {
  description = "CIDR blocks allowed to reach the public HTTP endpoint."
  type        = list(string)
  default     = ["0.0.0.0/0"]
}

variable "instance_type" {
  description = "EC2 instance type for the Discord bot."
  type        = string
  default     = "t3.small"
}

variable "associate_public_ip_address" {
  description = "Whether to assign a public IP directly to the Discord bot EC2 instance."
  type        = bool
  default     = true
}

variable "root_volume_size_gb" {
  description = "Root EBS volume size in GiB."
  type        = number
  default     = 20
}

variable "app_user" {
  description = "System user created on the Discord bot EC2 instance."
  type        = string
  default     = "gigi"
}

variable "app_dir" {
  description = "Application directory created on the Discord bot EC2 instance."
  type        = string
  default     = "/opt/gigi-discord-bot"
}

variable "enable_https_ingress" {
  description = "Whether to also open port 443 for future TLS termination."
  type        = bool
  default     = false
}

variable "tags" {
  description = "Additional AWS tags applied to created resources."
  type        = map(string)
  default     = {}
}
