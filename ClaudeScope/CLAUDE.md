Notes from my planning discussion:

Necessary Features:
- Connect to a Network Tables Instance
- Disconnect from the current Log instance
- Load log file
- Find all time ranges where a specific boolean field is true/false
- Find all time ranges where a specific value field is within a threshold
- Get Log Info - all NT fields, time range, FMS Info
- Get Data Range
    - For specific time range, default is start and current/end of log
    - If the start and end time are negative, than those are backward offsets from the end/live point of the log
    - Export options for csv or other format
- Run simple statistics - mean, median, mode, avg change/time, min change/time, max change/time, quartiles, min, max
- Set a specific NT value now
- Get a specific NT value at a specific time - default time is now

Each of these actions are avaliable within a session. A session is mostly just something that controls where the data comes from. Ex. if your session is based on a log file, you cant set an nt value. 

All logs should be based around a single interface, that way it can support both live network tables and past log data
