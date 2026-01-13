package crmapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

// BronivikClient is a simple HTTP client to call bronivik_jr availability APIs.
type BronivikClient struct {
	baseURL    string
	apiKey     string
	apiExtra   string
	httpClient *http.Client

	redis    *redis.Client
	cacheTTL time.Duration
}

// AvailabilityResponse represents the response from availability API.
type AvailabilityResponse struct {
	Available bool `json:"available"`
}

// Item represents an item from the API.
type Item struct {
	ID            int64  `json:"id"`
	Name          string `json:"name"`
	TotalQuantity int    `json:"total_quantity"`
}

// NewBronivikClient constructs a client with baseURL, API key and extra header.
func NewBronivikClient(baseURL, apiKey, apiExtra string) *BronivikClient {
	return &BronivikClient{
		baseURL:    baseURL,
		apiKey:     apiKey,
		apiExtra:   apiExtra,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// UseRedisCache configures optional Redis caching for GET endpoints.
func (c *BronivikClient) UseRedisCache(redisClient *redis.Client, ttl time.Duration) {
	c.redis = redisClient
	c.cacheTTL = ttl
}

// GetAvailability fetches availability for item/date (YYYY-MM-DD).
func (c *BronivikClient) GetAvailability(ctx context.Context, itemName, date string) (*AvailabilityResponse, error) {
	endpoint := fmt.Sprintf("%s/api/v1/availability/%s?date=%s", c.baseURL, url.PathEscape(itemName), url.QueryEscape(date))
	cacheKey := fmt.Sprintf("availability:%s:%s", itemName, date)
	var resp AvailabilityResponse

	if c.readCache(ctx, cacheKey, &resp) {
		return &resp, nil
	}

	if err := c.doGet(ctx, endpoint, &resp); err != nil {
		return nil, err
	}
	c.writeCache(ctx, cacheKey, resp)
	return &resp, nil
}

// GetAvailabilityBulk fetches availability for multiple items/dates.
type BulkAvailabilityRequest struct {
	Items []string `json:"items"`
	Dates []string `json:"dates"`
}

type BulkAvailabilityResponse struct {
	Results []map[string]any `json:"results"`
}

func (c *BronivikClient) GetAvailabilityBulk(ctx context.Context, items, dates []string) (*BulkAvailabilityResponse, error) {
	endpoint := fmt.Sprintf("%s/api/v1/availability/bulk", c.baseURL)
	body := BulkAvailabilityRequest{Items: items, Dates: dates}
	var resp BulkAvailabilityResponse
	if err := c.doPost(ctx, endpoint, body, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// ListItems returns all items.
func (c *BronivikClient) ListItems(ctx context.Context) ([]Item, error) {
	endpoint := fmt.Sprintf("%s/api/v1/items", c.baseURL)
	cacheKey := "items"
	var wrap struct {
		Items []Item `json:"items"`
	}

	if c.readCache(ctx, cacheKey, &wrap) {
		return wrap.Items, nil
	}

	if err := c.doGet(ctx, endpoint, &wrap); err != nil {
		return nil, err
	}
	c.writeCache(ctx, cacheKey, wrap)
	return wrap.Items, nil
}

func (c *BronivikClient) readCache(ctx context.Context, key string, out any) bool {
	if c.redis == nil || c.cacheTTL <= 0 {
		return false
	}
	val, err := c.redis.Get(ctx, key).Result()
	if err != nil {
		return false
	}
	if err := json.Unmarshal([]byte(val), out); err != nil {
		return false
	}
	return true
}

func (c *BronivikClient) writeCache(ctx context.Context, key string, val any) {
	if c.redis == nil || c.cacheTTL <= 0 {
		return
	}
	data, err := json.Marshal(val)
	if err != nil {
		return
	}
	_ = c.redis.Set(ctx, key, data, c.cacheTTL).Err()
}

func (c *BronivikClient) doGet(ctx context.Context, endpoint string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, http.NoBody)
	if err != nil {
		return err
	}
	c.addHeaders(req)
	return c.do(req, out)
}

func (c *BronivikClient) doPost(ctx context.Context, endpoint string, body, out any) error {
	data, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(string(data)))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	c.addHeaders(req)
	return c.do(req, out)
}

func (c *BronivikClient) do(req *http.Request, out any) error {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("http %d", resp.StatusCode)
	}
	if out == nil {
		return nil
	}
	dec := json.NewDecoder(resp.Body)
	return dec.Decode(out)
}

func (c *BronivikClient) addHeaders(req *http.Request) {
	if c.apiKey != "" {
		req.Header.Set("x-api-key", c.apiKey)
	}
	if c.apiExtra != "" {
		req.Header.Set("x-api-extra", c.apiExtra)
	}
}

// Device represents a device from the devices API.
type Device struct {
	ID                int64  `json:"id"`
	Name              string `json:"name"`
	Description       string `json:"description,omitempty"`
	Available         bool   `json:"available"`
	PermanentReserved bool   `json:"permanent_reserved"`
}

// DevicesResponse represents the response from GET /api/devices.
type DevicesResponse struct {
	Devices []Device `json:"devices"`
}

// BookDeviceRequest is the request body for booking a device.
type BookDeviceRequest struct {
	DeviceID          int64  `json:"device_id,omitempty"`
	DeviceName        string `json:"device_name,omitempty"`
	Date              string `json:"date"`
	ExternalBookingID string `json:"external_booking_id"`
	ClientName        string `json:"client_name,omitempty"`
	ClientPhone       string `json:"client_phone,omitempty"`
}

// BookDeviceResponse is the response from POST /api/book-device.
type BookDeviceResponse struct {
	Success   bool   `json:"success"`
	BookingID int64  `json:"booking_id,omitempty"`
	Error     string `json:"error,omitempty"`
}

// GetDevices fetches available devices for a date.
func (c *BronivikClient) GetDevices(ctx context.Context, date string, includeReserved bool) ([]Device, error) {
	endpoint := fmt.Sprintf("%s/api/devices?date=%s", c.baseURL, url.QueryEscape(date))
	if includeReserved {
		endpoint += "&include_reserved=true"
	}

	cacheKey := fmt.Sprintf("devices:%s:%v", date, includeReserved)
	var resp DevicesResponse

	if c.readCache(ctx, cacheKey, &resp) {
		return resp.Devices, nil
	}

	if err := c.doGet(ctx, endpoint, &resp); err != nil {
		return nil, err
	}
	c.writeCache(ctx, cacheKey, resp)
	return resp.Devices, nil
}

// GetAvailableDevicesForDate returns only available devices for a date.
func (c *BronivikClient) GetAvailableDevicesForDate(ctx context.Context, date time.Time) ([]Device, error) {
	dateStr := date.Format("2006-01-02")
	devices, err := c.GetDevices(ctx, dateStr, true) // include reserved for full list
	if err != nil {
		return nil, err
	}

	// Filter to available only
	var available []Device
	for _, d := range devices {
		if d.Available {
			available = append(available, d)
		}
	}
	return available, nil
}

// BookDevice books a device via Bot 1 API.
func (c *BronivikClient) BookDevice(ctx context.Context, req BookDeviceRequest) (*BookDeviceResponse, error) {
	endpoint := fmt.Sprintf("%s/api/book-device", c.baseURL)
	var resp BookDeviceResponse
	if err := c.doPost(ctx, endpoint, req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// BookDeviceSimple is a convenience method for booking.
func (c *BronivikClient) BookDeviceSimple(
	ctx context.Context,
	deviceID int64,
	date time.Time,
	externalBookingID string,
	clientName, clientPhone string,
) (*BookDeviceResponse, error) {
	req := BookDeviceRequest{
		DeviceID:          deviceID,
		Date:              date.Format("2006-01-02"),
		ExternalBookingID: externalBookingID,
		ClientName:        clientName,
		ClientPhone:       clientPhone,
	}
	return c.BookDevice(ctx, req)
}

// CancelDeviceBooking cancels a device booking by external ID.
func (c *BronivikClient) CancelDeviceBooking(ctx context.Context, externalBookingID string) error {
	endpoint := fmt.Sprintf("%s/api/book-device/%s", c.baseURL, url.PathEscape(externalBookingID))
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, endpoint, http.NoBody)
	if err != nil {
		return err
	}
	c.addHeaders(req)
	return c.do(req, nil)
}

// HealthCheck checks if Bot 1 API is available.
func (c *BronivikClient) HealthCheck(ctx context.Context) error {
	endpoint := fmt.Sprintf("%s/healthz", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, http.NoBody)
	if err != nil {
		return err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check failed: %d", resp.StatusCode)
	}
	return nil
}
