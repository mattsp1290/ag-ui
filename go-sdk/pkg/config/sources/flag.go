package sources

import (
	"context"
	"flag"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// FlagSource loads configuration from command-line flags
type FlagSource struct {
	flagSet     *flag.FlagSet
	prefix      string
	keyMapping  map[string]string
	transformer func(string) string
	priority    int
	parsed      bool
}

// FlagSourceOptions configures flag source behavior
type FlagSourceOptions struct {
	FlagSet     *flag.FlagSet
	Prefix      string
	KeyMapping  map[string]string
	Transformer func(string) string
	Priority    int
}

// NewFlagSource creates a new command-line flag source
func NewFlagSource() *FlagSource {
	return NewFlagSourceWithOptions(&FlagSourceOptions{
		FlagSet:  flag.CommandLine,
		Priority: 30,
	})
}

// NewFlagSourceWithOptions creates a new flag source with options
func NewFlagSourceWithOptions(options *FlagSourceOptions) *FlagSource {
	if options == nil {
		options = &FlagSourceOptions{}
	}
	
	if options.FlagSet == nil {
		options.FlagSet = flag.CommandLine
	}
	
	return &FlagSource{
		flagSet:     options.FlagSet,
		prefix:      options.Prefix,
		keyMapping:  options.KeyMapping,
		transformer: options.Transformer,
		priority:    options.Priority,
	}
}

// Name returns the source name
func (f *FlagSource) Name() string {
	if f.prefix != "" {
		return fmt.Sprintf("flags:%s", f.prefix)
	}
	return "flags"
}

// Priority returns the source priority
func (f *FlagSource) Priority() int {
	return f.priority
}

// Load loads configuration from command-line flags
func (f *FlagSource) Load(ctx context.Context) (map[string]interface{}, error) {
	config := make(map[string]interface{})
	
	// Parse flags if not already parsed
	if !f.parsed && f.flagSet == flag.CommandLine {
		if !flag.Parsed() {
			flag.Parse()
		}
		f.parsed = true
	}
	
	// Visit all defined flags
	f.flagSet.Visit(func(fl *flag.Flag) {
		key := fl.Name
		value := fl.Value.String()
		
		// Filter by prefix if specified
		if f.prefix != "" && !strings.HasPrefix(key, f.prefix+"-") {
			return
		}
		
		// Remove prefix
		if f.prefix != "" {
			key = strings.TrimPrefix(key, f.prefix+"-")
		}
		
		// Apply key mapping
		if f.keyMapping != nil {
			if mapped, ok := f.keyMapping[key]; ok {
				key = mapped
			}
		}
		
		// Apply transformer
		if f.transformer != nil {
			key = f.transformer(key)
		} else {
			// Default transformation: replace hyphens with dots
			key = strings.ReplaceAll(key, "-", ".")
		}
		
		// Parse value based on flag type
		parsedValue := f.parseValue(fl, value)
		
		// Set nested value
		if err := f.setNestedValue(config, key, parsedValue); err != nil {
			return // Skip invalid keys
		}
	})
	
	return config, nil
}

// Watch starts watching for flag changes (not applicable for flags)
func (f *FlagSource) Watch(ctx context.Context, callback func(map[string]interface{})) error {
	// Command-line flags don't change during runtime
	return nil
}

// CanWatch returns whether this source supports watching
func (f *FlagSource) CanWatch() bool {
	return false
}

// LastModified returns when the source was last modified
func (f *FlagSource) LastModified() time.Time {
	// Flags don't have modification times, use program start time
	return time.Now()
}

// parseValue parses a flag value based on its type
func (f *FlagSource) parseValue(fl *flag.Flag, value string) interface{} {
	// Determine the type from the flag's value
	switch fl.Value.(type) {
	case boolFlag:
		if val, err := strconv.ParseBool(value); err == nil {
			return val
		}
		return false
		
	case intFlag:
		if val, err := strconv.ParseInt(value, 10, 32); err == nil {
			return int(val)
		}
		return 0
		
	case int64Flag:
		if val, err := strconv.ParseInt(value, 10, 64); err == nil {
			return val
		}
		return int64(0)
		
	case float64Flag:
		if val, err := strconv.ParseFloat(value, 64); err == nil {
			return val
		}
		return 0.0
		
	case durationFlag:
		if val, err := time.ParseDuration(value); err == nil {
			return val
		}
		return time.Duration(0)
		
	default:
		// Handle comma-separated values
		if strings.Contains(value, ",") {
			parts := strings.Split(value, ",")
			slice := make([]string, len(parts))
			for i, part := range parts {
				slice[i] = strings.TrimSpace(part)
			}
			return slice
		}
		
		// Default to string
		return value
	}
}

// setNestedValue sets a value in a nested map using dot notation
func (f *FlagSource) setNestedValue(config map[string]interface{}, key string, value interface{}) error {
	keys := strings.Split(key, ".")
	current := config
	
	for i, k := range keys {
		if i == len(keys)-1 {
			// Last key, set the value
			current[k] = value
			return nil
		}
		
		// Intermediate key, ensure it's a map
		if _, ok := current[k]; !ok {
			current[k] = make(map[string]interface{})
		}
		
		if next, ok := current[k].(map[string]interface{}); ok {
			current = next
		} else {
			// Can't traverse further, key conflicts with existing value
			return fmt.Errorf("key conflict at %s", k)
		}
	}
	
	return nil
}

// Interface types to help with type detection
type boolFlag interface {
	flag.Value
	IsBoolFlag() bool
}

type intFlag interface {
	flag.Value
}

type int64Flag interface {
	flag.Value
}

type float64Flag interface {
	flag.Value
}

type durationFlag interface {
	flag.Value
}