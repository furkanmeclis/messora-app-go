// Package metrics provides the Prometheus registry and platform collectors.
package metrics

import (
	"context"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	goredis "github.com/redis/go-redis/v9"
)

// NewRegistry returns a registry with Go runtime and process collectors and a
// build-info gauge.
func NewRegistry(version, role string) *prometheus.Registry {
	reg := prometheus.NewRegistry()
	reg.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)
	info := prometheus.NewGauge(prometheus.GaugeOpts{
		Name:        "messora_build_info",
		Help:        "Build information; value is always 1.",
		ConstLabels: prometheus.Labels{"version": version, "role": role},
	})
	info.Set(1)
	reg.MustRegister(info)
	return reg
}

// Handler serves the registry.
func Handler(reg *prometheus.Registry) http.Handler {
	return promhttp.HandlerFor(reg, promhttp.HandlerOpts{Registry: reg})
}

// PoolCollector exports pgxpool statistics.
type PoolCollector struct {
	pool                                 *pgxpool.Pool
	total, idle, acquired, max, acquires *prometheus.Desc
	waitDur                              *prometheus.Desc
}

// NewPoolCollector creates a collector for pool.
func NewPoolCollector(pool *pgxpool.Pool) *PoolCollector {
	d := func(name, help string) *prometheus.Desc {
		return prometheus.NewDesc("messora_db_pool_"+name, help, nil, nil)
	}
	return &PoolCollector{
		pool:     pool,
		total:    d("connections", "Open connections."),
		idle:     d("idle_connections", "Idle connections."),
		acquired: d("acquired_connections", "Connections currently in use."),
		max:      d("max_connections", "Configured maximum."),
		acquires: d("acquires_total", "Successful acquires."),
		waitDur:  d("acquire_wait_seconds_total", "Time spent waiting for a connection."),
	}
}

// Describe implements prometheus.Collector.
func (c *PoolCollector) Describe(ch chan<- *prometheus.Desc) {
	for _, d := range []*prometheus.Desc{c.total, c.idle, c.acquired, c.max, c.acquires, c.waitDur} {
		ch <- d
	}
}

// Collect implements prometheus.Collector.
func (c *PoolCollector) Collect(ch chan<- prometheus.Metric) {
	s := c.pool.Stat()
	ch <- prometheus.MustNewConstMetric(c.total, prometheus.GaugeValue, float64(s.TotalConns()))
	ch <- prometheus.MustNewConstMetric(c.idle, prometheus.GaugeValue, float64(s.IdleConns()))
	ch <- prometheus.MustNewConstMetric(c.acquired, prometheus.GaugeValue, float64(s.AcquiredConns()))
	ch <- prometheus.MustNewConstMetric(c.max, prometheus.GaugeValue, float64(s.MaxConns()))
	ch <- prometheus.MustNewConstMetric(c.acquires, prometheus.CounterValue, float64(s.AcquireCount()))
	ch <- prometheus.MustNewConstMetric(c.waitDur, prometheus.CounterValue, s.AcquireDuration().Seconds())
}

// NewRedisUpGauge reports 1 when Redis answers PING at scrape time.
func NewRedisUpGauge(rdb goredis.UniversalClient) prometheus.GaugeFunc {
	return prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Name: "messora_redis_up", Help: "1 if Redis answered PING during the scrape.",
	}, func() float64 {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if rdb.Ping(ctx).Err() != nil {
			return 0
		}
		return 1
	})
}
