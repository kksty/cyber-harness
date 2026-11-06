---
type: Tool Playbook
title: arsenal
description: Security tool package manager. Search, install, update, remove CLI tools from chainreactors, projectdiscovery, and any GitHub repo.
tags: [runtime, package-manager]
status: stable
generated: { by: process:okf-maintain, at: 2026-08-02T11:46:25Z }
---

# arsenal

Security tool package manager. Search, install, update, remove CLI tools from chainreactors, projectdiscovery, and any GitHub repo.

Installed tools are immediately available via bash. `arsenal` itself runs through bash.

## Execution environment

Arsenal installs release artifacts for the harness host's OS and architecture into
the host PATH. Its catalogue is extensible; it is not an OS package manager.
Packages installed inside a WSL distribution belong to that distribution and
are separate from Arsenal's host installations. WSL availability, distribution
names, installed packages and graphical-session support are discovered runtime
properties, not assumptions implied by the `bash` tool name.

The embedded shell accepts POSIX sh syntax on every host. Native command file
arguments use the current invocation directory, including after a shell `cd`.
`/dev/null` redirection and built-in curl's `/dev/null` file arguments use the
host null device. Windows, POSIX-shell and WSL path namespaces differ; a path
passed into another environment must name a file in that environment.
The native read/write file tools use session-relative local paths (or explicitly
mounted virtual URIs for reading), within their configured filesystem root.
An absolute path accepted by an external executable is not that file service's
path contract.

## First step

Before installing anything, run `arsenal list` to see what's available and what's already installed:

```bash
arsenal list
```

Output shows `*` and version for installed tools:
```
* v2.14.1  gogo               [chainreactors     ] Host, port, service and banner discovery
* v1.2.3   dnsx               [projectdiscovery  ] Fast DNS toolkit
           nuclei             [projectdiscovery  ] Fast vulnerability scanner with 9000+ templates
           httpx              [projectdiscovery  ] Fast HTTP toolkit with tech detection
3/22 installed
```

## Quick reference

```bash
arsenal list                                        # all tools + install status + version
arsenal search port scanner                         # find tools by keyword/tag
arsenal info nuclei                                 # detail + docs URL + hint + latest version
arsenal install httpx                               # install latest (idempotent)
arsenal install dnsx --version 1.2.3                # install pinned version
arsenal update nuclei                               # re-download latest version
arsenal remove httpx                                # delete installed binary
arsenal releases nuclei                             # check latest release tag
arsenal add ffuf/ffuf --pattern "{name}_{version}_{os}_{arch}.tar.gz"  # register third-party repo
```

## Key behaviors

- **install is idempotent** — already-installed tools return success unless a different version is explicitly requested. Use `arsenal update` to refresh to the latest release.
- **bundled tools work offline** — bundled builds restore tools at initialization. Install prefers matching bundled versions; update checks remote releases. User upgrades are preserved across application restarts.
- **removing a bundled tool** deletes its disk installation; the next initialization restores it from the application.
- **version from git tag** — versions come from GitHub release tags, not filename parsing.
- **install output includes docs + hint** — follow the hint (e.g. "run nuclei -update-templates").
- **gogo/spray/zombie are built-in pseudo-commands** — no install needed for those. Arsenal has them for standalone binary use only.

## Typical workflow

```bash
arsenal list                          # see what's installed and available
arsenal search subdomain              # find tools for the task
arsenal install subfinder             # install
subfinder -d target.example              # use directly
```

## Adding unknown tools

```bash
arsenal add ffuf/ffuf --pattern "{name}_{version}_{os}_{arch}.tar.gz"
arsenal install ffuf
ffuf -u https://target.example/FUZZ -w wordlist.txt
```

Common patterns: `{name}_{version}_{os}_{arch}.tar.gz` | `{name}_{version}_{os}_{arch}.zip` | `{name}_{os}_{arch}` (raw binary).

## Related concepts

- Security distributions ship companions such as `gogo`, `spray`, and
  `zombie` as built-in pseudo-commands; Arsenal covers everything else.
- Use [tmux](tmux.md) to observe long-running commands installed through
  Arsenal.
