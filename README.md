
# Braintrust Go SDK

## Development Setup

This project uses [mise](https://mise.jdx.dev/) to manage dependencies and configure the dev environment. If you want to do it manually,
install the tools listed in `mise.toml`.


First, [install mise](https://mise.jdx.dev/installing-mise.html) and then [activate it in your shell](https://mise.jdx.dev/getting-started.html#activate-mise). Now we 
can start.

```bash
# Clone the repo.
git clone git@github.com:braintrustdata/braintrust-x-go.git
cd braintrust-x-go

# Install our depdencies.
mise trust
mise install

# Setup your env variables 
cp env.example .env
```

## Build, Test and Run

All of the common dev tasks are in our `Makefile`.

```bash
make build
make test
make help
```