---
name: simulate
description: Use when asked to simulate the FRC robot, test behavior in simulation, analyze robot performance without hardware, or run auto/teleop in a virtual environment. Trigger on: "/simulate", "run the sim", "test in simulation", "simulate auto", "simulate teleop", "run simulation".
---

# Simulate — FRC Robot Simulation with ClaudeScope

Launches WPILib robot simulation, connects ClaudeScope to the live NT4 feed, and runs a goal-driven investigation: observe, enable, collect data, tune, report — fully headless.

## Trigger

`/simulate <goal>` — goal is natural language describing what to test or analyze.

- `/simulate run auto and check drive path accuracy`
- `/simulate verify shooter PID holds 3000 rpm`
- `/simulate check superstructure state transitions during teleop`

---

## Step 0 — Prerequisites Check

**Do this before launching anything.** Read `<robot-project>/src/main/java/frc/robot/Robot.java` and check if `simulationPeriodic()` contains the NT sim-control block. If missing, apply it now — do not skip this step.

**Add to `simulationPeriodic()` after `robotContainer.updateSimulation()`:**
```java
var nt = NetworkTableInstance.getDefault();
DriverStationSim.setEnabled(nt.getEntry("/Sim/Enable").getBoolean(false));
DriverStationSim.setAutonomous(nt.getEntry("/Sim/Autonomous").getBoolean(false));
DriverStationSim.setTest(nt.getEntry("/Sim/Test").getBoolean(false));
DriverStationSim.notifyNewData();
```

**Add imports if missing:**
```java
import edu.wpi.first.networktables.NetworkTableInstance;
import edu.wpi.first.wpilibj.simulation.DriverStationSim;
```

> WPILib simulation's DS enable state is internal to HALSim — not exposed as an NT key by default. This hook is the only way to enable the robot from outside the process. The FRC DS app and SimGUI both use a custom HALSim protocol Claude cannot interact with.

---

## Phase 1 — Setup

1. **Find the robot project path** — check memory and CLAUDE.md first; if not found, search upward from cwd for `build.gradle` containing `GradleRIO`; ask the user if still not found
2. **Launch the sim** as a background process (use `run_in_background: true` on the Bash tool):
   ```bash
   cd /path/to/robot-project && ./gradlew.bat simulateJava
   ```
3. **Poll until NT is live** — retry every 3s, up to 90s timeout (Gradle + JVM startup is slow):
   ```bash
   ClaudeScope connect 127.0.0.1
   # Returns {"session_id":"<id>"} on success
   ```
4. **Enumerate all fields:**
   ```bash
   ClaudeScope info --session <id>
   ```

---

## Phase 2 — Discover

1. Parse the goal → identify relevant subsystems
2. Map to NT field paths using AdvantageKit patterns:
   - Subsystem outputs: `/RealOutputs/<Subsystem>/<Field>`
   - Robot state: `/RobotState/<Field>`
3. Determine required robot mode:

| Goal type | Robot mode |
|---|---|
| Observe initialized state | Disabled — no enable needed |
| Test teleop behavior | `Enable=true`, `Autonomous=false` |
| Run autonomous routine | `Autonomous=true`, `Enable=true` |
| Test mode | `Test=true`, `Enable=true` |

---

## Phase 3 — Execute

**Enable the robot if needed:**
```bash
# Autonomous
MSYS_NO_PATHCONV=1 ClaudeScope set /Sim/Autonomous=true --session <id>
MSYS_NO_PATHCONV=1 ClaudeScope set /Sim/Enable=true --session <id>

# Teleop
MSYS_NO_PATHCONV=1 ClaudeScope set /Sim/Autonomous=false --session <id>
MSYS_NO_PATHCONV=1 ClaudeScope set /Sim/Enable=true --session <id>
```

> `MSYS_NO_PATHCONV=1` is required when using the Bash tool (Git Bash/MSYS2) for keys starting with `/`. Not needed in PowerShell.

**Collect data** (see `thefrc-suite:scope` for full ClaudeScope reference):

| Command | Use for |
|---|---|
| `range` | Time-series data over a window |
| `get` | Single value at a timestamp |
| `stats` | mean/min/max/quartiles for numeric fields |
| `find-bool` | Time windows where a boolean was true/false |
| `find-threshold` | Time windows where a value was in a range |

**Tuning loop:** read current value → `set` NT override → re-collect → compare → iterate

**Disable when done collecting:**
```bash
MSYS_NO_PATHCONV=1 ClaudeScope set /Sim/Enable=false --session <id>
```

---

## Phase 4 — Report

1. Analyze collected data against the goal — surface specific anomalies, tracking errors, unexpected state transitions with timestamps and values
2. Propose concrete next steps: code changes, gain adjustments, logic fixes
3. Disconnect ClaudeScope:
   ```bash
   ClaudeScope disconnect --session <id>
   ```
4. Terminate the sim process:
   ```powershell
   Stop-Process -Name "java" -ErrorAction SilentlyContinue
   ```

---

## Constraints

| Constraint | Detail |
|---|---|
| Windows build | Use `.\gradlew.bat`, not `./gradlew` |
| Build time | First run: 60–90s — use the full polling timeout |
| NT only in SIM | `NT4Publisher` only — no `.wpilog` written in SIM mode |
| Struct fields | Type `structschema` = raw bytes — note as undecodable |
| Path prefix | `MSYS_NO_PATHCONV=1` only needed in Bash tool, not PowerShell |
