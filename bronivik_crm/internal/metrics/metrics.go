package metrics

import (
    "sync"

    "github.com/prometheus/client_golang/prometheus"
)

var (
    once sync.Once

    bookingCreated = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Namespace: "bronivik_crm",
            Name:      "booking_created_total",
            Help:      "Count of bookings created by status.",
        },
        []string{"status"},
    )

    bookingCancelled = prometheus.NewCounter(
        prometheus.CounterOpts{
            Namespace: "bronivik_crm",
            Name:      "booking_cancelled_total",
            Help:      "Count of bookings cancelled by users.",
        },
    )

    managerDecision = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Namespace: "bronivik_crm",
            Name:      "manager_decision_total",
            Help:      "Count of manager decisions over bookings.",
        },
        []string{"decision"},
    )
)

// Register registers metrics (idempotent).
func Register() {
    once.Do(func() {
        prometheus.MustRegister(bookingCreated, bookingCancelled, managerDecision)
    })
}

func IncBookingCreated(status string) {
    bookingCreated.WithLabelValues(status).Inc()
}

func IncBookingCancelled() {
    bookingCancelled.Inc()
}

func IncManagerDecision(decision string) {
    managerDecision.WithLabelValues(decision).Inc()
}
