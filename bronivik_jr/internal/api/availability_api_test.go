package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"bronivik/internal/database"
	"bronivik/internal/models"

	"github.com/rs/zerolog"
)

const testAPIKey = "valid-key"

type ErrorResponse struct {
	Error string `json:"error"`
}

type testServer struct {
	*httptest.Server
	Handler http.Handler
}

func setupTestServer(t *testing.T) *testServer {
	db := newTestDB(t)
	server := newTestHTTPServer(db)
	handler := server.server.Handler
	return &testServer{
		Server:  httptest.NewServer(handler),
		Handler: handler,
	}
}

func TestHandleItemsAvailability_Validation(t *testing.T) {
	srv := setupTestServer(t)
	defer srv.Close()

	tests := []struct {
		name       string
		body       interface{}
		wantStatus int
		wantError  string
	}{
		{
			name:       "missing required fields",
			body:       map[string]string{},
			wantStatus: http.StatusBadRequest,
			wantError:  "start_date and end_date are required",
		},
		{
			name: "missing end_date",
			body: map[string]string{
				"start_date": "2025-01-15",
			},
			wantStatus: http.StatusBadRequest,
			wantError:  "start_date and end_date are required",
		},
		{
			name: "invalid start_date format",
			body: map[string]string{
				"start_date": "15-01-2025",
				"end_date":   "2025-01-20",
			},
			wantStatus: http.StatusBadRequest,
			wantError:  "invalid start_date format; expected YYYY-MM-DD",
		},
		{
			name: "invalid end_date format",
			body: map[string]string{
				"start_date": "2025-01-15",
				"end_date":   "20-01-2025",
			},
			wantStatus: http.StatusBadRequest,
			wantError:  "invalid end_date format; expected YYYY-MM-DD",
		},
		{
			name: "start_date after end_date",
			body: map[string]string{
				"start_date": "2025-01-20",
				"end_date":   "2025-01-15",
			},
			wantStatus: http.StatusBadRequest,
			wantError:  "start_date must be before or equal to end_date",
		},
		{
			name: "date range exceeds 90 days",
			body: map[string]string{
				"start_date": "2025-01-01",
				"end_date":   "2025-05-01",
			},
			wantStatus: http.StatusBadRequest,
			wantError:  "date range exceeds maximum of 90 days",
		},
		{
			name:       "invalid JSON",
			body:       "not json",
			wantStatus: http.StatusBadRequest,
			wantError:  "invalid JSON body",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var body []byte
			if s, ok := tt.body.(string); ok {
				body = []byte(s)
			} else {
				body, _ = json.Marshal(tt.body)
			}

			req := httptest.NewRequest(http.MethodPost, "/api/items/availability", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Api-Key", testAPIKey)

			w := httptest.NewRecorder()
			srv.Handler.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", w.Code, tt.wantStatus)
			}

			var resp ErrorResponse
			if err := json.Unmarshal(w.Body.Bytes(), &resp); err == nil {
				if resp.Error != tt.wantError {
					t.Errorf("error = %q, want %q", resp.Error, tt.wantError)
				}
			}
		})
	}
}

func TestHandleItemsAvailability_MethodNotAllowed(t *testing.T) {
	srv := setupTestServer(t)
	defer srv.Close()

	req := httptest.NewRequest(http.MethodGet, "/api/items/availability", http.NoBody)
	req.Header.Set("X-Api-Key", testAPIKey)

	w := httptest.NewRecorder()
	srv.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want %d", w.Code, http.StatusMethodNotAllowed)
	}
}

func TestHandleItemsAvailability_EmptyPeriod(t *testing.T) {
	srv := setupTestServer(t)
	defer srv.Close()

	body := AvailabilityRequest{
		StartDate: "2025-01-15",
		EndDate:   "2025-01-17",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/items/availability", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Api-Key", testAPIKey)

	w := httptest.NewRecorder()
	srv.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp AvailabilityResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	// Check period in response
	if resp.Period.Start != "2025-01-15" {
		t.Errorf("period.start = %q, want %q", resp.Period.Start, "2025-01-15")
	}
	if resp.Period.End != "2025-01-17" {
		t.Errorf("period.end = %q, want %q", resp.Period.End, "2025-01-17")
	}
}

func TestHandleItemsAvailability_FilterByItemIDs(t *testing.T) {
	srv := setupTestServer(t)
	defer srv.Close()

	body := AvailabilityRequest{
		StartDate: "2025-01-15",
		EndDate:   "2025-01-15",
		ItemIDs:   []int64{1, 2},
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/items/availability", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Api-Key", testAPIKey)

	w := httptest.NewRecorder()
	srv.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp AvailabilityResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	// Verify only requested items are returned
	for _, item := range resp.Items {
		found := false
		for _, id := range body.ItemIDs {
			if item.ID == id {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("unexpected item ID %d in response", item.ID)
		}
	}
}

func TestHandleItemsAvailability_SingleDay(t *testing.T) {
	srv := setupTestServer(t)
	defer srv.Close()

	body := AvailabilityRequest{
		StartDate: "2025-01-15",
		EndDate:   "2025-01-15",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/items/availability", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Api-Key", testAPIKey)

	w := httptest.NewRecorder()
	srv.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp AvailabilityResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	// Each item should have exactly one date in availability
	for _, item := range resp.Items {
		if len(item.Availability) != 1 {
			t.Errorf("item %d has %d dates, want 1", item.ID, len(item.Availability))
		}
		if len(item.Availability) > 0 && item.Availability[0].Date != "2025-01-15" {
			t.Errorf("item %d date = %q, want %q", item.ID, item.Availability[0].Date, "2025-01-15")
		}
	}
}

func TestHandleItemsAvailability_MaxRange(t *testing.T) {
	srv := setupTestServer(t)
	defer srv.Close()

	// Exactly 90 days should be allowed
	startDate := time.Now().Format("2006-01-02")
	endDate := time.Now().AddDate(0, 0, 90).Format("2006-01-02")

	body := AvailabilityRequest{
		StartDate: startDate,
		EndDate:   endDate,
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/items/availability", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Api-Key", testAPIKey)

	w := httptest.NewRecorder()
	srv.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d for 90-day range, want %d", w.Code, http.StatusOK)
	}
}

func TestHandleItemsAvailability_BatchedResults(t *testing.T) {
	db := newTestDB(t)
	camera := createTestItem(t, db, "camera", 1)
	createTestItem(t, db, "lens", 2)

	bookingDate := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	insertTestBooking(t, db, &camera, bookingDate, models.StatusConfirmed)

	server := newTestHTTPServer(db)
	handler := server.server.Handler

	body := AvailabilityRequest{
		StartDate: "2025-01-15",
		EndDate:   "2025-01-16",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/items/availability", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp AvailabilityResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	itemsByName := make(map[string]ItemAvailability, len(resp.Items))
	for _, item := range resp.Items {
		itemsByName[item.Name] = item
	}

	cameraResp, ok := itemsByName["camera"]
	if !ok {
		t.Fatal("camera item missing from response")
	}
	if len(cameraResp.Availability) != 2 {
		t.Fatalf("camera availability length = %d, want 2", len(cameraResp.Availability))
	}
	if cameraResp.Availability[0].Available || cameraResp.Availability[0].Reason != "booked" {
		t.Fatalf("expected booked camera on first date, got %+v", cameraResp.Availability[0])
	}
	if !cameraResp.Availability[1].Available || cameraResp.Availability[1].Reason != "" {
		t.Fatalf("expected available camera on second date, got %+v", cameraResp.Availability[1])
	}

	lensResp, ok := itemsByName["lens"]
	if !ok {
		t.Fatal("lens item missing from response")
	}
	for _, availability := range lensResp.Availability {
		if !availability.Available || availability.Reason != "" {
			t.Fatalf("expected free lens availability, got %+v", availability)
		}
	}
}

func BenchmarkHandleItemsAvailability_90Days(b *testing.B) {
	dbPath := filepath.Join(b.TempDir(), "bench.db")
	logger := zerolog.New(io.Discard)
	db, err := database.NewDB(dbPath, &logger)
	if err != nil {
		b.Fatalf("new db: %v", err)
	}
	b.Cleanup(func() { _ = db.Close() })

	ctx := context.Background()
	baseDate := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 25; i++ {
		item := models.Item{
			Name:          "item-" + strconv.Itoa(i),
			TotalQuantity: 2,
			SortOrder:     int64(i),
			IsActive:      true,
		}
		if err := db.CreateItem(ctx, &item); err != nil {
			b.Fatalf("create item %d: %v", i, err)
		}
		for day := 0; day < 90; day += 7 {
			if _, err := db.ExecContext(ctx, `
				INSERT INTO bookings (
					user_id, user_name, user_nickname, phone,
					item_id, item_name, date, status, comment,
					created_at, updated_at, version
				) VALUES (?, ?, ?, ?, ?, ?, ?, ?, '', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 1)
			`,
				int64(1),
				"bench",
				"bench_nick",
				"+100000000",
				item.ID,
				item.Name,
				baseDate.AddDate(0, 0, day).Format("2006-01-02"),
				models.StatusConfirmed,
			); err != nil {
				b.Fatalf("insert booking: %v", err)
			}
		}
	}

	server := newTestHTTPServer(db)
	handler := server.server.Handler
	body := AvailabilityRequest{
		StartDate: baseDate.Format("2006-01-02"),
		EndDate:   baseDate.AddDate(0, 0, 89).Format("2006-01-02"),
	}
	jsonBody, _ := json.Marshal(body)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/items/availability", bytes.NewReader(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			b.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
		}
	}
}

func TestDateAvailability_Reasons(t *testing.T) {
	tests := []struct {
		name      string
		available bool
		reason    string
	}{
		{
			name:      "available date",
			available: true,
			reason:    "",
		},
		{
			name:      "booked date",
			available: false,
			reason:    "booked",
		},
		{
			name:      "reserved date",
			available: false,
			reason:    "reserved",
		},
		{
			name:      "maintenance date",
			available: false,
			reason:    "maintenance",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			da := DateAvailability{
				Date:      "2025-01-15",
				Available: tt.available,
				Reason:    tt.reason,
			}

			// Verify JSON serialization
			data, err := json.Marshal(da)
			if err != nil {
				t.Fatalf("failed to marshal: %v", err)
			}

			var unmarshaled DateAvailability
			if err := json.Unmarshal(data, &unmarshaled); err != nil {
				t.Fatalf("failed to unmarshal: %v", err)
			}

			if unmarshaled.Available != tt.available {
				t.Errorf("available = %v, want %v", unmarshaled.Available, tt.available)
			}
			if unmarshaled.Reason != tt.reason {
				t.Errorf("reason = %q, want %q", unmarshaled.Reason, tt.reason)
			}
		})
	}
}

func TestAvailabilityRequest_JSONSerialization(t *testing.T) {
	req := AvailabilityRequest{
		StartDate: "2025-01-15",
		EndDate:   "2025-01-20",
		ItemIDs:   []int64{1, 2, 3},
		CabinetID: ptrInt64(42),
		Category:  "equipment",
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var unmarshaled AvailabilityRequest
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if unmarshaled.StartDate != req.StartDate {
		t.Errorf("start_date = %q, want %q", unmarshaled.StartDate, req.StartDate)
	}
	if unmarshaled.EndDate != req.EndDate {
		t.Errorf("end_date = %q, want %q", unmarshaled.EndDate, req.EndDate)
	}
	if len(unmarshaled.ItemIDs) != len(req.ItemIDs) {
		t.Errorf("item_ids length = %d, want %d", len(unmarshaled.ItemIDs), len(req.ItemIDs))
	}
	if unmarshaled.CabinetID == nil || *unmarshaled.CabinetID != *req.CabinetID {
		t.Errorf("cabinet_id mismatch")
	}
	if unmarshaled.Category != req.Category {
		t.Errorf("category = %q, want %q", unmarshaled.Category, req.Category)
	}
}

// Helper function for int64 pointer
func ptrInt64(v int64) *int64 {
	return &v
}
