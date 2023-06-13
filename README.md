# luks-vault

## Getting started

Luks-vault is a simple project to integrate LUKS with Vault hashicorp. It will handle rotation passPhrase key of LUKS
and write to secret vault.

## Prepare

- vault server
- Agent install to node which are using LUKS to encrypt device
- Support Unix OS only

## How to build

```shell
 docker run --rm \
            -v /Users/bienkma/go/src/git.ghtk.vn/devops/luks-vault:/go/src/git.ghtk.vn/devops/luks-vault \
            -w /go/src/git.ghtk.vn/devops/luks-vault \
            golang:1.18 sh -c \
            'GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -mod=mod -a -installsuffix cgo -o luks-vault main.go'
```

## Installation

- Install agent

```shell
cp luks-vault /usr/local/bin/luks-vault
chmod 755 /usr/local/bin/luks-vault
mkdir -p /var/log/luks-vault
mkdir -p /etc/luks-vault
touch /etc/luks-vault/config.yaml
```

- Make config file for agent

```shell
vault_address: "http://10.110.32.85:8200"
vault_token: "change_me"
vault_mount_path: "luks/10.110.96.70"
vault_secret_path: "dev/sda"
vault_module_luks: true
device_path: "/dev/sda" # LUKS device
folder_pass_phrase_path: "/etc/data-at-rest"
pid_file_name: "/run/luks-vault.pid"
log_file_name: "/var/log/luks-vault/agent.log"
```

- Make /lib/systemd/system/luks-vault.service file

```shell
[Unit]
Description=LUKS Vault agent
After=network.target auditd.service
Wants=network.target

[Service]
Type=forking
User=root
Group=root
ExecStart=/usr/local/bin/luks-vault
ExecStop=/bin/kill -3 $MAINPID
PIDFile=/run/luks-vault.pid
ExecStartPost=/bin/sleep 1
WorkingDirectory=/usr/local/bin
RestartSec=15
KillMode=none
PrivateTmp=false
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
Alias=luks-vault.service
```

- Make sure systemd has ready load file above
```shell
systemctl daemon-reload
systemctl status luks-vault
```

## create secret in vault with field

```json
{
  "created": "2023-06-06T11:46:37.079847+07:00",
  "key": "passPharse_change_me_now",
  "slot": "0",
  "ttl": "30m"
}
```
- Note: ttl valid time units are “ns”, “us” (or “µs”), “ms”, “s”, “m”, “h”.

## Test and Deploy

```shell
systemctl start luks-vault
tail -f /var/log/luks-vault/agent.log
systemctl status luks-vault
systemctl stop luks-vault
```

## License

For open source projects, say how it is licensed.