# Native binary deployment

The release package contains:

- `ai-shortlink`: a single Go server binary with embedded HTML, CSS, JavaScript, templates, SQL migrations and static assets.
- `shortlink.env.example`: startup-level configuration. Runtime settings are written by the setup wizard to `DATA_DIR/app-config.json`.

## Start

```bash
chmod +x ./ai-shortlink
cp shortlink.env.example shortlink.env
./ai-shortlink
```

Open `http://SERVER_IP:8080` and complete the setup wizard.
