package reminders

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Metrics holds Prometheus metrics for the reminder system.
type Metrics struct {
	// RemindersSentTotal is the total number of reminders sent.
	RemindersSentTotal *prometheus.CounterVec

	// RemindersQueueSize is the current number of pending reminders.
	RemindersQueueSize prometheus.Gauge

	// ReminderSendDuration is the time to send a reminder.
	ReminderSendDuration prometheus.Histogram

	// RemindersCleanedUp is the total number of reminders cleaned up.
	RemindersCleanedUp prometheus.Counter

	// ReminderRetries is the total number of retry attempts.
	ReminderRetries prometheus.Counter

	// RateLimitWaits is the total number of rate limit waits.
	RateLimitWaits prometheus.Counter
}

// NewMetrics creates and registers Prometheus metrics for reminders.
func NewMetrics(namespace string) *Metrics {
	return &Metrics{
		RemindersSentTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "reminders_sent_total",
				Help:      "Total number of reminders sent",
			},
			[]string{"status", "reminder_type"},
		),

		RemindersQueueSize: promauto.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "reminders_queue_size",
				Help:      "Current number of pending reminders",
			},
		),

		ReminderSendDuration: promauto.NewHistogram(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Name:      "reminder_send_duration_seconds",
				Help:      "Time to send a reminder",
				Buckets:   []float64{.01, .05, .1, .5, 1, 2, 5},
			},
		),

		RemindersCleanedUp: promauto.NewCounter(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "reminders_cleaned_up_total",
				Help:      "Total number of reminders cleaned up",
			},
		),

		ReminderRetries: promauto.NewCounter(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "reminder_retries_total",
				Help:      "Total number of retry attempts",
			},
		),

		RateLimitWaits: promauto.NewCounter(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "rate_limit_waits_total",
				Help:      "Total number of rate limit waits",
			},
		),
	}
}

// IncSent increments the sent counter for a given status and type.
func (m *Metrics) IncSent(status string, reminderType ReminderType) {
	m.RemindersSentTotal.WithLabelValues(status, string(reminderType)).Inc()
}

// SetQueueSize sets the current queue size.
func (m *Metrics) SetQueueSize(size int64) {
	m.RemindersQueueSize.Set(float64(size))
}

// ObserveSendDuration records the time taken to send a reminder.
func (m *Metrics) ObserveSendDuration(seconds float64) {
	m.ReminderSendDuration.Observe(seconds)
}

// IncCleanedUp increments the cleanup counter.
func (m *Metrics) IncCleanedUp(count int64) {
	m.RemindersCleanedUp.Add(float64(count))
}

// IncRetries increments the retry counter.
func (m *Metrics) IncRetries() {
	m.ReminderRetries.Inc()
}

// IncRateLimitWaits increments the rate limit wait counter.
func (m *Metrics) IncRateLimitWaits() {
	m.RateLimitWaits.Inc()
}
