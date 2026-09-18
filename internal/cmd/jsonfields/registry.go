// Package jsonfields provides a registry for --json field completions.
// Each command package registers its available field names here, and a single
// root-level completion function delegates lookups.
package jsonfields

import "sync"

var (
	mu       sync.Mutex
	registry = make(map[string][]string)
)

// Register stores JSON field names for a command path.
// commandPath must match cmd.CommandPath(), e.g. "ywiki page list".
func Register(commandPath string, fields []string) {
	mu.Lock()
	defer mu.Unlock()
	registry[commandPath] = fields
}

// Get returns the JSON fields registered for a command path.
func Get(commandPath string) ([]string, bool) {
	mu.Lock()
	defer mu.Unlock()
	fields, ok := registry[commandPath]
	return fields, ok
}
