package session

// SessionType identifies whether a session is live NT4 or a loaded log file.
type SessionType int

const (
	LiveSession SessionType = iota
	LogSession
)

// FieldInfo describes one data field in a session.
type FieldInfo struct {
	Key  string `json:"key"`
	Type string `json:"type"` // "double", "boolean", "string", etc.
}

// DataPoint is one timestamped value.
type DataPoint struct {
	Timestamp int64 `json:"timestamp"` // microseconds since log start
	Value     any   `json:"value"`
}

// TimeRange is a closed interval in microseconds.
type TimeRange struct {
	Start int64 `json:"start"`
	End   int64 `json:"end"`
}

// Stats holds descriptive statistics for a numeric field over a time window.
type Stats struct {
	Mean     float64 `json:"mean"`
	Median   float64 `json:"median"`
	Min      float64 `json:"min"`
	Max      float64 `json:"max"`
	Q1       float64 `json:"q1"`
	Q3       float64 `json:"q3"`
	AvgDelta float64 `json:"avg_delta"` // average change per second
	MinDelta float64 `json:"min_delta"`
	MaxDelta float64 `json:"max_delta"`
}

// DataSession is the common interface for both live NT4 and wpilog sessions.
type DataSession interface {
	// Type returns LiveSession or LogSession.
	Type() SessionType

	// Fields returns all known fields and their types.
	Fields() ([]FieldInfo, error)

	// TimeRange returns the start and end timestamps in microseconds.
	// For live sessions, start=0 and end=time since connect (relative).
	TimeRange() (start, end int64, err error)

	// GetValues returns the value of each key at or before timestamp t.
	// If t == 0, returns the latest known value.
	GetValues(keys []string, t int64) (map[string]*DataPoint, error)

	// GetRanges returns all data points for each key in [start, end].
	// If start == 0, begins from the first record. If end == 0, goes to the last.
	GetRanges(keys []string, start, end int64) (map[string][]DataPoint, error)

	// FindBoolRanges returns all intervals where key equals value.
	FindBoolRanges(key string, value bool) ([]TimeRange, error)

	// FindThresholdRanges returns all intervals where min <= key <= max.
	FindThresholdRanges(key string, min, max float64) ([]TimeRange, error)

	// Stats computes descriptive statistics for a numeric field over [start, end].
	Stats(key string, start, end int64) (*Stats, error)

	// Set publishes key/value pairs. Returns an error for LogSession.
	Set(pairs map[string]any) error

	// Close releases all resources held by the session.
	Close() error
}
