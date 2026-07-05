# luks-vault

LUKS passphrase rotation agent for Linux, integrated with [HashiCorp Vault](https://www.vaultproject.io/) KV v2.

`luks-vault` runs as a background service, reads LUKS key metadata from Vault, and rotates passphrases when the configured TTL expires. It alternates between LUKS key slots `0` and `1`, updates the local key file used by `crypttab`, and stores the new passphrase back in Vault.

## Features

- Automatic LUKS passphrase rotation based on Vault TTL
- Slot `0` / `1` swap strategy for safer key rollover
- HashiCorp Vault KV v2 integration with TLS/mTLS and AppRole or token auth
- Rotation state file for crash recovery and step resume
- Config reload via `SIGHUP` / `systemctl reload` without restart
- Exponential backoff when rotation fails
- Runs as non-root user (`luks-vault`) with limited `sudo` access to `cryptsetup`
- Systemd service included
- `.deb` and `.rpm` packages via `make package`
- GitHub Actions CI for test, build, and release artifacts

## How it works

```text
┌─────────────┐     poll TTL     ┌──────────────┐
│ HashiCorp   │ ◄─────────────── │  luks-vault  │
│ Vault KV v2 │ ───────────────► │   (daemon)   │
└─────────────┘   write new key  └──────┬───────┘
                                        │
                        sudo cryptsetup │ write key file
                                        ▼
                                  ┌──────────────┐
                                  │ LUKS device  │
                                  │ /etc/data-   │
                                  │ at-rest/key  │
                                  └──────────────┘
```

When TTL expires, the agent:

1. Generates a new passphrase
2. Adds it to the alternate LUKS key slot
3. Verifies the new passphrase
4. Updates `/etc/data-at-rest/key`
5. Writes metadata to Vault
6. Removes the old LUKS key slot
7. Deletes the temporary key file

Each successful step is persisted to `agent.state_file`. If the process crashes mid-rotation, the next run resumes from the last completed step instead of starting over.

## Requirements

- Linux with LUKS-encrypted block device
- [HashiCorp Vault](https://www.vaultproject.io/) with KV v2 enabled
- `cryptsetup`
- `sudo`
- `systemd`
- LUKS device already initialized with a key in slot `0` or `1`

For LUKS setup reference, see [data-at-rest guide](https://bienkma.github.io/mysharing/database/data-at-rest.html).

## Install from package

Build packages locally:

```shell
make package VERSION=1.0.0
```

Install on Debian/Ubuntu:

```shell
sudo dpkg -i dist/luks-vault_1.0.0-1_amd64.deb
sudo apt-get install -f
```

Install on RHEL/CentOS/Fedora:

```shell
sudo rpm -ivh dist/luks-vault-1.0.0-1.x86_64.rpm
```

The package installs:

| Path | Description |
|------|-------------|
| `/usr/local/bin/luks-vault` | Agent binary |
| `/lib/systemd/system/luks-vault.service` | Systemd unit |
| `/etc/sudoers.d/luks-vault` | Limited sudo rules for `cryptsetup` |
| `/etc/luks-vault/config.yaml.example` | Example configuration |
| `/var/lib/luks-vault/` | Rotation state directory |
| `/var/log/luks-vault/` | Log directory |

After install:

```shell
sudo vi /etc/luks-vault/config.yaml
sudo install -o luks-vault -g luks-vault -m 600 /path/to/approle-secret-id /etc/luks-vault/approle-secret-id
sudo cp vault-ca.pem vault-client.pem vault-client-key.pem /etc/luks-vault/
sudo systemctl enable --now luks-vault
sudo systemctl status luks-vault
```

On first install, the package creates `/etc/luks-vault/config.yaml` from the example if it does not exist yet.

Release packages are also published as GitHub Actions artifacts and attached to Git tags (`v*`).

## Configuration

Configuration file: `/etc/luks-vault/config.yaml`

```yaml
agent:
  poll_interval: 10s
  pid_file: /run/luks-vault/luks-vault.pid
  log_file: /var/log/luks-vault/agent.log
  state_file: /var/lib/luks-vault/rotation.state

vault:
  address: https://vault.example.com:8200
  auth_method: approle
  approle:
    role_id: change-me-role-id
    secret_id_file: /etc/luks-vault/approle-secret-id
  kv2_mount: luks/hostname
  secret_path: dev/sda
  tls:
    ca_cert: /etc/luks-vault/vault-ca.pem
    client_cert: /etc/luks-vault/vault-client.pem
    client_key: /etc/luks-vault/vault-client-key.pem
    skip_verify: false

luks:
  enabled: true
  device: /dev/sda
  passphrase_dir: /etc/data-at-rest
  cryptsetup_use_sudo: true
```

### Agent

| Key | Description |
|-----|-------------|
| `agent.poll_interval` | How often the agent polls Vault |
| `agent.pid_file` | PID file for legacy daemon mode |
| `agent.log_file` | Log file path |
| `agent.state_file` | Rotation state file for crash recovery |

### Vault

| Key | Description |
|-----|-------------|
| `vault.address` | Vault API URL, use `https://` in production |
| `vault.auth_method` | `token` or `approle` |
| `vault.token` | Static token when `auth_method=token` |
| `vault.approle.role_id` | AppRole role ID |
| `vault.approle.secret_id` | AppRole secret ID |
| `vault.approle.secret_id_file` | File containing AppRole secret ID |
| `vault.kv2_mount` | KV v2 mount path |
| `vault.secret_path` | Secret path under the mount |
| `vault.tls.ca_cert` | CA certificate for Vault TLS |
| `vault.tls.client_cert` | Client certificate for mTLS |
| `vault.tls.client_key` | Client private key for mTLS |
| `vault.tls.skip_verify` | Skip TLS verification, default `false` |

### LUKS

| Key | Description |
|-----|-------------|
| `luks.enabled` | Enable LUKS rotation |
| `luks.device` | LUKS block device, e.g. `/dev/sda` |
| `luks.passphrase_dir` | Directory for current and temporary key files |
| `luks.cryptsetup_use_sudo` | Run `cryptsetup` via `sudo -n` |

Example files:

- [`packaging/config/config.yaml.example`](packaging/config/config.yaml.example) for production with AppRole + mTLS
- [`config/config.yaml.example`](config/config.yaml.example) for local development with token auth

> **Note:** Configuration uses nested YAML keys such as `vault.address`, `luks.device`, and `agent.log_file`. Older flat keys like `vault_address` or `device_path` are no longer supported.

Reload config without restart:

```shell
sudo systemctl reload luks-vault
# or
sudo kill -HUP $(pidof luks-vault)
```

Minimal token-auth example for development:

```yaml
vault:
  address: https://127.0.0.1:8200
  auth_method: token
  token: change_me
  kv2_mount: luks/hostname
  secret_path: dev/sda
  tls:
    ca_cert: /etc/luks-vault/vault-ca.pem
    skip_verify: false
```

## Vault secret format

Create a KV v2 secret with these fields:

```json
{
  "created": "2026-07-05T12:00:00+07:00",
  "key": "current_passphrase_on_luks",
  "slot": "1",
  "ttl": "30m"
}
```

| Field | Description |
|-------|-------------|
| `created` | RFC3339 timestamp of the current key |
| `key` | Current LUKS passphrase |
| `slot` | Active LUKS key slot (`0` or `1`) |
| `ttl` | Rotation interval (`ns`, `us`, `ms`, `s`, `m`, `h`) |

The `slot` value must match the active LUKS key slot on the device.

## Prepare LUKS and boot unlock

Create the local key file used by the agent and `crypttab`:

```shell
sudo mkdir -p /etc/data-at-rest
echo "current_passphrase_on_luks" | sudo tee /etc/data-at-rest/key
sudo chown root:luks-vault /etc/data-at-rest
sudo chmod 750 /etc/data-at-rest
sudo chown root:luks-vault /etc/data-at-rest/key
sudo chmod 640 /etc/data-at-rest/key
```

Configure `/etc/crypttab`:

```text
# <target name>  <source device>              <key file>              <options>
data01           /dev/mapper/data-data01      /etc/data-at-rest/key   luks
```

If you initialize an extra LUKS slot manually:

```shell
sudo cryptsetup -v -q luksAddKey /dev/sda -d /path/to/init-key -S 1
```

## Security model

The packaged service runs as user `luks-vault`, not root.

- Vault access and key file updates run as `luks-vault`
- `cryptsetup` runs through `sudo` with a restricted rule set in `/etc/sudoers.d/luks-vault`
- Only these commands are allowed:
  - `luksAddKey`
  - `luksKillSlot`
  - `open --test-passphrase`

Verify sudo access:

```shell
sudo -u luks-vault sudo -n /usr/sbin/cryptsetup --version
```

For legacy root-based deployment, set `luks.cryptsetup_use_sudo: false` and run the binary without `--foreground`.

## Build from source

Requires Go 1.26+.

```shell
git clone https://github.com/bienkma/luks-vault.git
cd luks-vault
make test
make build
```

Cross-compile for Linux:

```shell
make build GOOS=linux GOARCH=amd64
make build GOOS=linux GOARCH=arm64
```

Build inside Docker:

```shell
make docker-build
```

Build install packages:

```shell
make package VERSION=1.0.0
make package VERSION=1.0.0 GOARCH=arm64
```

Useful Makefile targets:

```shell
make help
make test
make vet
make deb
make rpm
make clean
```

## Run manually

Foreground mode for systemd:

```shell
/usr/local/bin/luks-vault --foreground
```

Legacy daemon mode:

```shell
/usr/local/bin/luks-vault
/usr/local/bin/luks-vault -s quit
/usr/local/bin/luks-vault -s reload
```

## Operations

```shell
sudo systemctl start luks-vault
sudo systemctl stop luks-vault
sudo systemctl reload luks-vault
sudo systemctl status luks-vault
sudo tail -f /var/log/luks-vault/agent.log
```

When running with `--foreground`, logs are written to `agent.log_file` from the config (default: `/var/log/luks-vault/agent.log`). You can also inspect systemd status with:

```shell
sudo journalctl -u luks-vault -f
```

## Development

Run tests:

```shell
go test ./...
go vet ./...
```

CI runs on push and pull requests:

- `go mod tidy` verification
- unit tests and `go vet`
- linux `amd64` / `arm64` builds
- `.deb` / `.rpm` package build
- GitHub Release assets on tags `v*`

## Project layout

```text
.
├── agent/              # daemon, rotation workflow, TTL, state, backoff
├── config/             # configuration loader and examples
├── module/             # Vault and LUKS integrations
├── packaging/          # systemd, sudoers, nfpm, postinstall scripts
├── main.go
├── Makefile
└── .github/workflows/  # CI pipeline
```

## License

Apache-2.0
