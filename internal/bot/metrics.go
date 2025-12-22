package bot

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Metrics структура для метрик Prometheus
type Metrics struct {
	MessagesProcessed    prometheus.Counter
	CommandsProcessed    prometheus.Counter
	CommandDuration      *prometheus.HistogramVec
	CalculationsTotal    *prometheus.CounterVec
	ErrorsTotal          prometheus.Counter
	UsersTotal           prometheus.Gauge
	UpdateProcessingTime prometheus.Histogram
}

// NewMetrics создает новые метрики
func NewMetrics() *Metrics {
	return &Metrics{
		MessagesProcessed: promauto.NewCounter(prometheus.CounterOpts{
			Name: "telegram_bot_messages_processed_total",
			Help: "Total number of processed messages",
		}),

		CommandsProcessed: promauto.NewCounter(prometheus.CounterOpts{
			Name: "telegram_bot_commands_processed_total",
			Help: "Total number of processed commands",
		}),

		CommandDuration: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "telegram_bot_command_duration_seconds",
			Help:    "Duration of command processing",
			Buckets: prometheus.DefBuckets,
		}, []string{"command"}),

		CalculationsTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "telegram_bot_calculations_total",
			Help: "Total number of calculations by type",
		}, []string{"type"}),

		ErrorsTotal: promauto.NewCounter(prometheus.CounterOpts{
			Name: "telegram_bot_errors_total",
			Help: "Total number of errors",
		}),

		UsersTotal: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "telegram_bot_users_total",
			Help: "Total number of users",
		}),

		UpdateProcessingTime: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:    "telegram_bot_update_processing_time_seconds",
			Help:    "Time spent processing updates",
			Buckets: prometheus.DefBuckets,
		}),
	}
}
