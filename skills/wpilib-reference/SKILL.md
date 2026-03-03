---
name: WPILIB-Reference
description: A reference for common coding styles, patterns, and features of the WPILIB FRC Framework
---

# Commands
Wpilib works off a command based framework. Commands are scheduled and then run in the following order:
1. Initialize
2. Execute
3. Is Finished?
4. End
The process of running commands is managed by the command scheduler. Commands also have required subsystems, and if a subsystem already has command running, the new command will interrupt it, triggering the end function.

Command can also be composed together to form more complex logic. The most common compositions are:
Commands.sequence(...commands)
Commands.parallel(...commands)
Commands.deadline(deadline command, ...commands)
Commands.either(a, b, boolean supplier)
Commands.defer(() -> command)
Commands.deferredProxy(() -> command) // unlike defer which just runs the command when called, this creates a command that will then schedule running this defered command, its rarely used
Commands.race(...commands)
Commands.repeatingSequence(...commands)
Commands.select(Map<K,Command> commands, Supplier<? extends K> selector)

Commands.none() //does literaly nothing
Commands.waitSeconds(time Seconds)
Commands.waitTime(time Time)

To create commands the best way is through functional commands:
Commands.runOnce(func, Set<Subsystem>)
Commands.run(func, Set<Subsystem>)
Commands.runEnd(func, func, Set<Subsystem>) // run until interrupted, then end
Commands.startRun(func, func, Set<Subsystem>)

although when in a subsystem you can remove the Commands part and use the subsystems built in methods to automatically include that specific subsystem

Commands can also be created in a class based manner by ovverriding each method, although try to avoid this if possible

# Subsystems
A subsystem is a class that holds io devices and command factories. It also has a default command which runs at all times unless another command is requiring the subsystem.
Examples of subsystems are: Drive, Intake, Shooter, Elevator, Arm, Vision

Subsystems extend SubsystemBase and have a periodic and simulationPeriodic method which run once every robot tick cycle (0.02s).
If the total time of all commands and subsystems periodic functionality exceeds 0.02s it is a command scheduler loop overrun error which slows down the robot and decreases performance.

# Logging
Logging in wpilib uses DataLogManager. DataLogManager is realtivly simple:
DataLogManager.log(message String) will log to the network tables and console. 

To easily log values for plotting use smart dashboard:
SmartDashboard.putNumber(name, value)
SmartDashboard.putNumberArray(name, value)
SmartDashboard.putBoolean(name, value)

SmartDashboard.putData(value Sendable)
SmartDashboard.putData(name, value Sendable)

Each of these methods also have get variations as well.
While SmartDashboard isn't as efficent as pure network tables it is much easier to read and alter.

# Units - Best Practices
Wpilib Units is a great framework for creating reliable and robust code that deals with measurements
Units is made up of a few parts:

### Measures
These are things like Distance, AngularVelocity, Mass, etc.
These are the types that you store things in when storing values with units

### Units
These are things like Inches, RotationsPerSecond (RPS), or MetersPerSecond.
Ex.: Inches.of(10).in(Meters)

### Units Helper
This has some common utility classes for managing units:
Units.degreesToRotations
Units.degreesToRadians
Units.radiansToDegrees
along with other simple conversion tools

## Common Objects
You will often deal with the following types:
Rotation2d, Translation2d, Pose2d, Pose3d

# Further Reference
Please refer to:
- ctre_reference.md - for motor and control information
- testing_reference.md - for testing information
- advantagekit_reference.md - for subsystem and logging information in advantage kit style projects
