package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"bronivik/internal/config"
	"bronivik/internal/database"
	"bronivik/internal/models"

	"github.com/rs/zerolog"
)

// Integration-style test: HTTP availability reflects bookings persisted in DB.
func TestAvailabilityReflectsBookings(t *testing.T) {
	db := newIntegrationDB(t)
	item := createIntegrationItem(t, db, "camera", 1)
	db.SetItems([]*models.Item{&item})

	server := newIntegrationHTTPServer(db)
	ts := httptest.NewServer(server.server.Handler)
	t.Cleanup(ts.Close)

	date := time.Date(2025, 12, 1, 0, 0, 0, 0, time.UTC)
	dateStr := date.Format("2006-01-02")

	checkAvailable(t, ts.URL, item.Name, dateStr, true, 0, item.TotalQuantity)

	insertIntegrationBooking(t, db, &item, date, "pending")

	checkAvailable(t, ts.URL, item.Name, dateStr, false, 1, item.TotalQuantity)
}

func TestExternalBookDeviceHonorsBlockingStatuses(t *testing.T) {
	tests := []struct {
		name               string
		existingStatus     string
		cancelBeforeBook   bool
		wantHTTPStatusCode int
		wantCreated        bool
	}{
		{
			name:               "pending blocks external booking",
			existingStatus:     models.StatusPending,
			wantHTTPStatusCode: http.StatusConflict,
		},
		{
			name:               "confirmed blocks external booking",
			existingStatus:     models.StatusConfirmed,
			wantHTTPStatusCode: http.StatusConflict,
		},
		{
			name:               "canceled does not block external booking",
			existingStatus:     models.StatusPending,
			cancelBeforeBook:   true,
			wantHTTPStatusCode: http.StatusOK,
			wantCreated:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := newIntegrationDB(t)
			item := createIntegrationItem(t, db, "shared-device", 1)
			db.SetItems([]*models.Item{&item})

			server := newIntegrationHTTPServer(db)
			ts := httptest.NewServer(server.server.Handler)
			t.Cleanup(ts.Close)

			date := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
			existing := &models.Booking{
				UserID:   1,
				UserName: "existing",
				Phone:    "+70000000000",
				ItemID:   item.ID,
				ItemName: item.Name,
				Date:     date,
				Status:   tt.existingStatus,
			}
			if err := db.CreateBooking(context.Background(), existing); err != nil {
				t.Fatalf("create existing booking: %v", err)
			}
			if tt.cancelBeforeBook {
				if err := db.UpdateBookingStatus(context.Background(), existing.ID, models.StatusCanceled); err != nil {
					t.Fatalf("cancel booking: %v", err)
				}
			}

			statusCode, resp := bookDevice(t, ts.URL, BookDeviceRequest{
				DeviceID:          item.ID,
				Date:              date.Format("2006-01-02"),
				ExternalBookingID: "crm-" + tt.name,
				ClientName:        "External Client",
				ClientPhone:       "+79990000000",
			})

			if statusCode != tt.wantHTTPStatusCode {
				t.Fatalf("status: want %d got %d", tt.wantHTTPStatusCode, statusCode)
			}
			if resp.Success != tt.wantCreated {
				t.Fatalf("success: want %v got %v", tt.wantCreated, resp.Success)
			}

			if !tt.wantCreated {
				return
			}

			stored, err := db.GetExternalBooking(context.Background(), "crm-"+tt.name)
			if err != nil {
				t.Fatalf("get external booking: %v", err)
			}
			if stored.Status != models.StatusConfirmed {
				t.Fatalf("status: want %s got %s", models.StatusConfirmed, stored.Status)
			}
		})
	}
}

func newIntegrationHTTPServer(db *database.DB) *HTTPServer {
	cfg := config.APIConfig{Enabled: true, HTTP: config.APIHTTPConfig{Enabled: true, Port: 0}, Auth: config.APIAuthConfig{Enabled: false}}
	logger := zerolog.New(io.Discard)
	return NewHTTPServer(&cfg, db, nil, nil, &logger)
}

func newIntegrationDB(t *testing.T) *database.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "integration.db")
	logger := zerolog.New(io.Discard)
	db, err := database.NewDB(path, &logger)
	if err != nil {
		t.Fatalf("new db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func createIntegrationItem(t *testing.T, db *database.DB, name string, total int64) models.Item {
	t.Helper()
	item := models.Item{Name: name, TotalQuantity: total, SortOrder: 1}
	if err := db.CreateItem(context.Background(), &item); err != nil {
		t.Fatalf("create item: %v", err)
	}
	return item
}

func insertIntegrationBooking(t *testing.T, db *database.DB, item *models.Item, date time.Time, status string) {
	t.Helper()
	_, err := db.ExecContext(context.Background(), `
		INSERT INTO bookings (
			user_id, user_name, user_nickname, phone, 
			item_id, item_name, date, status, comment, 
			created_at, updated_at, version
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, '', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 1)
	`, int64(1), "tester", "tester_nick", "+100", item.ID, item.Name, date.Format("2006-01-02"), status)
	if err != nil {
		t.Fatalf("insert booking: %v", err)
	}
}

func checkAvailable(t *testing.T, baseURL, itemName, dateStr string, wantAvailable bool, wantBooked, wantTotal int64) {
	t.Helper()
	url := fmt.Sprintf("%s/api/v1/availability/%s?date=%s", baseURL, itemName, dateStr)
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: got %d", resp.StatusCode)
	}

	var body struct {
		Available   bool  `json:"available"`
		BookedCount int64 `json:"booked_count"`
		Total       int64 `json:"total"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if body.Available != wantAvailable {
		t.Fatalf("available: want %v got %v", wantAvailable, body.Available)
	}
	if body.BookedCount != wantBooked {
		t.Fatalf("booked_count: want %d got %d", wantBooked, body.BookedCount)
	}
	if body.Total != wantTotal {
		t.Fatalf("total: want %d got %d", wantTotal, body.Total)
	}
}

func bookDevice(t *testing.T, baseURL string, req BookDeviceRequest) (int, BookDeviceResponse) {
	t.Helper()

	body, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	resp, err := http.Post(baseURL+"/api/book-device", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()

	var parsed BookDeviceResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	return resp.StatusCode, parsed
}
