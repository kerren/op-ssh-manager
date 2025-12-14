# 1Password Item Schema

This document describes the expected schema for 1Password items used by op-ssh-manager.

## Server Items

Server items represent SSH hosts to be managed. They should be tagged with `op-ssh-manager/server`.

### Required Fields

| Field | Type | Description |
|-------|------|-------------|
| `alias` | String | SSH Host alias (unique identifier). Must be alphanumeric with hyphens. |
| `hostname` | String | IP address or DNS hostname |

### Optional Fields

| Field | Type | Description |
|-------|------|-------------|
| `port` | Number | SSH port (default: 22) |
| `default_user` | String | Default SSH username |
| `keys` | Multiline String | Key references (one per line) |
| `jump_alias` | String | Alias of jump host (must exist as another server) |
| `users` | Multiline String | Per-user access mappings |
| `environment` | String | Environment label (e.g., "production", "staging") |
| `owner_team` | String | Team ownership label |

### Key Reference Format

Keys can be referenced in several formats:

```
# Full format with vault
vault=Development;item=my-ssh-key

# Item only (searches all accessible vaults)
item=my-ssh-key

# Simple format
my-ssh-key

# Secret reference format
op://Development/my-ssh-key
```

### Per-User Access Format

Per-user mappings allow different SSH usernames and keys per 1Password user:

```
email=alice@example.com;user=alice;keys=alice-key
email=bob@example.com;user=bob;keys=bob-key-1,bob-key-2
```

## SSH Key Items

SSH key items should have the category "SSH_KEY" in 1Password. They are automatically discovered when referenced by server items.

### Required Fields

| Field | Type | Description |
|-------|------|-------------|
| `public key` | String | The full SSH public key |

## Jump Host Items

Jump hosts are server items tagged with `op-ssh-manager/jump` in addition to `op-ssh-manager/server`.

## Example Server Item

```
Title: prod-db
Category: Server (or Login)
Tags: op-ssh-manager/server

Fields:
  alias: prod-db
  hostname: 10.0.1.100
  port: 22
  default_user: ubuntu
  keys: vault=Production;item=production-key
  jump_alias: bastion
  environment: production
  owner_team: platform
```

## Example Jump Host Item

```
Title: bastion
Category: Server
Tags: op-ssh-manager/server, op-ssh-manager/jump

Fields:
  alias: bastion
  hostname: bastion.example.com
  port: 22
  default_user: ubuntu
  keys: vault=Production;item=bastion-key
  environment: production
```
