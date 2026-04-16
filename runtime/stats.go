package runtime

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	goruntime "runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

var processStartedAt = time.Now()

type ByteStat struct {
	Bytes uint64 `json:"bytes"`
	Human string `json:"human"`
}

type CPUStats struct {
	NumCPU             int     `json:"num_cpu"`
	GOMAXPROCS         int     `json:"gomaxprocs"`
	NumCgoCall         int64   `json:"num_cgo_call"`
	UserSeconds        float64 `json:"user_seconds"`
	SystemSeconds      float64 `json:"system_seconds"`
	TotalSeconds       float64 `json:"total_seconds"`
	ProcessPercent     float64 `json:"process_percent"`
	SampleWindowSecond float64 `json:"sample_window_seconds"`
}

type MemoryStats struct {
	CurrentHeap  ByteStat `json:"current_heap"`
	TotalAlloc   ByteStat `json:"total_alloc"`
	Sys          ByteStat `json:"sys"`
	HeapSys      ByteStat `json:"heap_sys"`
	HeapInUse    ByteStat `json:"heap_in_use"`
	HeapIdle     ByteStat `json:"heap_idle"`
	HeapReleased ByteStat `json:"heap_released"`
	StackInUse   ByteStat `json:"stack_in_use"`
	StackSys     ByteStat `json:"stack_sys"`
	HeapObjects  uint64   `json:"heap_objects"`
}

type GCStats struct {
	NumGC             uint32   `json:"num_gc"`
	NextGC            ByteStat `json:"next_gc"`
	Pressure          float64  `json:"pressure"`
	PressurePercent   float64  `json:"pressure_percent"`
	GCCPUFraction     float64  `json:"gc_cpu_fraction"`
	LastGCUnixNano    uint64   `json:"last_gc_unix_nano"`
	LastGCAt          string   `json:"last_gc_at,omitempty"`
	PauseTotalNS      uint64   `json:"pause_total_ns"`
	LastPauseNS       uint64   `json:"last_pause_ns"`
	ForcedGCCount     uint32   `json:"forced_gc_count"`
	CompletedCyclePct float64  `json:"completed_cycle_percent"`
}

type DiskStats struct {
	Path             string   `json:"path"`
	Total            ByteStat `json:"total"`
	Used             ByteStat `json:"used"`
	Free             ByteStat `json:"free"`
	Available        ByteStat `json:"available"`
	UsedPercent      float64  `json:"used_percent"`
	FreePercent      float64  `json:"free_percent"`
	AvailablePercent float64  `json:"available_percent"`
	Error            string   `json:"error,omitempty"`
}

type OSProcessStats struct {
	MaxRSS                     ByteStat `json:"max_rss"`
	MinorPageFaults            int64    `json:"minor_page_faults"`
	MajorPageFaults            int64    `json:"major_page_faults"`
	InputBlocks                int64    `json:"input_blocks"`
	OutputBlocks               int64    `json:"output_blocks"`
	VoluntaryContextSwitches   int64    `json:"voluntary_context_switches"`
	InvoluntaryContextSwitches int64    `json:"involuntary_context_switches"`
	Error                      string   `json:"error,omitempty"`
}

type ProfileLinks struct {
	Index        string `json:"index"`
	Cmdline      string `json:"cmdline"`
	CPU          string `json:"cpu"`
	Trace        string `json:"trace"`
	Symbol       string `json:"symbol"`
	Heap         string `json:"heap"`
	Goroutine    string `json:"goroutine"`
	Allocs       string `json:"allocs"`
	Block        string `json:"block"`
	Mutex        string `json:"mutex"`
	ThreadCreate string `json:"threadcreate"`
}

type ProcessStats struct {
	PID           int       `json:"pid"`
	PPID          int       `json:"ppid"`
	Executable    string    `json:"executable"`
	WorkingDir    string    `json:"working_dir"`
	StartedAt     time.Time `json:"started_at"`
	UptimeSeconds float64   `json:"uptime_seconds"`
}

type GoStats struct {
	Version    string `json:"version"`
	Goroutines int    `json:"goroutines"`
	NumCPU     int    `json:"num_cpu"`
	GOMAXPROCS int    `json:"gomaxprocs"`
	NumCgoCall int64  `json:"num_cgo_call"`
}

type PlatformStatsResponse struct {
	Timestamp           time.Time      `json:"timestamp"`
	AppID               string         `json:"app_id"`
	APIBaseURL          string         `json:"api_base_url"`
	RegisteredEndpoints int            `json:"registered_endpoints"`
	RegisteredCronJobs  int            `json:"registered_cron_jobs"`
	Process             ProcessStats   `json:"process"`
	Go                  GoStats        `json:"go"`
	CPU                 CPUStats       `json:"cpu"`
	Memory              MemoryStats    `json:"memory"`
	GC                  GCStats        `json:"gc"`
	Disk                DiskStats      `json:"disk"`
	OS                  OSProcessStats `json:"os"`
	Profiles            ProfileLinks   `json:"profiles"`
}

type cpuSample struct {
	at           time.Time
	totalSeconds float64
}

var (
	cpuSampleMu   sync.Mutex
	lastCPUSample cpuSample
)

func collectPlatformStats() PlatformStatsResponse {
	now := time.Now()
	meta := Meta()
	process := collectProcessStats()
	mem := collectGoMemStats()
	cpu, osStats := collectCPUAndOSStats(now)
	disk := collectDiskStats(process.WorkingDir)

	return PlatformStatsResponse{
		Timestamp:           now,
		AppID:               meta.AppID,
		APIBaseURL:          meta.APIBaseURL,
		RegisteredEndpoints: len(listEndpoints()),
		RegisteredCronJobs:  len(listCronJobs()),
		Process:             process,
		Go: GoStats{
			Version:    goruntime.Version(),
			Goroutines: goruntime.NumGoroutine(),
			NumCPU:     goruntime.NumCPU(),
			GOMAXPROCS: goruntime.GOMAXPROCS(0),
			NumCgoCall: goruntime.NumCgoCall(),
		},
		CPU:      cpu,
		Memory:   mem.memory,
		GC:       mem.gc,
		Disk:     disk,
		OS:       osStats,
		Profiles: profileLinks(meta.APIBaseURL),
	}
}

type goMemSnapshot struct {
	memory MemoryStats
	gc     GCStats
}

func collectGoMemStats() goMemSnapshot {
	var ms goruntime.MemStats
	goruntime.ReadMemStats(&ms)

	var lastPause uint64
	if ms.NumGC > 0 {
		lastPause = ms.PauseNs[(ms.NumGC-1)%uint32(len(ms.PauseNs))]
	}

	pressure := percentFraction(float64(ms.HeapAlloc), float64(ms.NextGC))
	lastGCAt := ""
	if ms.LastGC != 0 {
		lastGCAt = time.Unix(0, int64(ms.LastGC)).UTC().Format(time.RFC3339Nano)
	}

	return goMemSnapshot{
		memory: MemoryStats{
			CurrentHeap:  humanBytes(ms.HeapAlloc),
			TotalAlloc:   humanBytes(ms.TotalAlloc),
			Sys:          humanBytes(ms.Sys),
			HeapSys:      humanBytes(ms.HeapSys),
			HeapInUse:    humanBytes(ms.HeapInuse),
			HeapIdle:     humanBytes(ms.HeapIdle),
			HeapReleased: humanBytes(ms.HeapReleased),
			StackInUse:   humanBytes(ms.StackInuse),
			StackSys:     humanBytes(ms.StackSys),
			HeapObjects:  ms.HeapObjects,
		},
		gc: GCStats{
			NumGC:             ms.NumGC,
			NextGC:            humanBytes(ms.NextGC),
			Pressure:          pressure,
			PressurePercent:   pressure * 100,
			GCCPUFraction:     ms.GCCPUFraction,
			LastGCUnixNano:    ms.LastGC,
			LastGCAt:          lastGCAt,
			PauseTotalNS:      ms.PauseTotalNs,
			LastPauseNS:       lastPause,
			ForcedGCCount:     ms.NumForcedGC,
			CompletedCyclePct: completedCyclePercent(ms),
		},
	}
}

func completedCyclePercent(ms goruntime.MemStats) float64 {
	if ms.NextGC == 0 {
		return 0
	}
	return float64(ms.HeapAlloc) / float64(ms.NextGC) * 100
}

func collectProcessStats() ProcessStats {
	exe, _ := os.Executable()
	if exe != "" {
		exe = filepath.Clean(exe)
	}
	wd, _ := os.Getwd()
	if wd != "" {
		wd = filepath.Clean(wd)
	}
	return ProcessStats{
		PID:           os.Getpid(),
		PPID:          os.Getppid(),
		Executable:    exe,
		WorkingDir:    wd,
		StartedAt:     processStartedAt.UTC(),
		UptimeSeconds: time.Since(processStartedAt).Seconds(),
	}
}

func collectCPUAndOSStats(now time.Time) (CPUStats, OSProcessStats) {
	usage, err := readProcessUsage()
	cpu := CPUStats{
		NumCPU:     goruntime.NumCPU(),
		GOMAXPROCS: goruntime.GOMAXPROCS(0),
		NumCgoCall: goruntime.NumCgoCall(),
	}
	osStats := OSProcessStats{}
	if err != nil {
		osStats.Error = err.Error()
		return cpu, osStats
	}

	totalSeconds := usage.UserSeconds + usage.SystemSeconds
	cpu.UserSeconds = usage.UserSeconds
	cpu.SystemSeconds = usage.SystemSeconds
	cpu.TotalSeconds = totalSeconds
	cpu.ProcessPercent, cpu.SampleWindowSecond = sampleCPUPercent(now, totalSeconds)

	osStats.MaxRSS = humanBytes(usage.MaxRSSBytes)
	osStats.MinorPageFaults = usage.MinorPageFaults
	osStats.MajorPageFaults = usage.MajorPageFaults
	osStats.InputBlocks = usage.InputBlocks
	osStats.OutputBlocks = usage.OutputBlocks
	osStats.VoluntaryContextSwitches = usage.VoluntaryContextSwitches
	osStats.InvoluntaryContextSwitches = usage.InvoluntaryContextSwitches
	return cpu, osStats
}

func sampleCPUPercent(now time.Time, totalSeconds float64) (float64, float64) {
	cpuSampleMu.Lock()
	defer cpuSampleMu.Unlock()
	if now.IsZero() {
		now = time.Now()
	}
	window := 0.0
	percent := 0.0
	if !lastCPUSample.at.IsZero() {
		window = now.Sub(lastCPUSample.at).Seconds()
		if window > 0 {
			percent = ((totalSeconds - lastCPUSample.totalSeconds) / window) * 100
			if percent < 0 {
				percent = 0
			}
		}
	}
	lastCPUSample = cpuSample{at: now, totalSeconds: totalSeconds}
	return percent, window
}

func collectDiskStats(path string) DiskStats {
	if path == "" {
		wd, _ := os.Getwd()
		path = wd
	}
	if path == "" {
		return DiskStats{Error: "working directory unavailable"}
	}
	stats, err := readDiskStats(path)
	if err != nil {
		return DiskStats{
			Path:  path,
			Error: err.Error(),
		}
	}
	return DiskStats{
		Path:             path,
		Total:            humanBytes(stats.TotalBytes),
		Used:             humanBytes(stats.UsedBytes),
		Free:             humanBytes(stats.FreeBytes),
		Available:        humanBytes(stats.AvailableBytes),
		UsedPercent:      percentFraction(float64(stats.UsedBytes), float64(stats.TotalBytes)) * 100,
		FreePercent:      percentFraction(float64(stats.FreeBytes), float64(stats.TotalBytes)) * 100,
		AvailablePercent: percentFraction(float64(stats.AvailableBytes), float64(stats.TotalBytes)) * 100,
	}
}

func percentFraction(numerator, denominator float64) float64 {
	if denominator <= 0 {
		return 0
	}
	return numerator / denominator
}

func humanBytes(bytes uint64) ByteStat {
	return ByteStat{
		Bytes: bytes,
		Human: humanizeBytes(bytes),
	}
}

func humanizeBytes(bytes uint64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := uint64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return trimFloat(float64(bytes)/float64(div), 1) + " " + string("KMGTPE"[exp]) + "iB"
}

func trimFloat(value float64, precision int) string {
	raw := strconv.FormatFloat(value, 'f', precision, 64)
	raw = strings.TrimRight(raw, "0")
	raw = strings.TrimRight(raw, ".")
	if raw == "" {
		return "0"
	}
	return raw
}

func profileLinks(baseURL string) ProfileLinks {
	resolve := func(path string) string {
		if strings.TrimSpace(baseURL) == "" {
			return path
		}
		base, err := url.Parse(baseURL)
		if err != nil {
			return path
		}
		ref, err := url.Parse(path)
		if err != nil {
			return path
		}
		return base.ResolveReference(ref).String()
	}
	return ProfileLinks{
		Index:        resolve("/debug/pprof/"),
		Cmdline:      resolve("/debug/pprof/cmdline"),
		CPU:          resolve("/debug/pprof/profile?seconds=30"),
		Trace:        resolve("/debug/pprof/trace?seconds=5"),
		Symbol:       resolve("/debug/pprof/symbol"),
		Heap:         resolve("/debug/pprof/heap"),
		Goroutine:    resolve("/debug/pprof/goroutine?debug=1"),
		Allocs:       resolve("/debug/pprof/allocs"),
		Block:        resolve("/debug/pprof/block"),
		Mutex:        resolve("/debug/pprof/mutex"),
		ThreadCreate: resolve("/debug/pprof/threadcreate"),
	}
}
