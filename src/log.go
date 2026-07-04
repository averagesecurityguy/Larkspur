package main

import (
	"time"
	"strings"

	"github.com/rs/zerolog"
)

// configureLogger configures the logger that will be used by larkspur. It is
// a structured logger that produces JSON output and can log Debug level and
// higher events.
func configureLogger(level string) {
	// Set the appropriate log level
	switch strings.ToLower(level) {
	case "debug":
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	case "info":
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	case "warn":
		zerolog.SetGlobalLevel(zerolog.WarnLevel)
	default:
		zerolog.SetGlobalLevel(zerolog.ErrorLevel)
	}

	// Configure UTC time
	zerolog.TimestampFunc = time.Now().UTC
}

