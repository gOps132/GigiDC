locals {
  common_tags = merge(
    {
      Project = var.name_prefix
      Managed = "terraform"
      Service = "discord-bot"
    },
    var.tags
  )
}

data "aws_vpc" "selected" {
  id = var.vpc_id
}

data "aws_subnet" "public" {
  id = var.public_subnet_id
}

data "aws_ami" "ubuntu" {
  most_recent = true
  owners      = ["099720109477"]

  filter {
    name   = "name"
    values = ["ubuntu/images/hvm-ssd-gp3/ubuntu-noble-24.04-amd64-server-*"]
  }

  filter {
    name   = "virtualization-type"
    values = ["hvm"]
  }

  filter {
    name   = "root-device-type"
    values = ["ebs"]
  }
}

resource "aws_security_group" "discord_bot" {
  name        = "${var.name_prefix}-sg"
  description = "Security group for the Gigi Discord bot EC2 instance"
  vpc_id      = data.aws_vpc.selected.id

  ingress {
    description = "SSH from admin addresses"
    from_port   = 22
    to_port     = 22
    protocol    = "tcp"
    cidr_blocks = var.admin_cidr_blocks
  }

  ingress {
    description = "Public HTTP endpoint"
    from_port   = 80
    to_port     = 80
    protocol    = "tcp"
    cidr_blocks = var.callback_ingress_cidr_blocks
  }

  dynamic "ingress" {
    for_each = var.enable_https_ingress ? [1] : []
    content {
      description = "Public HTTPS callback endpoint"
      from_port   = 443
      to_port     = 443
      protocol    = "tcp"
      cidr_blocks = var.callback_ingress_cidr_blocks
    }
  }

  egress {
    description = "Allow all outbound traffic"
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = merge(local.common_tags, {
    Name = "${var.name_prefix}-sg"
  })
}

resource "aws_instance" "discord_bot" {
  ami                         = data.aws_ami.ubuntu.id
  instance_type               = var.instance_type
  subnet_id                   = data.aws_subnet.public.id
  key_name                    = var.key_name
  vpc_security_group_ids      = [aws_security_group.discord_bot.id]
  associate_public_ip_address = var.associate_public_ip_address
  user_data_replace_on_change = true
  user_data = templatefile("${path.module}/templates/bootstrap-user-data.sh.tftpl", {
    app_user = var.app_user
    app_dir  = var.app_dir
  })

  root_block_device {
    volume_size           = var.root_volume_size_gb
    volume_type           = "gp3"
    delete_on_termination = true
  }

  tags = merge(local.common_tags, {
    Name = var.name_prefix
  })
}
