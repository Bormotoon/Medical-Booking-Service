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

    bookingCanceled = prometheus.NewCounter(
        prometheus.CounterOpts{
            Namespace: "bronivik_crm",
            Name:      "booking_canceled_total",
            Help:      "Count of bookings canceled by users.",
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
        prometheus.MustRegister(bookingCreated, bookingCanceled, managerDecision)
    })
}

func IncBookingCreated(status string) {
    bookingCreated.WithLabelValues(status).Inc()
}

func IncBookingCanceled() {
    bookingCanceled.Inc()
}

func IncManagerDecision(decision string) {
    managerDecision.WithLabelValues(decision).Inc()
}
