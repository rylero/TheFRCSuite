<img width="1384" height="496" alt="image" src="https://github.com/user-attachments/assets/3cd2c109-de74-47ab-a027-a4f52654431d" />

# The FRC Suite

A series of tools that maximize Claude's effectiveness for FRC robot debugging and development.

## Claude Code Plugin

Install the plugin to get all skills in Claude Code:

```bash
/plugin marketplace add rylero/TheFRCSuite
/plugin install thefrc-suite@rylero/TheFRCSuite
```

**Included skills:**
- `/scope` — Analyze `.wpilog` files and query live NetworkTables via ClaudeScope
- `/wpilib-reference` — WPILib command-based framework patterns and reference

## ClaudeScope

A CLI tool for querying FRC robot log files (`.wpilog`) and live NetworkTables.

**Download:** [GitHub Releases](https://github.com/rylero/TheFRCSuite/releases)

| Platform | Binary |
|---|---|
| Windows | `ClaudeScope-windows-amd64.exe` |
| macOS (Apple Silicon) | `ClaudeScope-darwin-arm64` |
| macOS (Intel) | `ClaudeScope-darwin-amd64` |
| Linux | `ClaudeScope-linux-amd64` |

Add the binary to your PATH and rename to `ClaudeScope` (or `ClaudeScope.exe` on Windows).

```bash
# Load a log and inspect it
ClaudeScope load my_match.wpilog
ClaudeScope info --session <id>
ClaudeScope help   # full JSON schema for all commands
```

See [ClaudeScope README](ClaudeScope/) for full documentation.

## TODO
- Testing reference skill
- Logging and AdvantageKit reference skill
