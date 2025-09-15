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
	restartOnFailEnv := os.Getenv("RESTART_ON_FAIL")
	cronTZ := os.Getenv("CRON_TZ")

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

	// Cronrunner's own logs go to stderr by default.
	// If LOG_FILE is set, it will capture only the child process output per run.

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

	// Configure scheduler options
	var cronOptions []cron.Option
	cronOptions = append(cronOptions, cron.WithSeconds())
	if strings.TrimSpace(cronTZ) != "" {
		loc, tzErr := time.LoadLocation(strings.TrimSpace(cronTZ))
		if tzErr != nil {
			log.Fatalf("Invalid CRON_TZ value '%s': %v", cronTZ, tzErr)
		}
		cronOptions = append(cronOptions, cron.WithLocation(loc))
		log.Printf("Using CRON_TZ timezone: %s", cronTZ)
	}

	c := cron.New(cronOptions...)

	// Parse RESTART_ON_FAIL: accept 1, true, TRUE, True
	restartOnFail := false
	if restartOnFailEnv != "" {
		switch strings.ToLower(strings.TrimSpace(restartOnFailEnv)) {
		case "1", "true", "yes", "y":
			restartOnFail = true
		}
	}

	_, err = c.AddFunc(cronSchedule, func() {

		log.Printf("Executing command: %s", appCommand)

		parts := strings.Fields(appCommand)
		if len(parts) == 0 {
			log.Printf("Empty command, skipping execution")
			return
		}

		start := time.Now()
		var hardDeadline time.Time
		if killAfterMin > 0 {
			hardDeadline = start.Add(time.Duration(killAfterMin) * time.Minute)
			log.Printf("Hard kill deadline set for %s (limit: %d minutes)", hardDeadline.Format(time.RFC3339), killAfterMin)
		}

		for attempt := 1; ; attempt++ {

			var cmd *exec.Cmd
			var ctx context.Context
			var cancel context.CancelFunc

			if killAfterMin > 0 {
				remaining := time.Until(hardDeadline)
				if remaining <= 0 {
					log.Printf("Kill deadline reached; not starting attempt %d", attempt)
					break
				}
				ctx, cancel = context.WithTimeout(context.Background(), remaining)
				cmd = exec.CommandContext(ctx, parts[0], parts[1:]...)
			} else {
				cmd = exec.Command(parts[0], parts[1:]...)
			}

			// Open per-run log file (if provided) and tee only child process output
			var cStdout io.Writer = os.Stdout
			var cStderr io.Writer = os.Stderr
			var execLogFile *os.File
			if logFilePath != "" {
				f, openErr := os.OpenFile(logFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
				if openErr != nil {
					log.Printf("Failed to open LOG_FILE '%s' for this run: %v", logFilePath, openErr)
				} else {
					execLogFile = f
					// Write per-run start separator only to the log file
					_, _ = io.WriteString(execLogFile, "===== RUN START "+time.Now().Format(time.RFC3339)+" =====\n")
					cStdout = io.MultiWriter(os.Stdout, execLogFile)
					cStderr = io.MultiWriter(os.Stderr, execLogFile)
				}
			}

			cmd.Stdout = cStdout
			cmd.Stderr = cStderr

			err := cmd.Run()
			duration := time.Since(start)

			if cancel != nil {
				cancel()
			}

			exitCode := 0
			killed := false

			if err != nil {
				// Check if this was a timeout
				if killAfterMin > 0 && ctx != nil && ctx.Err() == context.DeadlineExceeded {
					log.Printf("Command timed out after %v; hard deadline %s reached (limit: %d minutes): %v", duration, hardDeadline.Format(time.RFC3339), killAfterMin, err)
					killed = true
				} else {
					if ee, ok := err.(*exec.ExitError); ok {
						exitCode = ee.ExitCode()
					} else if cmd.ProcessState != nil {
						exitCode = cmd.ProcessState.ExitCode()
					}
				}
			}

			// Write per-run end separator with exit code and duration, then close the log file
			if execLogFile != nil {
				_, _ = io.WriteString(execLogFile, "===== RUN END "+time.Now().Format(time.RFC3339)+" exit="+strconv.Itoa(exitCode)+" duration="+duration.String()+" =====\n\n")
				_ = execLogFile.Close()
			}

			log.Printf("Command exited after %v: exit code %d, error: %v", duration, exitCode, err)

			if restartOnFail && killed {
				log.Printf("RESTART_ON_FAIL is enabled; restarting command...")
				continue
			}

			log.Printf("Command completed")
			break
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
