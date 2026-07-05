# luks-vault

LUKS passphrase rotation agent for Linux, integrated with [HashiCorp Vault](https://www.vaultproject.io/) KV v2.

`luks-vault` runs as a background service, reads LUKS key metadata from Vault, and rotates passphrases when the configured TTL expires. It alternates between LUKS key slots `0` and `1`, updates the local key file used by `crypttab`, and stores the new passphrase back in Vault.

## Features

- Automatic LUKS passphrase rotation based on Vault TTL
- Slot `0` / `1` swap strategy for safer key rollover
- HashiCorp Vault KV v2 integration
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

After install:

```shell
sudo cp /etc/luks-vault/config.yaml.example /etc/luks-vault/config.yaml
sudo vi /etc/luks-vault/config.yaml
sudo systemctl enable --now luks-vault
sudo systemctl status luks-vault
```

Release packages are also published as GitHub Actions artifacts and attached to Git tags (`v*`).

## Configuration

Configuration file: `/etc/luks-vault/config.yaml`

```yaml
vault_address: "http://127.0.0.1:8200"
vault_token: "change_me"
vault_mount_path: "luks/hostname"
vault_secret_path: "dev/sda"
vault_module_luks: true
cryptsetup_use_sudo: true
device_path: "/dev/sda"
folder_pass_phrase_path: "/etc/data-at-rest"
pid_file_name: "/run/luks-vault/luks-vault.pid"
log_file_name: "/var/log/luks-vault/agent.log"
```

| Key | Description |
|-----|-------------|
| `vault_address` | Vault API address |
| `vault_token` | Vault token with read/write access to the secret |
| `vault_mount_path` | KV v2 mount path |
| `vault_secret_path` | Secret path under the mount |
| `vault_module_luks` | Enable LUKS rotation logic |
| `cryptsetup_use_sudo` | Run `cryptsetup` via `sudo -n` as non-root user |
| `device_path` | LUKS block device, e.g. `/dev/sda` |
| `folder_pass_phrase_path` | Directory for current and temporary key files |
| `pid_file_name` | PID file path |
| `log_file_name` | Log file path |

Example file: [`packaging/config/config.yaml.example`](packaging/config/config.yaml.example)

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

For legacy root-based deployment, set `cryptsetup_use_sudo: false` and run the binary without `--foreground`.

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
sudo systemctl status luks-vault
sudo tail -f /var/log/luks-vault/agent.log
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
├── agent/              # daemon, rotation workflow, TTL logic
├── config/             # configuration loader
├── module/             # Vault and LUKS integrations
├── packaging/          # systemd, sudoers, nfpm, postinstall scripts
├── main.go
├── Makefile
└── .github/workflows/  # CI pipeline
```

## License

Apache-2.0
