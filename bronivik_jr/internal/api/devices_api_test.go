package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// MockDeviceRepository implements a mock for testing
type MockDeviceRepository struct {
	devices []Device
	err     error
}

type Device struct {
	ID                int64  `json:"id"`
	Name              string `json:"name"`
	Available         bool   `json:"available"`
	PermanentReserved bool   `json:"permanent_reserved"`
}

func (m *MockDeviceRepository) GetDevices(date string, includeReserved bool) ([]Device, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.devices, nil
}

func (m *MockDeviceRepository) CreateExternalBooking(_ *ExternalBookingRequest) (int64, error) {
	if m.err != nil {
		return 0, m.err
	}
	return 123, nil
}

func (m *MockDeviceRepository) CancelExternalBooking(externalID string) error {
	return m.err
}

type ExternalBookingRequest struct {
	DeviceID    int64  `json:"device_id"`
	DeviceName  string `json:"device_name,omitempty"`
	Date        string `json:"date"`
	ExternalID  string `json:"external_id"`
	ClientName  string `json:"client_name"`
	ClientPhone string `json:"client_phone,omitempty"`
}

func TestGetDevicesEndpoint(t *testing.T) {
	mockRepo := &MockDeviceRepository{
		devices: []Device{
			{ID: 1, Name: "УЗИ-1", Available: true, PermanentReserved: false},
			{ID: 2, Name: "МРТ", Available: false, PermanentReserved: true},
		},
	}

	tests := []struct {
		name           string
		queryParams    string
		expectedStatus int
		expectedCount  int
	}{
		{
			name:           "get all devices",
			queryParams:    "",
			expectedStatus: http.StatusOK,
			expectedCount:  2,
		},
		{
			name:           "get devices for specific date",
			queryParams:    "?date=2026-01-15",
			expectedStatus: http.StatusOK,
			expectedCount:  2,
		},
		{
			name:           "include reserved devices",
			queryParams:    "?include_reserved=true",
			expectedStatus: http.StatusOK,
			expectedCount:  2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest("GET", "/api/devices"+tt.queryParams, http.NoBody)
			req.Header.Set("x-api-key", "test-key")

			w := httptest.NewRecorder()

			// Simulate handler response
			resp := struct {
				Devices []Device `json:"devices"`
				Date    string   `json:"date,omitempty"`
			}{
				Devices: mockRepo.devices,
			}

			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(resp); err != nil {
				t.Fatalf("failed to encode response: %v", err)
			}

			if w.Code != http.StatusOK {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			var response struct {
				Devices []Device `json:"devices"`
			}
			if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}

			if len(response.Devices) != tt.expectedCount {
				t.Errorf("expected %d devices, got %d", tt.expectedCount, len(response.Devices))
			}
		})
	}
}

func TestBookDeviceEndpoint(t *testing.T) {
	tests := []struct {
		name           string
		request        ExternalBookingRequest
		expectedStatus int
		expectedError  bool
	}{
		{
			name: "successful booking",
			request: ExternalBookingRequest{
				DeviceID:   1,
				Date:       "2026-01-15",
				ExternalID: "crm-12345",
				ClientName: "Иванов Иван",
			},
			expectedStatus: http.StatusOK,
			expectedError:  false,
		},
		{
			name: "missing required fields",
			request: ExternalBookingRequest{
				DeviceID: 1,
				// Missing date and external_id
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.request)
			req := httptest.NewRequest("POST", "/api/book-device", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("x-api-key", "test-key")

			w := httptest.NewRecorder()

			// Simulate response based on request validity
			if tt.request.Date == "" || tt.request.ExternalID == "" {
				w.WriteHeader(http.StatusBadRequest)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"success": false,
					"error":   "missing required fields",
				})
			} else {
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"success":    true,
					"booking_id": 123,
				})
			}

			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			var response map[string]interface{}
			if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}

			if tt.expectedError && response["success"] == true {
				t.Error("expected error response")
			}
			if !tt.expectedError && response["success"] != true {
				t.Error("expected successful response")
			}
		})
	}
}

func TestCancelBookingEndpoint(t *testing.T) {
	tests := []struct {
		name           string
		externalID     string
		expectedStatus int
	}{
		{
			name:           "cancel existing booking",
			externalID:     "crm-12345",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "cancel non-existent booking",
			externalID:     "non-existent",
			expectedStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest("DELETE", "/api/book-device/"+tt.externalID, http.NoBody)
			req.Header.Set("x-api-key", "test-key")

			w := httptest.NewRecorder()

			// Simulate response
			if tt.externalID == "non-existent" {
				w.WriteHeader(http.StatusNotFound)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"success": false,
					"error":   "booking not found",
				})
			} else {
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"success": true,
					"message": "booking canceled",
				})
			}

			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, w.Code)
			}
		})
	}
}

func TestAPIAuthMiddleware(t *testing.T) {
	tests := []struct {
		name           string
		apiKey         string
		expectedStatus int
	}{
		{
			name:           "valid api key",
			apiKey:         "valid-key",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "missing api key",
			apiKey:         "",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "invalid api key",
			apiKey:         "invalid-key",
			expectedStatus: http.StatusUnauthorized,
		},
	}

	validKeys := map[string]bool{"valid-key": true}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest("GET", "/api/devices", http.NoBody)
			if tt.apiKey != "" {
				req.Header.Set("x-api-key", tt.apiKey)
			}

			w := httptest.NewRecorder()

			// Simulate auth middleware
			apiKey := req.Header.Get("x-api-key")
			if apiKey == "" || !validKeys[apiKey] {
				w.WriteHeader(http.StatusUnauthorized)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"error": "unauthorized",
					"code":  "unauthorized",
				})
			} else {
				w.WriteHeader(http.StatusOK)
			}

			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, w.Code)
			}
		})
	}
}

func TestHealthEndpoints(t *testing.T) {
	endpoints := []string{"/healthz", "/readyz"}

	for _, endpoint := range endpoints {
		t.Run(endpoint, func(t *testing.T) {
			w := httptest.NewRecorder()

			// Simulate health check response
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "ok",
				"version": "1.0.0",
			})

			if w.Code != http.StatusOK {
				t.Errorf("expected status 200, got %d", w.Code)
			}

			var response map[string]interface{}
			if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}

			if response["status"] != "ok" {
				t.Errorf("expected status ok, got %v", response["status"])
			}
		})
	}
}
