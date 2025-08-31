package main

import (
	"context"
	"encoding/base64"
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
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		
		var err error
		var ctx context.Context
		var cancel context.CancelFunc
		
		if killAfterMin > 0 {
			ctx, cancel = context.WithTimeout(context.Background(), time.Duration(killAfterMin)*time.Minute)
			defer cancel()
			cmd = exec.CommandContext(ctx, parts[0], parts[1:]...)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
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