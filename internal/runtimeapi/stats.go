package runtimeapi

import "time"

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
