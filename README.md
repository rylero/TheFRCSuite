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
- `/simulate` — Launch robot simulation, connect ClaudeScope, and run a goal-driven investigation headlessly

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

## Example
Here is an example of using Claude Scope that highights its usecase. I asked the following question:
```
/scope i think the swerve modules are tracking to slowly to the setpoints? could it not be tuned correctly? take a look. I have a log file at "C:\Users\----\Documents\akit_26-04-09_01-06-13.wpilog"
```

And after running claude scope and analyzing the data it was able to come up with:
```
● Now I have everything I need. Let me synthesize the findings.

  ---
  Confirmed: modules are underdamped, not just slow.

  Here's what the log shows:

  Steer response during a direction change (~t=1.476–1.69s in enabled window):
  - Module 0's turn velocity oscillates: 9.87 → 19.6 → 8.95 → 19.2 → 22.6 → 10.1 → 3.8 → 0.9 rad/s over ~200ms
  - The voltage follows the oscillation: 3.17V → 1.04V → 3.49V → 4.63V → 4.91V — the controller is fighting itself
  - Modules 2 & 3 also show persistent ~1.4–2.2V output for 400+ms after the peak, still chasing the setpoint

  Root cause: kD is too low for the kP you're running.

  Your current gains (voltage mode, mechanism level):
  kP = 30 V/rad
  kD = 0.2 V·s/rad   ← this is the problem
  kV = 1.5            (fine)
  kS = 0.1            (fine)

  For critical damping, a rough target is kD ≈ kP × desired_settling_time. With kP=30 and steer gear ratio 12.1:1, you need kD roughly 5–10× higher
  to stop the oscillation. At kD=0.2, the system rings with a ~45ms half-period.

  Recommended fix:
  new Slot0Configs()
      .withKP(30)
      .withKI(0)
      .withKD(1.0)   // up from 0.2 — try 0.8–1.5 range
      .withKS(0.1)
      .withKV(1.5)
      .withKA(0)

  Start at kD=1.0, tune up if still ringing, tune down if you see noise-driven chatter. You can also try kD=0.5 as a conservative first step.

  Module 2 & 3 extra note: they show a persistent small voltage offset (~-0.23V) even when settled at 0 velocity. That's kS doing its job, which is
  fine — but if you see static angle drift, bump kS slightly.
```

I could then also ask follow up questions:
```
❯ Do you think kP is enough? from the voltages you showed it seems like im not powering the motors much considering the voltage is between 1-5V

● Good catch — but I think the 1-5V is actually expected behavior, not a sign kP is too low.

  Here's why: Phoenix 6 gains operate in V/rotation (not V/radian). So:

  kP = 30 V/rotation (mechanism)

  Saturates at 12V when error > 12/30 = 0.4 rotations = 144°

  A 45° correction → 30 × 0.125 rot = 3.75V ✓ matches your data
  A 90° correction → 30 × 0.25 rot = 7.5V

  The oscillation you're seeing (velocities bouncing 9→19→9→22 rad/s) happens after the big initial move. By the time the module is oscillating,
  it's already close to the target — positional error is only ~5-15°, hence 1-5V. That's not weak kP, that's underdamped overshoot.

  kP is fine. kD is the fix. kD=0.2 V·s/rotation means at peak velocity of 22 rad/s ≈ 3.5 rot/s, you only get 0.2 × 3.5 = 0.7V of damping. That's
  nothing against 3-5V of kP driving force.

  To actually answer your question: if you raise kP without raising kD proportionally, the oscillation gets worse, not better — faster initial
  response but harder overshoot.

  Recommended path:
  1. First fix: kD = 1.0 (keep kP=30)
  2. If initial snap still feels sluggish after oscillation is gone, then raise kP to 40-50 with kD scaled proportionally
```

See [ClaudeScope README](ClaudeScope/) for full documentation.

## TODO
- Testing reference skill
- Logging and AdvantageKit reference skill
