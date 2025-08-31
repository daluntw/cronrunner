# CronRunner

A lightweight, statically-compiled cron runner designed for Docker containers. Solves the problem of running scheduled tasks inside Docker containers without requiring a full cron daemon.

## Features

- **Lightweight**: Single statically-compiled binary with no dependencies
- **Docker-optimized**: Designed specifically for containerized environments  
- **Flexible scheduling**: Supports full cron expressions with seconds precision
- **Command timeout**: Optional timeout to prevent runaway processes
- **Base64 encoding**: Environment variables are base64-encoded for special character support
- **Comprehensive logging**: Detailed execution logs with timing information
- **Graceful shutdown**: Handles SIGINT/SIGTERM signals properly

## Installation

### Download Pre-built Binary

```bash
# Download latest release
wget https://github.com/daluntw/cronrunner/releases/download/v1.0/cronrunner-linux-amd64.tar.gz -O- | tar -xz
mv cronrunner-linux-amd64 /usr/local/bin/cronrunner
chmod +x /usr/local/bin/cronrunner
```

### Use in Dockerfile

```dockerfile
FROM python:3.13

# Install cronrunner
RUN wget https://github.com/daluntw/cronrunner/releases/download/v1.0/cronrunner-linux-amd64.tar.gz -O- | tar -xz && \
    mv cronrunner-linux-amd64 /cronrunner && \
    chmod +x /cronrunner

# Your application setup
COPY . /app
WORKDIR /app
RUN chmod +x start.sh

# Use cronrunner as entrypoint
CMD ["/cronrunner"]
```

## Usage

### Environment Variables

| Variable | Required | Description | Format |
|----------|----------|-------------|---------|
| `CRON_EXPRESSION` | Yes | Cron schedule expression | Base64 encoded |
| `CRON_CMD` | Yes | Command to execute | Base64 encoded |
| `CRON_KILL_AFTER_MIN` | No | Timeout in minutes | Plain integer |

### Examples

**Basic usage:**
```bash
docker run -d \
  -e CRON_EXPRESSION=$(echo "0 8 * * *" | base64) \
  -e CRON_CMD=$(echo "/app/backup.sh" | base64) \
  your-image
```

**With timeout (kill after 30 minutes):**
```bash
docker run -d \
  -e CRON_EXPRESSION=$(echo "0 */6 * * *" | base64) \
  -e CRON_CMD=$(echo "/app/heavy-task.sh" | base64) \
  -e CRON_KILL_AFTER_MIN=30 \
  your-image
```

**High-frequency execution (every 10 seconds):**
```bash
docker run -d \
  -e CRON_EXPRESSION=$(echo "*/10 * * * * *" | base64) \
  -e CRON_CMD=$(echo "/app/monitor.sh" | base64) \
  your-image
```

### Cron Expression Format

CronRunner supports standard cron expressions with optional seconds field:

```
┌───────────── second (0-59) [optional]
│ ┌───────────── minute (0-59)
│ │ ┌───────────── hour (0-23)
│ │ │ ┌───────────── day of month (1-31)
│ │ │ │ ┌───────────── month (1-12)
│ │ │ │ │ ┌───────────── day of week (0-6)
│ │ │ │ │ │
* * * * * *
```

**Common patterns:**
- `0 8 * * *` - Daily at 8:00 AM
- `0 */6 * * *` - Every 6 hours
- `*/30 * * * * *` - Every 30 seconds
- `0 0 * * 0` - Weekly on Sunday midnight
- `0 0 1 * *` - Monthly on 1st day

## Base64 Encoding

Environment variables are base64-encoded to handle special characters and complex commands:

```bash
# Simple command
echo "/app/script.sh" | base64

# Command with arguments
echo "/app/backup.sh --verbose --output /tmp" | base64

# Shell command with pipes
echo "ps aux | grep python | wc -l" | base64
```

## Logging

CronRunner provides comprehensive logging:

```
2025/09/01 08:00:00 Starting cronrunner with schedule: 0 8 * * *
2025/09/01 08:00:00 Command to execute: /app/backup.sh
2025/09/01 08:00:00 Command timeout: 30 minutes
2025/09/01 08:00:00 Cron runner started successfully
2025/09/01 08:00:00 Executing command: /app/backup.sh
2025/09/01 08:05:23 Command completed successfully in 5m23.456s
```

## Building from Source

### Prerequisites
- Go 1.25.0 or later

### Build Commands

```bash
# Clone repository
git clone https://github.com/daluntw/cronrunner.git
cd cronrunner

# Build for current platform
go build -o cronrunner .

# Build static Linux binary
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -ldflags '-extldflags "-static"' -o cronrunner-linux-amd64 .
```

## Use Cases

### Database Backups
```dockerfile
FROM postgres:15

RUN wget https://github.com/daluntw/cronrunner/releases/download/v1.0/cronrunner-linux-amd64.tar.gz -O- | tar -xz && \
    mv cronrunner-linux-amd64 /cronrunner

COPY backup.sh /app/
RUN chmod +x /app/backup.sh

CMD ["/cronrunner"]
```

```bash
docker run -d \
  -e CRON_EXPRESSION=$(echo "0 2 * * *" | base64) \
  -e CRON_CMD=$(echo "/app/backup.sh" | base64) \
  -e CRON_KILL_AFTER_MIN=60 \
  postgres-backup
```

### Log Rotation
```bash
docker run -d \
  -e CRON_EXPRESSION=$(echo "0 0 * * *" | base64) \
  -e CRON_CMD=$(echo "find /var/log -name '*.log' -mtime +7 -delete" | base64) \
  -v /var/log:/var/log \
  log-cleaner
```

### Health Checks
```bash
docker run -d \
  -e CRON_EXPRESSION=$(echo "*/30 * * * * *" | base64) \
  -e CRON_CMD=$(echo "curl -f http://localhost:8080/health || exit 1" | base64) \
  -e CRON_KILL_AFTER_MIN=1 \
  health-monitor
```

## Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Support

- **Issues**: [GitHub Issues](https://github.com/daluntw/cronrunner/issues)
- **Releases**: [GitHub Releases](https://github.com/daluntw/cronrunner/releases)
- **Documentation**: This README

## Alternatives

If CronRunner doesn't meet your needs, consider:
- **supercronic**: Another Docker-focused cron runner
- **dcron**: Lightweight cron daemon for containers
- **Traditional cron**: Full cron daemon (heavier resource usage)