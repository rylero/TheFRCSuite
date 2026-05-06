# Simulate Skill — Design Spec

**Date:** 2026-05-06
**Status:** Approved

## Problem

There is no automated way to launch the FRC robot simulation, connect telemetry tooling to it, and analyze robot behavior — all without user intervention. Testing ideas in simulation currently requires manually launching the sim, opening the FRC DS app, enabling the robot, and reading data in AdvantageScope or similar tools. This skill automates that entire workflow so Claude can simulate, observe, control, and tune the robot headlessly.

## Trigger

`/simulate <goal>` — goal is a natural-language description of what to test or analyze.

Examples:
- `/simulate run auto and check drive path accuracy`
- `/simulate verify shooter PID holds 3000 rpm`
- `/simulate check superstructure state transitions during teleop`

## Architecture

Four sequential phases, preceded by a prerequisite code check.

---

### Step 0 — Prerequisites Check

Before launching anything, Claude reads `Robot.java` and checks `simulationPeriodic()` for the NT sim-control block. If the block is missing, Claude applies it automatically — no user action required.

**Code to add to `simulationPeriodic()` (after `robotContainer.updateSimulation()`):**

```java
var nt = NetworkTableInstance.getDefault();
DriverStationSim.setEnabled(nt.getEntry("/Sim/Enable").getBoolean(false));
DriverStationSim.setAutonomous(nt.getEntry("/Sim/Autonomous").getBoolean(false));
DriverStationSim.setTest(nt.getEntry("/Sim/Test").getBoolean(false));
DriverStationSim.notifyNewData();
```

**Required imports (add if missing):**
```java
import edu.wpi.first.networktables.NetworkTableInstance;
import edu.wpi.first.wpilibj.simulation.DriverStationSim;
```

**Why NT-based:** WPILib simulation's DS enable state is managed through HALSim's internal state machine — it is not exposed as an NT key by default. There is no way to enable the robot from outside the process via NT without this hook. The FRC DS app and SimGUI both use a custom HALSim protocol that Claude cannot interact with. This one-time code addition creates the external control surface ClaudeScope needs.

---

### Phase 1 — Setup

1. Determine the robot project path: check memory and CLAUDE.md first; if not found, search upward from the current working directory for a `build.gradle` containing `GradleRIO`, or ask the user
2. Run `.\gradlew.bat simulateJava` in the robot project directory as a background process (use the Bash tool with `run_in_background: true`)
3. Poll `ClaudeScope connect 127.0.0.1` every 3 seconds, up to a 90-second timeout (Gradle build + JVM startup is slow on first run)
4. On successful connection: run `ClaudeScope info --session <id>` to enumerate all NT fields and the current time range

---

### Phase 2 — Discover

1. Parse the goal to identify relevant subsystems — use AdvantageKit field naming patterns: `/RealOutputs/<Subsystem>/<Field>`
2. Identify what robot mode the goal requires:
   - Observation only → disabled (no enable needed)
   - Teleop behavior → enable only (`/Sim/Enable=true`)
   - Autonomous routine → enable + autonomous flag (`/Sim/Autonomous=true`, `/Sim/Enable=true`)
   - Test mode → enable + test flag (`/Sim/Test=true`, `/Sim/Enable=true`)
3. Build a specific field query plan targeting the relevant subsystems

---

### Phase 3 — Execute

**Enabling the robot (if needed):**
```bash
# For autonomous:
MSYS_NO_PATHCONV=1 ClaudeScope set /Sim/Autonomous=true --session <id>
MSYS_NO_PATHCONV=1 ClaudeScope set /Sim/Enable=true --session <id>

# For teleop:
MSYS_NO_PATHCONV=1 ClaudeScope set /Sim/Autonomous=false --session <id>
MSYS_NO_PATHCONV=1 ClaudeScope set /Sim/Enable=true --session <id>
```

**Data collection:**
- `ClaudeScope range` — time-series data for a field over a window
- `ClaudeScope get` — single value at a timestamp
- `ClaudeScope stats` — mean/min/max/quartiles for numeric fields
- `ClaudeScope find-bool` — find time windows where a boolean was true/false
- `ClaudeScope find-threshold` — find time windows where a value was in range

**Tuning iterations:**
1. Read current value
2. `ClaudeScope set` to inject an NT override
3. Re-collect and compare
4. Repeat until goal is satisfied

**Disable when done collecting:**
```bash
MSYS_NO_PATHCONV=1 ClaudeScope set /Sim/Enable=false --session <id>
```

---

### Phase 4 — Report

1. Analyze collected data against the stated goal — surface specific anomalies, tracking errors, unexpected state transitions, with timestamps and values
2. Propose concrete next steps: code changes, gain adjustments, logic fixes
3. Disconnect ClaudeScope:
   ```bash
   ClaudeScope disconnect --session <id>
   ```
4. Terminate the background sim process — on Windows use `Stop-Process -Name "java" -ErrorAction SilentlyContinue` via PowerShell, or capture the PID from the background Bash job and kill it directly

---

## Key Constraints

| Constraint | Detail |
|---|---|
| Windows launch | Use `.\gradlew.bat simulateJava`, not `./gradlew` |
| NT path prefix | When using the Bash tool (Git Bash/MSYS2), prefix ClaudeScope commands with `MSYS_NO_PATHCONV=1` when keys start with `/`. Not needed in PowerShell. |
| AK field pattern | `/RealOutputs/<Subsystem>/<Field>` for subsystem outputs, `/RobotState/<Field>` for robot state |
| SIM mode NT only | In `SIM` mode the robot uses `NT4Publisher` only — no `.wpilog` file is written |
| Build time | First `simulateJava` run can take 60-90s — use the full polling timeout |
| Struct fields | Fields of type `structschema` are raw bytes — skip or note as undecodable |

## Files Affected

- `<robot-project>/src/main/java/frc/robot/Robot.java` — `simulationPeriodic()` gets the NT sim-control block (Step 0); path discovered dynamically at runtime
- `C:\Users\ryan\Dev\TheFRCSuite\skills\simulate\SKILL.md` — the skill file (new)

## Verification

1. Run `/simulate check that the drive initializes correctly in sim` in a session
2. Confirm Claude checks `Robot.java` and applies the NT block if missing
3. Confirm `gradlew.bat simulateJava` launches in the background
4. Confirm ClaudeScope connects to `127.0.0.1`
5. Confirm `ClaudeScope info` returns fields under `/RealOutputs/Drive/...`
6. Confirm Claude queries relevant fields and produces a coherent report
7. Confirm ClaudeScope disconnects and the sim process is terminated at the end
