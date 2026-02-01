# settle
A small utility to help me organize my installed software and configuration. It's just a simple way to keep my development environment consistent.

## Installation (Debian/Ubuntu)

```bash
# Download latest release, make executable, and install to ~/.local/bin
curl -fL https://github.com/slash3b/settle/releases/latest/download/settle-linux-amd64 -o /tmp/settle \
  && chmod +x /tmp/settle \
  && mv -f /tmp/settle ~/.local/bin/settle
```

Make sure `~/.local/bin` is in your `PATH`.
