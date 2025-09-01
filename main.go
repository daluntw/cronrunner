package main

import (
	"context"
	"encoding/base64"
	"io"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/robfig/cron/v3"
)

func main() {
	cronExpr := os.Getenv("CRON_EXPRESSION")
	appCmd := os.Getenv("CRON_CMD")
	killAfterMinStr := os.Getenv("CRON_KILL_AFTER_MIN")
	logFilePath := os.Getenv("LOG_FILE")

	if cronExpr == "" {
		log.Fatal("CRON_EXPRESSION environment variable is required")
	}

	if appCmd == "" {
		log.Fatal("CRON_CMD environment variable is required")
	}

	var killAfterMin int
	if killAfterMinStr != "" {
		var err error
		killAfterMin, err = strconv.Atoi(killAfterMinStr)
		if err != nil {
			log.Fatalf("Invalid CRON_KILL_AFTER_MIN value: %v", err)
		}
	}

	// Configure logging and output tee if LOG_FILE is specified
	var (
		logFile      *os.File
		stdoutWriter io.Writer = os.Stdout
		stderrWriter io.Writer = os.Stderr
	)
	if logFilePath != "" {
		var err error
		logFile, err = os.OpenFile(logFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			log.Fatalf("Failed to open LOG_FILE '%s': %v", logFilePath, err)
		}
		defer logFile.Close()
		stdoutWriter = io.MultiWriter(os.Stdout, logFile)
		stderrWriter = io.MultiWriter(os.Stderr, logFile)
		// Log package writes to stderr by default; keep that behavior and tee to file
		log.SetOutput(stderrWriter)
		log.Printf("Teeing stdout/stderr to log file: %s", logFilePath)
	}

	cronDecoded, err := base64.StdEncoding.DecodeString(cronExpr)
	if err != nil {
		log.Fatalf("Failed to decode CRON_EXPRESSION: %v", err)
	}

	appDecoded, err := base64.StdEncoding.DecodeString(appCmd)
	if err != nil {
		log.Fatalf("Failed to decode CRON_CMD: %v", err)
	}

	cronSchedule := string(cronDecoded)
	appCommand := string(appDecoded)

	log.Printf("Starting cronrunner with schedule: %s", cronSchedule)
	log.Printf("Command to execute: %s", appCommand)
	if killAfterMin > 0 {
		log.Printf("Command timeout: %d minutes", killAfterMin)
	}

	c := cron.New(cron.WithSeconds())

	_, err = c.AddFunc(cronSchedule, func() {
		log.Printf("Executing command: %s", appCommand)
		start := time.Now()

		parts := strings.Fields(appCommand)
		if len(parts) == 0 {
			log.Printf("Empty command, skipping execution")
			return
		}

		cmd := exec.Command(parts[0], parts[1:]...)
		cmd.Stdout = stdoutWriter
		cmd.Stderr = stderrWriter

		var err error
		var ctx context.Context
		var cancel context.CancelFunc

		if killAfterMin > 0 {
			ctx, cancel = context.WithTimeout(context.Background(), time.Duration(killAfterMin)*time.Minute)
			defer cancel()
			cmd = exec.CommandContext(ctx, parts[0], parts[1:]...)
			cmd.Stdout = stdoutWriter
			cmd.Stderr = stderrWriter
		}

		err = cmd.Run()
		duration := time.Since(start)

		if err != nil {
			if killAfterMin > 0 && ctx != nil && ctx.Err() == context.DeadlineExceeded {
				log.Printf("Command timed out after %v (limit: %d minutes): %v", duration, killAfterMin, err)
			} else {
				log.Printf("Command failed after %v: %v", duration, err)
			}
		} else {
			log.Printf("Command completed successfully in %v", duration)
		}
	})

	if err != nil {
		log.Fatalf("Failed to add cron job: %v", err)
	}

	c.Start()
	log.Printf("Cron runner started successfully")

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Printf("Shutting down cron runner...")
	c.Stop()
	log.Printf("Cron runner stopped")
}
