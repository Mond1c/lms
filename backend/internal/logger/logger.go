package logger

import (
	"io"
	"log"
	"os"
	"path/filepath"

	"gopkg.in/natefinch/lumberjack.v2"
)

// SetupLogger configures logging to files with rotation
func SetupLogger(logDir string) error {
	// Create logs directory if it doesn't exist
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return err
	}

	// Configure lumberjack for log rotation
	logFile := &lumberjack.Logger{
		Filename:   filepath.Join(logDir, "app.log"),
		MaxSize:    100, // megabytes
		MaxBackups: 30,  // keep 30 old log files
		MaxAge:     30,  // days
		Compress:   true,
	}

	// Write to both file and stdout
	multiWriter := io.MultiWriter(os.Stdout, logFile)

	// Set default logger output
	log.SetOutput(multiWriter)
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)

	log.Println("Logger initialized, writing to:", logFile.Filename)

	return nil
}
