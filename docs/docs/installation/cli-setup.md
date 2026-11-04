---
title: uni CLI Setup
---

The `uni setup` command prepares your local workstation to use a uni server from the command line and from supported local AI clients.

Use it after a uni server is running and reachable from your machine.

## What it does

`uni setup` performs these steps:

1. Resolves the uni app URL to use, either from `--url`, from an existing local default, or by prompting you.
2. Authenticates to that uni server. If `UNI_TOKEN` is set, the CLI uses that token. Otherwise, it uses the same browser-based API key flow as `uni login`.
3. Stores the normalized default uni URL in the local uni CLI config.
4. Stores a newly acquired uni API key in the host OS keyring, scoped to that uni URL.
5. Optionally installs uni bootstrap skills into supported local AI clients.

The bootstrap skills let local agents use the `uni` CLI to search for uni-managed skills, install skills, and run local client scans without manually editing client configuration.

:::note
`uni setup` configures the local CLI and local client bootstrap files. It does not deploy the uni server or configure server-side authentication providers.
:::

## Prerequisites

- The `uni` CLI is installed and available on your `PATH`.
- The Obot server URL is reachable from your workstation.
- If authentication is enabled, Obot has at least one configured authentication provider that your user can use.
- Your local OS keyring is available so the CLI can store a newly acquired API key.

If Obot authentication is enabled but no provider is configured yet, finish server-side authentication setup first. See [Enabling Authentication](./enabling-authentication.md) and [Auth Providers](../configuration/auth-providers.md).

## Basic usage

Run setup with your uni app URL:

```bash
uni setup --url https://obot.example.com
```

For a local Docker deployment using the default port:

```bash
uni setup --url http://localhost:8080
```

If authentication is required, the CLI opens a browser to complete login. After login succeeds, setup saves the default URL and asks where to install local bootstrap skills.

## Choosing local client targets

Use `--clients` to choose where bootstrap skills are installed:

| Value | Description | Install location |
|-------|-------------|------------------|
| `agents` | Install into the shared Agent Skills directory used by clients that support `~/.agents`. | `~/.agents/skills` |
| `claude-code` | Install into Claude Code's skills directory. | `~/.claude/skills` |
| `none` | Skip local client bootstrap installation. | Not applicable |

You can install into more than one target:

```bash
uni setup --url https://obot.example.com --clients agents,claude-code
```

To configure only CLI authentication and the default URL:

```bash
uni setup --url https://obot.example.com --clients none
```

When `--clients` is omitted in an interactive terminal, setup prompts you. The prompt always offers `agents`. It offers `claude-code` when Claude Code is detected locally. You can still install Claude Code support explicitly with `--clients claude-code`.

## Non-interactive setup

For scripts or GUI wrappers, pass both `--url` and `--clients` with `--non-interactive`:

```bash
uni setup \
  --url https://obot.example.com \
  --clients agents \
  --non-interactive
```

Non-interactive mode never reads from stdin. It still uses the normal API key flow, so it may open a browser and wait for authentication unless a valid key is already stored.

Use `--yes` to accept defaults and confirmations. If `--clients` is omitted with `--yes`, setup installs the shared `agents` target by default:

```bash
uni setup --url https://obot.example.com --yes
```

If a different default uni URL is already configured, setup refuses to replace it unless you pass `--yes`:

```bash
uni setup --url https://new-obot.example.com --yes
```

## Check setup status

Use `uni setup status` to verify the local configuration:

```bash
uni setup status
```

The command prints:

- CLI version
- Default uni URL
- Whether the stored API key is valid
- Whether setup is complete

For JSON output:

```bash
uni setup status --json
```

## What setup writes locally

`uni setup` writes:

- The default uni URL to the uni CLI config file under the user's XDG config directory.
- An API key to the host OS keyring under the `uni` service, scoped by uni app URL, when setup acquires a new key through the login flow.
- Bootstrap skill files under the selected client skill directories, such as `~/.agents/skills` or `~/.claude/skills`.

## Troubleshooting

### `auth_unavailable`

The uni server did not report exactly one usable configured authentication provider. Configure an auth provider first.

### `server_unreachable`

Check that the URL points to the uni app, that the server is running, and that the CLI can reach it from your workstation.

### Missing `--url` in non-interactive mode

Pass `--url`, or run setup interactively and enter the URL when prompted.

### `--clients is required in non-interactive mode`

Pass `--clients agents`, `--clients claude-code`, `--clients agents,claude-code`, or `--clients none`.

### Existing URL mismatch

If setup reports that another uni URL is already configured, pass `--yes` to replace the stored default URL.
