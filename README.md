
# Braintrust Go SDK

## Development Setup

This project uses [mise](https://mise.jdx.dev/) to manage development dependencies and environment variables. If you want to do it manually,
install the dependencies listed in `mise.toml`.

### Installing mise

1. Install mise using one of these methods:

   **Using curl (macOS/Linux):**
   ```bash
   curl https://mise.jdx.dev/install.sh | sh
   ```

   **Using Homebrew (macOS):**
   ```bash
   brew install mise
   ```

   **Using Windows (PowerShell):**
   ```powershell
   iwr https://mise.jdx.dev/install.ps1 -useb | iex
   ```

2. Add mise to your shell. This enables mise's tool version management and environment variable handling whenever you switch to this directory.
   ```bash
   # For bash/zsh
   echo 'eval "$(mise activate)"' >> ~/.bashrc  # or ~/.zshrc
   
   # For fish
   echo 'mise activate fish | source' >> ~/.config/fish/config.fish
   ```
2. Run `mise install` to install all required tools
3. Run `mise use` to activate the development environment

### Setup Env Variables

First, `cp env.example .env` and then set your env variables accordingly.

## Build and Test

All of the dev tasks are in our `Makefile`. Run `make help` for more.

