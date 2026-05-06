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
2. **Launch the sim** using the **PowerShell tool** with `run_in_background: true` — the Bash tool (Git Bash) cannot run `.bat` files:
   ```powershell
   .\gradlew.bat simulateJava 2>&1
   ```
3. **Poll until NT is live** — retry every 3s, up to 90s timeout (Gradle + JVM startup is slow). Use **PowerShell tool**:
   ```powershell
   $session = $null
   for ($i = 0; $i -lt 30; $i++) {
     $r = ClaudeScope connect 127.0.0.1 2>&1
     if ($r -match 'session_id') { $session = ($r | ConvertFrom-Json).session_id; break }
     Start-Sleep 3
   }
   ```
4. **Enumerate all fields:**
   ```powershell
   ClaudeScope info --session <id>
   ```

---

## Phase 2 — Discover

1. Parse the goal → identify relevant subsystems
2. Map to NT field paths using AdvantageKit patterns:
   - Subsystem outputs: `/AdvantageKit/RealOutputs/<Subsystem>/<Field>`
   - Robot state: `/AdvantageKit/DriverStation/<Field>`
3. Determine required robot mode:

| Goal type | Robot mode |
|---|---|
| Observe initialized state | Disabled — no enable needed |
| Test teleop behavior | `Enable=true`, `Autonomous=false` |
| Run autonomous routine | `Autonomous=true`, `Enable=true` |
| Test mode | `Test=true`, `Enable=true` |

---

## Phase 3 — Execute

### Selecting an Autonomous Routine

**⚠️ CRITICAL: SmartDashboard SendableChooser keys cannot be set via `ClaudeScope set`.**

The robot re-publishes `active`, `default`, and `options` on every loop cycle, immediately overwriting any external write. Setting `/SmartDashboard/<Chooser>/active` will appear to succeed (`{}`) but the value will revert to `""` within 20ms.

**Workaround — use `setDefaultOption` in code:**

1. Verify the exact option name first (user descriptions may not match exactly):
   ```powershell
   ClaudeScope get "/SmartDashboard/Auto Choices/options" --session <id>
   ```
2. In `AutoRoutines.java` (or wherever the chooser is built), temporarily change the target auto from `addOption` → `setDefaultOption`:
   ```java
   // Before (revert after test):
   autoChooser.setDefaultOption("Left Trench Mid Rush (double)", command);
   // Was: autoChooser.addOption(...)
   ```
3. Kill the sim, rebuild (`.\gradlew.bat simulateJava`), reconnect ClaudeScope
4. Verify it took before enabling:
   ```powershell
   ClaudeScope get "/SmartDashboard/Auto Choices/active" --session <id>
   # Should show the chosen auto name
   ```
5. **Revert the code change** after the test is complete

### Enabling the Robot

Use **PowerShell tool** (no `MSYS_NO_PATHCONV=1` needed):
```powershell
# Autonomous
ClaudeScope set "/Sim/Autonomous=true" --session <id>
ClaudeScope set "/Sim/Enable=true" --session <id>

# Teleop
ClaudeScope set "/Sim/Autonomous=false" --session <id>
ClaudeScope set "/Sim/Enable=true" --session <id>
```

> `/Sim/Enable` and `/Sim/Autonomous` work because the robot only subscribes to them (never publishes). The SendableChooser limitation above does not apply here.

### Collecting Data

**⚠️ Always query `range` from `--start 0`, not from the enable timestamp.**

`range` only returns data points where the value *changed*. If you query a window that starts after the last change, it returns `null` — not the value that was current at that time. Querying from 0 guarantees you capture all transitions, then clip to your window manually.

```powershell
# Correct — always start from 0
ClaudeScope range /AdvantageKit/RealOutputs/Superstructure/CurrentSuperState --start 0 --end 0 --session <id>

# Wrong — returns null if no changes occurred in the narrow window
ClaudeScope range /AdvantageKit/RealOutputs/Superstructure/CurrentSuperState --start 17000000 --end 42000000 --session <id>
```

**⚠️ `get --time` is unreliable for historical lookup** — it may return the latest value rather than the value at the specified timestamp. Use `range --start 0` and manually find the value at any point in time.

**Finding the exact auto enable time** — use DS transitions, not AK Timestamp:
```powershell
# Get precise enable time (µs) — use this as your window start
ClaudeScope range /AdvantageKit/DriverStation/Enabled --start 0 --end 0 --session <id>
ClaudeScope range /AdvantageKit/DriverStation/Autonomous --start 0 --end 0 --session <id>
```

**Time-in-state analysis pattern:**
1. Query `range` from 0 → end of window for the state field
2. Find exact enable time `T` from DS Enabled transitions
3. Clip the returned transitions to `[T, T + duration]`
4. For each segment, duration = next_timestamp − current_timestamp (last segment ends at window boundary)
5. Sum durations by state value

| Command | Use for |
|---|---|
| `range` | Time-series data — always `--start 0` for string/enum fields |
| `get` | Current value only (time=0); unreliable for historical lookup |
| `stats` | mean/min/max/quartiles for numeric fields |
| `find-bool` | Time windows where a boolean was true/false |
| `find-threshold` | Time windows where a value was in a range |

**Disable when done collecting:**
```powershell
ClaudeScope set "/Sim/Enable=false" --session <id>
```

---

## Phase 4 — Report

1. Analyze collected data against the goal — surface specific anomalies, tracking errors, unexpected state transitions with timestamps and values
2. Propose concrete next steps: code changes, gain adjustments, logic fixes
3. Disconnect ClaudeScope:
   ```powershell
   ClaudeScope disconnect --session <id>
   ```
4. Terminate the sim process:
   ```powershell
   Stop-Process -Name "java" -ErrorAction SilentlyContinue
   ```
5. **Revert any temporary code changes** (e.g. `setDefaultOption` → `addOption`)

---

## Constraints

| Constraint | Detail |
|---|---|
| Windows build | Use PowerShell tool + `.\gradlew.bat`, never Bash tool (Git Bash can't run `.bat`) |
| Build time | First run: 60–90s — use the full polling timeout |
| NT only in SIM | `NT4Publisher` only — no `.wpilog` written in SIM mode |
| Struct fields | Type `structschema` = raw bytes — note as undecodable |
| Path prefix | `MSYS_NO_PATHCONV=1` only needed in Bash tool, not PowerShell |
| SendableChooser | Robot re-publishes chooser keys every loop — external `set` is overwritten immediately |
| `range` nulls | String/enum fields return `null` if no changes in the queried window — always query from 0 |
| `get --time` | Unreliable for historical state — use `range --start 0` instead |
