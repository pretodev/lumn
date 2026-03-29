package daemon

import (
	"io"
	"os"
	"time"

	luaenv "github.com/pretodev/lumn/internal/lua"
)

type Config struct {
	Paths            Paths
	WebhookPort      int
	QueueLimit       int
	ShutdownTimeout  time.Duration
	RetentionMaxRuns int
	RetentionMaxDays int
	LogLevel         string
}

func DefaultConfig(paths Paths) Config {
	return Config{
		Paths:            paths,
		WebhookPort:      6890,
		QueueLimit:       10,
		ShutdownTimeout:  30 * time.Second,
		RetentionMaxRuns: 1000,
		RetentionMaxDays: 30,
		LogLevel:         "info",
	}
}

func LoadConfig(paths Paths, stderr io.Writer, warnf func(string, ...any)) Config {
	cfg := DefaultConfig(paths)
	if _, err := os.Stat(paths.ConfigPath); err != nil {
		return cfg
	}

	rt, err := luaenv.NewRuntime(paths.StateDir, paths.StateDir, stderr)
	if err != nil {
		if warnf != nil {
			warnf("failed to create config runtime, using defaults: %v", err)
		}
		return cfg
	}
	defer rt.Close()

	ref, err := rt.LoadWorkflow(paths.ConfigPath)
	if err != nil {
		if warnf != nil {
			warnf("failed to load %s, using defaults: %v", paths.ConfigPath, err)
		}
		return cfg
	}
	defer rt.DeleteRef(ref)

	value, err := rt.RefToGoValue(ref)
	if err != nil {
		if warnf != nil {
			warnf("failed to parse %s, using defaults: %v", paths.ConfigPath, err)
		}
		return cfg
	}

	root, ok := normalizeStringMap(value)
	if !ok {
		if warnf != nil {
			warnf("daemon config must return a table, using defaults")
		}
		return cfg
	}

	for key, item := range root {
		switch key {
		case "webhook_port":
			if v, ok := toInt(item); ok && v > 0 {
				cfg.WebhookPort = v
			}
		case "queue_limit":
			if v, ok := toInt(item); ok && v > 0 {
				cfg.QueueLimit = v
			}
		case "shutdown_timeout":
			if v, ok := toInt(item); ok && v > 0 {
				cfg.ShutdownTimeout = time.Duration(v) * time.Second
			}
		case "retention":
			retention, ok := normalizeStringMap(item)
			if !ok {
				continue
			}
			if v, ok := toInt(retention["max_executions"]); ok && v > 0 {
				cfg.RetentionMaxRuns = v
			}
			if v, ok := toInt(retention["max_days"]); ok && v > 0 {
				cfg.RetentionMaxDays = v
			}
		case "log_level":
			if v, ok := item.(string); ok && v != "" {
				cfg.LogLevel = v
			}
		default:
			if warnf != nil {
				warnf("unknown lumnd.conf field %q", key)
			}
		}
	}

	return cfg
}

func normalizeStringMap(value any) (map[string]any, bool) {
	raw, ok := value.(map[any]any)
	if !ok {
		typed, ok := value.(map[string]any)
		return typed, ok
	}
	out := make(map[string]any, len(raw))
	for key, item := range raw {
		keyString, ok := key.(string)
		if !ok {
			return nil, false
		}
		out[keyString] = normalizeConfigValue(item)
	}
	return out, true
}

func normalizeConfigValue(value any) any {
	switch typed := value.(type) {
	case map[any]any:
		out, ok := normalizeStringMap(typed)
		if !ok {
			return typed
		}
		return out
	case []any:
		out := make([]any, 0, len(typed))
		for _, item := range typed {
			out = append(out, normalizeConfigValue(item))
		}
		return out
	default:
		return typed
	}
}

func toInt(value any) (int, bool) {
	switch typed := value.(type) {
	case int:
		return typed, true
	case int64:
		return int(typed), true
	case float64:
		return int(typed), true
	default:
		return 0, false
	}
}
