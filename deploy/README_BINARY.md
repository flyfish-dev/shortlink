# Native binary deployment

The release package contains:

- `ai-shortlink`: a single Go server binary with embedded HTML, CSS, JavaScript, templates, SQL migrations and static assets.
- `shortlink.env.example`: startup-level configuration. Runtime settings are written by the setup wizard to `DATA_DIR/app-config.json`.
- `ai-shortlink.service`: a systemd unit file that uses `/opt/shortlink` as the production directory.

## Production directory

Use the following production directory consistently:

```text
/opt/shortlink
```

## Start manually

```bash
sudo mkdir -p /opt/shortlink
sudo cp ai-shortlink shortlink.env.example ai-shortlink.service /opt/shortlink/
cd /opt/shortlink
sudo cp shortlink.env.example shortlink.env
sudo chmod +x ./ai-shortlink
./ai-shortlink
```

Open `http://SERVER_IP:8080` and complete the setup wizard.

## systemd

```bash
sudo cp /opt/shortlink/ai-shortlink.service /etc/systemd/system/ai-shortlink.service
sudo systemctl daemon-reload
sudo systemctl enable --now ai-shortlink
```

## External config

```bash
SHORTLINK_CONFIG=/opt/shortlink/shortlink.env /opt/shortlink/ai-shortlink
```

Environment variables override values loaded from `shortlink.env`.
