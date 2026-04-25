// Package service provides the business logic layer for MAClawBot.
// Subscribers handle specific message types via the event bus.
package service

import (
	"maclawbot/internal/model"
)

// ---- shared utility functions ----

func hasNonZeroType(items []model.Item) bool {
	for _, it := range items {
		if it.Type != 0 {
			return true
		}
	}
	return false
}

func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

func minStr(a, b int) int {
	if a < b {
		return a
	}
	return b
}
