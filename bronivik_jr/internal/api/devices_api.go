package api

import (
	"encoding/json"
	"net/http"
	"time"

	"bronivik/internal/metrics"
)

// DeviceResponse represents a device in API response.
type DeviceResponse struct {
	ID                int64  `json:"id"`
	Name              string `json:"name"`
	Description       string `json:"description,omitempty"`
	Available         bool   `json:"available"`
	PermanentReserved bool   `json:"permanent_reserved"`
}

// BookDeviceRequest is the request body for booking a device.
type BookDeviceRequest struct {
	DeviceID          int64  `json:"device_id"`
	DeviceName        string `json:"device_name,omitempty"` // Alternative to device_id
	Date              string `json:"date"`                  // Format: YYYY-MM-DD
	StartTime         string `json:"start_time,omitempty"`  // Format: HH:MM (optional)
	EndTime           string `json:"end_time,omitempty"`    // Format: HH:MM (optional)
	ExternalBookingID string `json:"external_booking_id"`   // ID from bronivik_crm
	ClientName        string `json:"client_name,omitempty"`
	ClientPhone       string `json:"client_phone,omitempty"`
}

// BookDeviceResponse is the response for booking a device.
type BookDeviceResponse struct {
	Success   bool   `json:"success"`
	BookingID int64  `json:"booking_id,omitempty"`
	Error     string `json:"error,omitempty"`
}

// handleDevices returns list of devices with availability for optional date.
// GET /api/devices?date=YYYY-MM-DD&include_reserved=true
func (s *HTTPServer) handleDevices(w http.ResponseWriter, r *http.Request) {
	metrics.IncHTTP("devices")
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Parse optional date parameter
	dateStr := r.URL.Query().Get("date")
	var date time.Time
	var err error
	if dateStr != "" {
		date, err = time.Parse("2006-01-02", dateStr)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid date format; expected YYYY-MM-DD")
			return
		}
	} else {
		date = time.Now()
	}

	// Include permanently reserved devices?
	includeReserved := r.URL.Query().Get("include_reserved") == "true"

	// Get all items from cache
	items := s.db.GetItems()

	var devices []DeviceResponse
	for _, item := range items {
		if !item.IsActive {
			continue
		}
		// Skip permanently reserved unless explicitly requested
		if item.PermanentReserved && !includeReserved {
			continue
		}

		// Check availability for the date
		info, err := s.db.GetItemAvailabilityByName(r.Context(), item.Name, date)
		available := false
		if err == nil && info != nil {
			available = info.Available
		}

		devices = append(devices, DeviceResponse{
			ID:                item.ID,
			Name:              item.Name,
			Description:       item.Description,
			Available:         available,
			PermanentReserved: item.PermanentReserved,
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{"devices": devices})
}

// handleBookDevice creates a booking for a device from bronivik_crm.
// POST /api/book-device
func (s *HTTPServer) handleBookDevice(w http.ResponseWriter, r *http.Request) {
	metrics.IncHTTP("book_device")
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req BookDeviceRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, BookDeviceResponse{
			Success: false,
			Error:   "invalid JSON body",
		})
		return
	}

	// Validate required fields
	if req.Date == "" {
		writeJSON(w, http.StatusBadRequest, BookDeviceResponse{
			Success: false,
			Error:   "date is required",
		})
		return
	}

	if req.ExternalBookingID == "" {
		writeJSON(w, http.StatusBadRequest, BookDeviceResponse{
			Success: false,
			Error:   "external_booking_id is required",
		})
		return
	}

	// Parse date
	date, err := time.Parse("2006-01-02", req.Date)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, BookDeviceResponse{
			Success: false,
			Error:   "invalid date format; expected YYYY-MM-DD",
		})
		return
	}

	// Find device by ID or name
	var deviceID int64
	var deviceName string

	if req.DeviceID > 0 {
		item, err := s.db.GetItemByID(r.Context(), req.DeviceID)
		if err != nil {
			writeJSON(w, http.StatusNotFound, BookDeviceResponse{
				Success: false,
				Error:   "device not found",
			})
			return
		}
		deviceID = item.ID
		deviceName = item.Name
	} else if req.DeviceName != "" {
		item, err := s.db.GetItemByName(r.Context(), req.DeviceName)
		if err != nil {
			writeJSON(w, http.StatusNotFound, BookDeviceResponse{
				Success: false,
				Error:   "device not found",
			})
			return
		}
		deviceID = item.ID
		deviceName = item.Name
	} else {
		writeJSON(w, http.StatusBadRequest, BookDeviceResponse{
			Success: false,
			Error:   "device_id or device_name is required",
		})
		return
	}

	// Check availability
	info, err := s.db.GetItemAvailabilityByName(r.Context(), deviceName, date)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, BookDeviceResponse{
			Success: false,
			Error:   "failed to check availability",
		})
		return
	}

	if !info.Available {
		writeJSON(w, http.StatusConflict, BookDeviceResponse{
			Success: false,
			Error:   "device not available for the selected date",
		})
		return
	}

	// Create booking via external API booking
	bookingID, err := s.db.CreateExternalBooking(
		r.Context(),
		deviceID,
		deviceName,
		date,
		req.ExternalBookingID,
		req.ClientName,
		req.ClientPhone,
	)
	if err != nil {
		s.log.Error().Err(err).
			Int64("device_id", deviceID).
			Str("date", req.Date).
			Str("external_id", req.ExternalBookingID).
			Msg("failed to create external booking")

		writeJSON(w, http.StatusInternalServerError, BookDeviceResponse{
			Success: false,
			Error:   "failed to create booking",
		})
		return
	}

	s.log.Info().
		Int64("booking_id", bookingID).
		Int64("device_id", deviceID).
		Str("date", req.Date).
		Str("external_id", req.ExternalBookingID).
		Msg("external booking created")

	writeJSON(w, http.StatusOK, BookDeviceResponse{
		Success:   true,
		BookingID: bookingID,
	})
}

// handleCancelExternalBooking cancels a booking created via API.
// DELETE /api/book-device/{external_booking_id}
func (s *HTTPServer) handleCancelExternalBooking(w http.ResponseWriter, r *http.Request) {
	metrics.IncHTTP("cancel_external_booking")
	if r.Method != http.MethodDelete {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Extract external_booking_id from path
	const prefix = "/api/book-device/"
	if !hasPrefix(r.URL.Path, prefix) {
		writeError(w, http.StatusBadRequest, "invalid path")
		return
	}

	externalID := r.URL.Path[len(prefix):]
	if externalID == "" {
		writeError(w, http.StatusBadRequest, "external_booking_id is required")
		return
	}

	err := s.db.CancelExternalBooking(r.Context(), externalID)
	if err != nil {
		s.log.Error().Err(err).
			Str("external_id", externalID).
			Msg("failed to cancel external booking")
		writeError(w, http.StatusNotFound, "booking not found or already canceled")
		return
	}

	s.log.Info().
		Str("external_id", externalID).
		Msg("external booking canceled")

	writeJSON(w, http.StatusOK, map[string]any{"success": true})
}

func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}
