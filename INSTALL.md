# Installation

## macOS and Linux

Install the latest release into `~/.local/bin`:

```bash
curl -fsSL https://raw.githubusercontent.com/anatoly-tenenev/spec-cli/main/scripts/install.sh | sh
```

Install a specific release:

```bash
curl -fsSL https://raw.githubusercontent.com/anatoly-tenenev/spec-cli/main/scripts/install.sh | sh -s -- v0.2.0
```

The script:

- detects `darwin/linux` and `amd64/arm64`
- downloads the matching archive from GitHub Releases
- verifies `sha256` against `checksums.txt`
- installs `spec-cli` into `~/.local/bin` by default
- runs `spec-cli version` as a post-install check

Use another install directory if needed:

```bash
curl -fsSL https://raw.githubusercontent.com/anatoly-tenenev/spec-cli/main/scripts/install.sh | \
  SPEC_CLI_INSTALL_DIR=/usr/local/bin sh
```
