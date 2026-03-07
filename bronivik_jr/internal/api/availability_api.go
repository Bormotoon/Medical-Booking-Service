package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"bronivik/internal/metrics"
	"bronivik/internal/models"
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
	CabinetID    *int64             `json:"cabinet_id,omitempty"`
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

	startDate, endDate, err := s.validateAvailabilityRequest(&req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Get items based on filters
	items, err := s.db.GetCurrentItems(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load items")
		return
	}
	filtered := make([]*models.Item, 0, len(items))
	for _, item := range items {
		if !s.shouldIncludeItem(item, &req) {
			continue
		}
		filtered = append(filtered, item)
	}

	filteredItems, err := s.buildItemAvailability(r.Context(), filtered, startDate, endDate)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to calculate availability")
		return
	}

	response := AvailabilityResponse{
		Items: filteredItems,
	}
	response.Period.Start = req.StartDate
	response.Period.End = req.EndDate

	writeJSON(w, http.StatusOK, response)
}

func (s *HTTPServer) validateAvailabilityRequest(req *AvailabilityRequest) (start, end time.Time, err error) {
	if req.StartDate == "" || req.EndDate == "" {
		return time.Time{}, time.Time{}, fmt.Errorf("start_date and end_date are required")
	}

	startDate, err := time.Parse("2006-01-02", req.StartDate)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid start_date format; expected YYYY-MM-DD")
	}

	endDate, err := time.Parse("2006-01-02", req.EndDate)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid end_date format; expected YYYY-MM-DD")
	}

	if startDate.After(endDate) {
		return time.Time{}, time.Time{}, fmt.Errorf("start_date must be before or equal to end_date")
	}

	days := int(endDate.Sub(startDate).Hours() / 24)
	if days > MaxAvailabilityDaysRange {
		return time.Time{}, time.Time{}, fmt.Errorf("date range exceeds maximum of 90 days")
	}

	return startDate, endDate, nil
}

func (s *HTTPServer) shouldIncludeItem(item *models.Item, req *AvailabilityRequest) bool {
	if !item.IsActive {
		return false
	}

	if req.CabinetID != nil {
		if item.CabinetID == nil || *item.CabinetID != *req.CabinetID {
			return false
		}
	}

	if len(req.ItemIDs) > 0 {
		found := false
		for _, id := range req.ItemIDs {
			if item.ID == id {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	return true
}

func (s *HTTPServer) getItemAvailabilityDates(ctx context.Context, item *models.Item, start, end time.Time) []DateAvailability {
	availability := make([]DateAvailability, 0)
	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		dateStr := d.Format("2006-01-02")

		if item.PermanentReserved {
			availability = append(availability, DateAvailability{
				Date:      dateStr,
				Available: false,
				Reason:    "reserved",
			})
			continue
		}

		info, err := s.db.GetItemAvailabilityByName(ctx, item.Name, d)
		if err != nil {
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
	return availability
}

func (s *HTTPServer) buildItemAvailability(
	ctx context.Context,
	items []*models.Item,
	start, end time.Time,
) ([]ItemAvailability, error) {
	if len(items) == 0 {
		return nil, nil
	}

	dateLabels := make([]string, 0)
	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		dateLabels = append(dateLabels, d.Format("2006-01-02"))
	}

	bookedCounts, err := s.loadBookedCountsByItemAndDate(ctx, items, start, end)
	if err != nil {
		return nil, err
	}

	result := make([]ItemAvailability, 0, len(items))
	for _, item := range items {
		itemAvailability := ItemAvailability{
			ID:        item.ID,
			Name:      item.Name,
			CabinetID: item.CabinetID,
		}

		perDate := make([]DateAvailability, 0, len(dateLabels))
		for _, dateLabel := range dateLabels {
			if item.PermanentReserved {
				perDate = append(perDate, DateAvailability{
					Date:      dateLabel,
					Available: false,
					Reason:    "reserved",
				})
				continue
			}

			booked := bookedCounts[item.ID][dateLabel]
			available := booked < int(item.TotalQuantity)
			reason := ""
			if !available {
				reason = "booked"
			}

			perDate = append(perDate, DateAvailability{
				Date:      dateLabel,
				Available: available,
				Reason:    reason,
			})
		}

		itemAvailability.Availability = perDate
		result = append(result, itemAvailability)
	}

	return result, nil
}

func (s *HTTPServer) loadBookedCountsByItemAndDate(
	ctx context.Context,
	items []*models.Item,
	start, end time.Time,
) (map[int64]map[string]int, error) {
	counts := make(map[int64]map[string]int, len(items))
	itemIDs := make([]any, 0, len(items))
	queryItemIDs := make([]int64, 0, len(items))
	for _, item := range items {
		counts[item.ID] = make(map[string]int)
		if item.PermanentReserved {
			continue
		}
		queryItemIDs = append(queryItemIDs, item.ID)
		itemIDs = append(itemIDs, item.ID)
	}
	if len(queryItemIDs) == 0 {
		return counts, nil
	}

	placeholders := strings.TrimRight(strings.Repeat("?,", len(queryItemIDs)), ",")
	query := fmt.Sprintf(`
		SELECT item_id, date(date) AS booking_date, COUNT(*) AS booked_count
		FROM bookings
		WHERE item_id IN (%s)
		  AND date(date) BETWEEN date(?) AND date(?)
		  AND status NOT IN (?, ?)
		GROUP BY item_id, booking_date
	`, placeholders)

	args := append(itemIDs,
		start.Format("2006-01-02"),
		end.Format("2006-01-02"),
		models.StatusCanceled,
		models.StatusRejectedLegacy,
	)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			itemID int64
			date   string
			booked int
		)
		if err := rows.Scan(&itemID, &date, &booked); err != nil {
			return nil, err
		}
		counts[itemID][date] = booked
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return counts, nil
}
