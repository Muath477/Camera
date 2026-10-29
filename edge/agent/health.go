package agent

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// DiskUsage returns total and used bytes of the filesystem holding path.
func DiskUsage(path string) (total, used uint64, err error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return 0, 0, err
	}
	total = st.Blocks * uint64(st.Bsize)
	free := st.Bavail * uint64(st.Bsize)
	return total, total - free, nil
}

// CPUTemp returns the hottest thermal zone in Celsius, or 0 if none is exposed.
func CPUTemp() float64 {
	zones, _ := filepath.Glob("/sys/class/thermal/thermal_zone*/temp")
	max := 0.0
	for _, z := range zones {
		raw, err := os.ReadFile(z)
		if err != nil {
			continue
		}
		milli, err := strconv.ParseFloat(strings.TrimSpace(string(raw)), 64)
		if err != nil {
			continue
		}
		if c := milli / 1000; c > max {
			max = c
		}
	}
	return max
}

// RetentionDays estimates how many days of continuous recording fit on the
// recordings disk given each camera's measured bitrate in MB per hour.
// Ten percent is held back because Frigate starts deleting before the disk
// is completely full.
func RetentionDays(diskTotalBytes uint64, mbPerHour []float64) float64 {
	var sum float64
	for _, b := range mbPerHour {
		sum += b
	}
	if sum <= 0 {
		return 0
	}
	usableMB := float64(diskTotalBytes) * 0.9 / (1 << 20)
	return usableMB / sum / 24
}

// KbpsToMBPerHour converts a stream bitrate to megabytes of storage per hour.
func KbpsToMBPerHour(kbps float64) float64 {
	return kbps * 1000 / 8 * 3600 / (1 << 20)
}
