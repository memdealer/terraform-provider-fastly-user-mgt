- Yes, I udnerstand that I could have submitted PR to original fastly provider.
- No i don't think they would have approved this approach
- I kinda wait till they fix it themselves, as they destroyed part that worked but did not think about how to fix it later.

# Terraform Provider for Fastly User Management

A focused Terraform provider for managing Fastly users and invitations. This provider uses the [Fastly Invitations API](https://www.fastly.com/documentation/reference/api/account/invitations/) to handle user creation since the direct user creation endpoint was deprecated in April 2025.

## Background

This provider is built on top of the official [fastly/terraform-provider-fastly](https://github.com/fastly/terraform-provider-fastly). 

Fastly deprecated the `POST /user` API endpoint in April 2025, breaking user creation in the official Terraform provider. Since the official provider hasn't been updated to use the new Invitations API workflow, and I still need to manage users via Terraform, I created this standalone provider focused solely on user management.

This provider:
- Uses the new [Invitations API](https://www.fastly.com/documentation/reference/api/account/invitations/) instead of the deprecated user creation endpoint
- Handles the invitation → user acceptance workflow seamlessly
- Is stripped down to only user management functionality

## Features

- **`fastly_user` resource** - Invite and manage Fastly users
- **`fastly_users` data source** - List all users in your Fastly account
- **`fastly_invitations` data source** - List all pending invitations

## Requirements

- [Terraform](https://www.terraform.io/downloads.html) >= 1.0
- [Go](https://golang.org/doc/install) >= 1.21 (to build the provider)
- A Fastly API key with appropriate permissions

## Installation

### From Source

```bash
git clone https://github.com/fastly/terraform-provider-fastly-user-mgt.git
cd terraform-provider-fastly-user-mgt
go build -o terraform-provider-fastly-user-mgt
```

Then move the binary to your Terraform plugins directory or use [development overrides](https://developer.hashicorp.com/terraform/cli/config/config-file#development-overrides-for-provider-developers).

## Configuration

```hcl
terraform {
  required_providers {
    fastly_mgt = {
      source = "fastly/fastly-user-mgt"
    }
  }
}

provider "fastly_mgt" {
  api_key = var.fastly_api_key  # Or set FASTLY_API_KEY environment variable
}
```

### Provider Arguments

| Argument | Description | Default |
|----------|-------------|---------|
| `api_key` | Fastly API key. Can also be set via `FASTLY_API_KEY` env var. | - |
| `base_url` | Fastly API URL. Can also be set via `FASTLY_API_URL` env var. | `https://api.fastly.com` |
| `force_http2` | Force HTTP/2 connections to the API. | `false` |

## Usage Examples

### Invite a New User

```hcl
resource "fastly_user" "engineer" {
  login = "engineer@example.com"
  name  = "New Engineer"
  role  = "engineer"
}
```

When you apply this configuration:
1. An invitation is sent to `engineer@example.com`
2. The resource tracks the pending invitation
3. Once the user accepts, the resource automatically transitions to managing the actual user

### List All Users

```hcl
data "fastly_users" "all" {}

output "users" {
  value = data.fastly_users.all.users
}
```

### List Pending Invitations

```hcl
data "fastly_invitations" "pending" {}

output "pending_invitations" {
  value = data.fastly_invitations.pending.invitations
}
```

### Complete Example

```hcl
terraform {
  required_providers {
    fastly_mgt = {
      source = "fastly/fastly-user-mgt"
    }
  }
}

provider "fastly_mgt" {
  # Uses FASTLY_API_KEY environment variable
}

# Invite team members
resource "fastly_user" "dev_team" {
  for_each = {
    alice = { email = "alice@example.com", role = "engineer" }
    bob   = { email = "bob@example.com", role = "user" }
    carol = { email = "carol@example.com", role = "superuser" }
  }

  login = each.value.email
  name  = each.key
  role  = each.value.role
}

# List all current users
data "fastly_users" "current" {}

# List pending invitations
data "fastly_invitations" "pending" {}

output "current_users" {
  value = [for u in data.fastly_users.current.users : u.login]
}

output "pending_invitations" {
  value = [for i in data.fastly_invitations.pending.invitations : i.email]
}
```

## Resource: fastly_user

### Arguments

| Argument | Type | Required | Description |
|----------|------|----------|-------------|
| `login` | string | Yes | The email address (login) of the user |
| `name` | string | Yes | The display name of the user |
| `role` | string | No | User role: `user` (default), `billing`, `engineer`, or `superuser` |

### Attributes

| Attribute | Description |
|-----------|-------------|
| `id` | The resource ID |
| `user_id` | The Fastly user ID (set once invitation is accepted) |
| `invitation_id` | The invitation ID (set while invitation is pending) |

### Import

Import an existing user by their user ID:

```bash
terraform import fastly_user.example xxxxxxxxxxxxxxxxxxxx
```

## Data Source: fastly_users

Lists all users in the current Fastly account.

### Attributes

| Attribute | Description |
|-----------|-------------|
| `users` | List of user objects |
| `users.id` | User ID |
| `users.login` | User email/login |
| `users.name` | User display name |
| `users.role` | User role |
| `users.customer_id` | Customer ID |
| `users.locked` | Whether the account is locked |
| `users.two_factor_auth_enabled` | Whether 2FA is enabled |
| `users.limit_services` | Whether user has limited service access |

## Data Source: fastly_invitations

Lists all pending invitations in the current Fastly account.

### Attributes

| Attribute | Description |
|-----------|-------------|
| `invitations` | List of invitation objects |
| `invitations.id` | Invitation ID |
| `invitations.email` | Invitee email address |
| `invitations.role` | Assigned role |
| `invitations.status_code` | Invitation status |

## How the Invitation Workflow Works

Since Fastly deprecated direct user creation, this provider uses the Invitations API:

1. **Create** - When you create a `fastly_user` resource, an invitation is sent to the email address
2. **Pending** - The resource stores the `invitation_id` and tracks the pending invitation
3. **Accepted** - When the user accepts the invitation, the next `terraform plan/apply` detects this and updates the state to use the actual `user_id`
4. **Manage** - Once accepted, you can update the user's `name` and `role` like any normal resource
5. **Delete** - Deletes either the pending invitation or the actual user

## License

This project is licensed under the Mozilla Public License 2.0 - see the [LICENSE](LICENSE) file for details.
