package utils

import (
	"fmt"
	"time"
)

func ConvertTimeStringToTime(timeString string) (time.Time, error) {
	latest_backup_time, err := time.Parse("2006-01-02_15-04-05", timeString)
	if err != nil {
		return latest_backup_time, fmt.Errorf("Failed to parse timestamp of latest backup: %s ", err)
	}
	return latest_backup_time, nil
}
