# op-ssh-manager

A cross-platform CLI that manages SSH hosts and key selection via 1Password.

## Features

- **Discover managed servers and SSH keys from 1Password** using tags
- **Generate/update SSH client configuration** without deleting or breaking unmanaged user entries
- **Select the correct 1Password SSH key per host** using the 1Password SSH agent
- **Support jump hosts** (ProxyJump) automatically
- **Apply and rotate authorized_keys** on servers
- **Produce auditable reports** (CSV/HTML)
- **Optional background sync** service

## Installation

### From Source

```bash
go install github.com/kerren/op-ssh-manager/cmd/op-ssh-manager@latest
```

### From Release

Download the latest release from the [releases page](https://github.com/kerren/op-ssh-manager/releases).

## Prerequisites

- [1Password CLI](https://developer.1password.com/docs/cli/) (`op`) installed and configured
- 1Password desktop app with SSH agent enabled (recommended)
- Go 1.22+ (for building from source)

## Quick Start

1. **Create server items in 1Password** with the tag `op-ssh-manager/server`:

   ```
   Title: prod-db
   Tags: op-ssh-manager/server

   Fields:
     alias: prod-db
     hostname: 10.0.1.100
     default_user: ubuntu
     keys: vault=Production;item=my-ssh-key
   ```

2. **Run the doctor command** to check your environment:

   ```bash
   op-ssh-manager doctor
   ```

3. **Sync your SSH configuration**:

   ```bash
   op-ssh-manager sync
   ```

4. **Connect to your server**:

   ```bash
   ssh prod-db
   # or
   op-ssh-manager ssh prod-db
   ```

## Commands

### `op-ssh-manager sync`

Synchronizes SSH configuration from 1Password:

```bash
op-ssh-manager sync           # Full sync
op-ssh-manager sync --dry-run # Preview changes
```

### `op-ssh-manager doctor`

Diagnoses environment issues:

```bash
op-ssh-manager doctor
```

### `op-ssh-manager ssh <alias>`

Connects to a server (with optional auto-sync):

```bash
op-ssh-manager ssh prod-db
op-ssh-manager ssh prod-db --no-sync
op-ssh-manager ssh prod-db -- -L 5432:localhost:5432
```

### `op-ssh-manager authorized-keys apply`

Applies public keys to remote servers:

```bash
op-ssh-manager authorized-keys apply --server prod-db
op-ssh-manager authorized-keys apply --all --dry-run
```

### `op-ssh-manager report`

Generates inventory reports:

```bash
op-ssh-manager report --format csv --output inventory.csv
op-ssh-manager report --format html --output inventory.html
```

### `op-ssh-manager daemon`

Manages background sync:

```bash
op-ssh-manager daemon run      # Run in foreground
op-ssh-manager daemon install  # Install as system service
op-ssh-manager daemon status   # Check status
```

## Configuration

Configuration file: `~/.config/op-ssh-manager/config.yaml`

See [docs/config.example.yaml](docs/config.example.yaml) for all options.

## 1Password Item Schema

### Server Items

Tag your server items with `op-ssh-manager/server` and include these fields:

| Field | Required | Description |
|-------|----------|-------------|
| `alias` | Yes | SSH Host alias |
| `hostname` | Yes | IP or DNS name |
| `port` | No | SSH port (default: 22) |
| `default_user` | No | SSH username |
| `keys` | No | Key references |
| `jump_alias` | No | Jump host alias |

### Key References

Reference SSH keys in 1Password:

```
vault=Development;item=my-key  # Full format
item=my-key                     # Search all vaults
my-key                          # Simple format
op://Vault/my-key               # Secret reference
```

See [docs/schema.md](docs/schema.md) for complete schema documentation.

## How It Works

1. **Discovery**: Lists items tagged `op-ssh-manager/server` from 1Password
2. **Key Resolution**: Fetches SSH key items referenced by servers
3. **Public Key Cache**: Exports public keys to `~/.ssh/op-ssh-manager/keys/`
4. **Config Generation**: Creates `~/.ssh/op-ssh-manager.conf`
5. **Include Setup**: Adds `Include` directive to `~/.ssh/config`
6. **Agent Config**: Updates 1Password agent.toml for key selection

### SSH Config Safety

op-ssh-manager uses the `Include` directive to safely manage configurations:

```
# ~/.ssh/config
# Managed by op-ssh-manager - do not edit this line
Include ~/.ssh/op-ssh-manager.conf

# Your other config below...
Host github.com
    User git
```

This ensures your existing SSH configuration is never modified or lost.

## Security

- **No private keys stored locally** - relies on 1Password SSH agent
- **Atomic writes** for all config files
- **Backups** created before modifications
- **Managed sections** clearly marked in all files

## Platform Support

| Platform | Sync | Daemon | Notes |
|----------|------|--------|-------|
| Linux | ✅ | ✅ systemd | Full support |
| macOS | ✅ | ✅ launchd | Full support |
| Windows | ✅ | ⚠️ Manual | SSH agent has limitations |

## Development

```bash
# Build
make build

# Test
make test

# Lint
make lint

# Release snapshot
make release-snapshot
```

## AI Usage

This project was developed with AI assistance as an experiment in using AI for real-world software development.

## License

MIT License - see [LICENSE](LICENSE) for details.
