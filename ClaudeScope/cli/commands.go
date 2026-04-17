package cli

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

// RunCommand routes CLI args to the correct daemon endpoint.
func RunCommand(args []string) ([]byte, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("no subcommand provided")
	}
	switch args[0] {
	case "connect":
		return runConnect(args[1:])
	case "load":
		return runLoad(args[1:])
	case "disconnect":
		return runDisconnect(args[1:])
	case "info":
		return runInfo(args[1:])
	case "get":
		return runGet(args[1:])
	case "range":
		return runRange(args[1:])
	case "find-bool":
		return runFindBool(args[1:])
	case "find-threshold":
		return runFindThreshold(args[1:])
	case "stats":
		return runStats(args[1:])
	case "set":
		return runSet(args[1:])
	default:
		return nil, fmt.Errorf("unknown subcommand: %s", args[0])
	}
}

func parseFlags(args []string) (positional []string, flags map[string]string) {
	flags = make(map[string]string)
	for i := 0; i < len(args); i++ {
		if strings.HasPrefix(args[i], "--") && i+1 < len(args) {
			key := strings.TrimPrefix(args[i], "--")
			flags[key] = args[i+1]
			i++
		} else {
			positional = append(positional, args[i])
		}
	}
	return
}

func requireSession(flags map[string]string) (string, error) {
	id, ok := flags["session"]
	if !ok || id == "" {
		return "", fmt.Errorf("--session <id> is required")
	}
	return id, nil
}

func flagInt64(flags map[string]string, key string, defaultVal int64) int64 {
	s, ok := flags[key]
	if !ok {
		return defaultVal
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return defaultVal
	}
	return v
}

func flagFloat64(flags map[string]string, key string) (float64, error) {
	s, ok := flags[key]
	if !ok {
		return 0, fmt.Errorf("missing required flag --%s", key)
	}
	return strconv.ParseFloat(s, 64)
}

func runConnect(args []string) ([]byte, error) {
	pos, _ := parseFlags(args)
	if len(pos) < 1 {
		return nil, fmt.Errorf("usage: connect <ip>")
	}
	return DoRequest(http.MethodPost, "/connect", map[string]string{"ip": pos[0]})
}

func runLoad(args []string) ([]byte, error) {
	pos, _ := parseFlags(args)
	if len(pos) < 1 {
		return nil, fmt.Errorf("usage: load <path.wpilog>")
	}
	return DoRequest(http.MethodPost, "/load", map[string]string{"path": pos[0]})
}

func runDisconnect(args []string) ([]byte, error) {
	_, flags := parseFlags(args)
	id, err := requireSession(flags)
	if err != nil {
		return nil, err
	}
	return DoRequest(http.MethodPost, "/disconnect", map[string]string{"session_id": id})
}

func runInfo(args []string) ([]byte, error) {
	_, flags := parseFlags(args)
	id, err := requireSession(flags)
	if err != nil {
		return nil, err
	}
	return DoRequest(http.MethodGet, "/info?session="+id, nil)
}

func runGet(args []string) ([]byte, error) {
	pos, flags := parseFlags(args)
	if len(pos) < 1 {
		return nil, fmt.Errorf("usage: get <key> [key2 ...] --session <id> [--time <us>]")
	}
	id, err := requireSession(flags)
	if err != nil {
		return nil, err
	}
	return DoRequest(http.MethodPost, "/get", map[string]any{
		"session_id": id,
		"keys":       pos,
		"time":       flagInt64(flags, "time", 0),
	})
}

func runRange(args []string) ([]byte, error) {
	pos, flags := parseFlags(args)
	if len(pos) < 1 {
		return nil, fmt.Errorf("usage: range <key> [key2 ...] --session <id> [--start <us>] [--end <us>]")
	}
	id, err := requireSession(flags)
	if err != nil {
		return nil, err
	}
	return DoRequest(http.MethodPost, "/range", map[string]any{
		"session_id": id,
		"keys":       pos,
		"start":      flagInt64(flags, "start", 0),
		"end":        flagInt64(flags, "end", 0),
	})
}

func runFindBool(args []string) ([]byte, error) {
	pos, flags := parseFlags(args)
	if len(pos) < 2 {
		return nil, fmt.Errorf("usage: find-bool <key> <true|false> --session <id>")
	}
	id, err := requireSession(flags)
	if err != nil {
		return nil, err
	}
	return DoRequest(http.MethodPost, "/find-bool", map[string]any{
		"session_id": id,
		"key":        pos[0],
		"value":      pos[1] == "true",
	})
}

func runFindThreshold(args []string) ([]byte, error) {
	pos, flags := parseFlags(args)
	if len(pos) < 1 {
		return nil, fmt.Errorf("usage: find-threshold <key> --min <n> --max <n> --session <id>")
	}
	id, err := requireSession(flags)
	if err != nil {
		return nil, err
	}
	minVal, err := flagFloat64(flags, "min")
	if err != nil {
		return nil, err
	}
	maxVal, err := flagFloat64(flags, "max")
	if err != nil {
		return nil, err
	}
	return DoRequest(http.MethodPost, "/find-threshold", map[string]any{
		"session_id": id,
		"key":        pos[0],
		"min":        minVal,
		"max":        maxVal,
	})
}

func runStats(args []string) ([]byte, error) {
	pos, flags := parseFlags(args)
	if len(pos) < 1 {
		return nil, fmt.Errorf("usage: stats <key> --session <id> [--start <us>] [--end <us>]")
	}
	id, err := requireSession(flags)
	if err != nil {
		return nil, err
	}
	return DoRequest(http.MethodPost, "/stats", map[string]any{
		"session_id": id,
		"key":        pos[0],
		"start":      flagInt64(flags, "start", 0),
		"end":        flagInt64(flags, "end", 0),
	})
}

func runSet(args []string) ([]byte, error) {
	pos, flags := parseFlags(args)
	if len(pos) < 1 {
		return nil, fmt.Errorf("usage: set <key>=<val> [key2=val2 ...] --session <id>")
	}
	id, err := requireSession(flags)
	if err != nil {
		return nil, err
	}
	pairs := make(map[string]any, len(pos))
	for _, kv := range pos {
		idx := strings.Index(kv, "=")
		if idx < 0 {
			return nil, fmt.Errorf("invalid key=value pair: %q", kv)
		}
		key, val := kv[:idx], kv[idx+1:]
		if f, err := strconv.ParseFloat(val, 64); err == nil {
			pairs[key] = f
		} else if b, err := strconv.ParseBool(val); err == nil {
			pairs[key] = b
		} else {
			pairs[key] = val
		}
	}
	return DoRequest(http.MethodPost, "/set", map[string]any{
		"session_id": id,
		"pairs":      pairs,
	})
}
