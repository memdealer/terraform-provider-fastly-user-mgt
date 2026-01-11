# Example: Basic usage of fastly-user-mgt provider
#
# Before running:
#   1. Build and install the provider: task install
#   2. Set your API key: export FASTLY_API_KEY="your-api-key"
#   3. Run: terraform init && terraform plan

terraform {
  required_providers {
    fastly_mgt = {
      source  = "local/fastly/fastly-user-mgt"
      version = "0.1.0"
    }
  }
}

provider "fastly_mgt" {
  # API key from FASTLY_API_KEY environment variable
}

# List all current users
data "fastly_users" "all" {}

output "all_users" {
  description = "All users in the Fastly account"
  value = [
    for u in data.fastly_users.all.users : {
      id    = u.id
      login = u.login
      name  = u.name
      role  = u.role
    }
  ]
}

# List pending invitations
data "fastly_invitations" "pending" {}

output "pending_invitations" {
  description = "All pending invitations"
  value = [
    for i in data.fastly_invitations.pending.invitations : {
      id    = i.id
      email = i.email
      role  = i.role
    }
  ]
}

# Uncomment to invite a new user:
#
# resource "fastly_user" "example" {
#   login = "newuser@example.com"
#   name  = "New User"
#   role  = "engineer"  # Options: user, billing, engineer, superuser
# }
#
# output "new_user" {
#   value = {
#     id            = fastly_user.example.id
#     login         = fastly_user.example.login
#     invitation_id = fastly_user.example.invitation_id
#     user_id       = fastly_user.example.user_id
#   }
# }
