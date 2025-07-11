package metrics

import (
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/disk"
	"github.com/shirou/gopsutil/mem"
)

var (
	CPUUsageGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "goedgeinfer_cpu_usage_percent",
		Help: "System CPU usage percent",
	})
	MemUsageGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "goedgeinfer_mem_usage_bytes",
		Help: "System memory usage in bytes",
	})
	DiskUsageGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "goedgeinfer_disk_usage_bytes",
		Help: "System disk usage in bytes (root fs)",
	})
	registerOnce sync.Once
)

func RegisterSysMetrics() {
	registerOnce.Do(func() {
		prometheus.MustRegister(CPUUsageGauge, MemUsageGauge, DiskUsageGauge)
		go func() {
			for {
				updateSysMetrics()
				time.Sleep(5 * time.Second)
			}
		}()
	})
}

func updateSysMetrics() {
	if cpuPercents, err := cpu.Percent(0, false); err == nil && len(cpuPercents) > 0 {
		CPUUsageGauge.Set(cpuPercents[0])
	}
	if vm, err := mem.VirtualMemory(); err == nil {
		MemUsageGauge.Set(float64(vm.Used))
	}
	if du, err := disk.Usage("/"); err == nil {
		DiskUsageGauge.Set(float64(du.Used))
	}
}
