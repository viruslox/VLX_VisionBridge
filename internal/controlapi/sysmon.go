package controlapi

import (
	"bufio"
	"bytes"
	"os"
	"strconv"
	"strings"
	"sync"
)

var (
	cpuMutex  sync.Mutex
	prevTotal uint64
	prevIdle  uint64
)

type SystemUsage struct {
	CPU  float64 `json:"cpu"`
	RAM  float64 `json:"ram"`
	Swap float64 `json:"swap"`
}

func GetSystemUsage() SystemUsage {
	var usage SystemUsage

	// CPU usage
	dataStat, err := os.ReadFile("/proc/stat")
	if err == nil {
		lines := strings.Split(string(dataStat), "\n")
		if len(lines) > 0 {
			fields := strings.Fields(lines[0])
			if len(fields) >= 5 && fields[0] == "cpu" {
				var total uint64
				var idle uint64

				for i := 1; i < len(fields); i++ {
					val, _ := strconv.ParseUint(fields[i], 10, 64)
					total += val
					if i == 4 || i == 5 { // idle and iowait
						idle += val
					}
				}

				cpuMutex.Lock()
				deltaTotal := total - prevTotal
				deltaIdle := idle - prevIdle

				prevTotal = total
				prevIdle = idle
				cpuMutex.Unlock()

				if deltaTotal > 0 {
					usage.CPU = float64(deltaTotal-deltaIdle) / float64(deltaTotal) * 100.0
				}
			}
		}
	}

	// Mem usage
	dataMem, err := os.ReadFile("/proc/meminfo")
	if err == nil {
		scanner := bufio.NewScanner(bytes.NewReader(dataMem))
		mem := make(map[string]float64)
		for scanner.Scan() {
			line := scanner.Text()
			idx := strings.IndexByte(line, ':')
			if idx == -1 {
				continue
			}
			key := line[:idx]
			valStr := strings.TrimSpace(line[idx+1:])
			valEnd := strings.IndexByte(valStr, ' ')
			if valEnd != -1 {
				valStr = valStr[:valEnd]
			}
			val, err := strconv.ParseFloat(valStr, 64)
			if err == nil {
				mem[key] = val
			}
		}

		if mem["MemTotal"] > 0 {
			if memAvailable, ok := mem["MemAvailable"]; ok {
				usage.RAM = (mem["MemTotal"] - memAvailable) / mem["MemTotal"] * 100.0
			} else {
				usage.RAM = (mem["MemTotal"] - mem["MemFree"] - mem["Buffers"] - mem["Cached"]) / mem["MemTotal"] * 100.0
			}
		}

		if mem["SwapTotal"] > 0 {
			usage.Swap = (mem["SwapTotal"] - mem["SwapFree"]) / mem["SwapTotal"] * 100.0
		}
	}

	return usage
}
