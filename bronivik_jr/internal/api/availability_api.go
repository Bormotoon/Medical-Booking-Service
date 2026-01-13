package api

import (
	"encoding/json"
	"net/http"
	"time"

	"bronivik/internal/metrics"
)

const (
	// MaxAvailabilityDaysRange is the maximum number of days allowed in availability request.
	MaxAvailabilityDaysRange = 90
)

// AvailabilityRequest is the request body for POST /api/items/availability.
type AvailabilityRequest struct {
	StartDate string  `json:"start_date"`           // Format: YYYY-MM-DD
	EndDate   string  `json:"end_date"`             // Format: YYYY-MM-DD
	ItemIDs   []int64 `json:"item_ids,omitempty"`   // Optional: filter by item IDs
	CabinetID *int64  `json:"cabinet_id,omitempty"` // Optional: filter by cabinet
	Category  string  `json:"category,omitempty"`   // Optional: filter by category
}

// DateAvailability represents availability for a single date.
type DateAvailability struct {
	Date      string `json:"date"`
	Available bool   `json:"available"`
	Reason    string `json:"reason,omitempty"` // "booked", "maintenance", "reserved"
}

// ItemAvailability represents an item with its availability per date.
type ItemAvailability struct {
	ID           int64              `json:"id"`
	Name         string             `json:"name"`
	Availability []DateAvailability `json:"availability"`
}

// AvailabilityResponse is the response for POST /api/items/availability.
type AvailabilityResponse struct {
	Items  []ItemAvailability `json:"items"`
	Period struct {
		Start string `json:"start"`
		End   string `json:"end"`
	} `json:"period"`
}

// handleItemsAvailability returns availability for items within a date range.
// POST /api/items/availability
func (s *HTTPServer) handleItemsAvailability(w http.ResponseWriter, r *http.Request) {
	metrics.IncHTTP("items_availability")

	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed; use POST")
		return
	}

	var req AvailabilityRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	// Validate required fields
	if req.StartDate == "" || req.EndDate == "" {
		writeError(w, http.StatusBadRequest, "start_date and end_date are required")
		return
	}

	// Parse dates
	startDate, err := time.Parse("2006-01-02", req.StartDate)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid start_date format; expected YYYY-MM-DD")
		return
	}

	endDate, err := time.Parse("2006-01-02", req.EndDate)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid end_date format; expected YYYY-MM-DD")
		return
	}

	// Validate date range
	if startDate.After(endDate) {
		writeError(w, http.StatusBadRequest, "start_date must be before or equal to end_date")
		return
	}

	// Check max range
	days := int(endDate.Sub(startDate).Hours() / 24)
	if days > MaxAvailabilityDaysRange {
		writeError(w, http.StatusBadRequest, "date range exceeds maximum of 90 days")
		return
	}

	// Get items based on filters
	items := s.db.GetItems()
	filteredItems := make([]ItemAvailability, 0)

	for _, item := range items {
		// Skip inactive items
		if !item.IsActive {
			continue
		}

		// Filter by item IDs if specified
		if len(req.ItemIDs) > 0 {
			found := false
			for _, id := range req.ItemIDs {
				if item.ID == id {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		// Build availability for each date in range
		availability := make([]DateAvailability, 0)
		for d := startDate; !d.After(endDate); d = d.AddDate(0, 0, 1) {
			dateStr := d.Format("2006-01-02")

			// Check if permanently reserved
			if item.PermanentReserved {
				availability = append(availability, DateAvailability{
					Date:      dateStr,
					Available: false,
					Reason:    "reserved",
				})
				continue
			}

			// Check availability for this date
			info, err := s.db.GetItemAvailabilityByName(r.Context(), item.Name, d)
			if err != nil {
				// Error getting availability - assume unavailable
				availability = append(availability, DateAvailability{
					Date:      dateStr,
					Available: false,
					Reason:    "error",
				})
				continue
			}

			reason := ""
			if !info.Available {
				reason = "booked"
			}

			availability = append(availability, DateAvailability{
				Date:      dateStr,
				Available: info.Available,
				Reason:    reason,
			})
		}

		filteredItems = append(filteredItems, ItemAvailability{
			ID:           item.ID,
			Name:         item.Name,
			Availability: availability,
		})
	}

	response := AvailabilityResponse{
		Items: filteredItems,
	}
	response.Period.Start = req.StartDate
	response.Period.End = req.EndDate

	writeJSON(w, http.StatusOK, response)
}
