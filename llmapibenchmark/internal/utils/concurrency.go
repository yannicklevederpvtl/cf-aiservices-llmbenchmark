package utils

import (
	"errors"
	"sort"
	"strconv"
	"strings"
)

// ParseConcurrencyLevels parses a comma-separated string of concurrency levels.
func ParseConcurrencyLevels(concurrencyStr string) ([]int, error) {
	// Split string
	strLevels := strings.Split(concurrencyStr, ",")
	
	// Convert to integers
	concurrencyLevels := make([]int, 0, len(strLevels))
	for _, levelStr := range strLevels {
		level, err := strconv.Atoi(strings.TrimSpace(levelStr))
		if err != nil {
			return nil, errors.New("invalid concurrency level: " + levelStr)
		}
		if level <= 0 {
			return nil, errors.New("concurrency level must be positive: " + strconv.Itoa(level))
		}
		concurrencyLevels = append(concurrencyLevels, level)
	}

	// Sort the levels for consistency
	sort.Ints(concurrencyLevels)
	return concurrencyLevels, nil
}


