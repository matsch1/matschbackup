**WORK IN PROGRESS**

This is a personal, automated backup solution written in Go.

## Core Features

- Intelligent Scheduling: Only runs a backup if the last one is older than a configurable threshold (--max-days).
- Concurrency: Supports parallel backups of multiple paths to maximize efficiency.
- Retention Policy: Automatically purges the oldest backup to stay within a defined --max-backups limit.
- Compression: Zips directories (--zip) to reduce transfer size and disk space on the remote.
- Verification: Guarantees a valid backup by creating a BACKUP_COMPLETED file upon successful transfer.
- Dry Run: Use --dry-run to test your configuration without any file changes.

## Prerequisities
- rclone: Must be installed and configured with a remote target.

## Quick Usage
``` bash
# Back up your documents and a project directory with 5 parallel transfers,
# zipping the project folder, and keeping a max of 7 backups.
./matschbackup \
  --path /home/user/documents \
  --path /home/user/projects/my-project \
  --remote "fritznas:/backups" \
  --concurrency 5 \
  --zip \
  --max-backups 7
```

## Command-Line Flags

| Flag           | Type      | Default | Description                                   |
|----------------|-----------|---------|-----------------------------------------------|
| --path         | []string  |         | Required. Local paths to back up. Can be used multiple times. |
| --remote       | string    |         | Required. The rclone target path.            |
| --max-backups  | int       | 14      | Max number of backups to keep.               |
| --max-days     | int       | 1       | Min. age in days for a new backup.          |
| --concurrency  | int       | 2       | Max. number of parallel backups.            |
| --zip, -z      | bool      | false   | Compress directories before copying.        |
| --dry-run      | bool      | false   | Simulate the process without changes.       |
| --debug, -d    | bool      | false   | Enable verbose debug output.                |

