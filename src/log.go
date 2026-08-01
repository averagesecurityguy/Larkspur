package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// openLogFile opens the file where logs will be written.
func openLogFile(path string) *os.File {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Printf("Bad log file: %v\n", err)
		os.Exit(exitCodeBadLogFile)
	}

	return f
}

// configureLogger configures the logger that will be used by larkspur. It is
// a structured logger that produces JSON output and can log Debug level and
// higher events.
func configureLogger(level string, file *os.File) {
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
	zerolog.TimestampFunc = func() time.Time {
    	return time.Now().UTC()
	}

	log.Logger = zerolog.New(file).With().Timestamp().Caller().Logger()
}
