package api

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	availabilityv1 "bronivik/internal/api/gen/availability/v1"
	"bronivik/internal/database"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type AvailabilityService struct {
	availabilityv1.UnimplementedAvailabilityServiceServer
	db *database.DB
}

func NewAvailabilityService(db *database.DB) *AvailabilityService {
	return &AvailabilityService{
		db: db,
	}
}

func (s *AvailabilityService) GetAvailability(ctx context.Context, req *availabilityv1.GetAvailabilityRequest) (
	*availabilityv1.GetAvailabilityResponse, error) {
	itemName := strings.TrimSpace(req.GetItemName())
	if itemName == "" {
		return nil, status.Error(codes.InvalidArgument, "item_name is required")
	}

	dateStr := strings.TrimSpace(req.GetDate())
	if dateStr == "" {
		return nil, status.Error(codes.InvalidArgument, "date is required")
	}

	date, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid date format; expected YYYY-MM-DD")
	}

	info, err := s.db.GetItemAvailabilityByName(ctx, itemName, date)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, status.Error(codes.NotFound, "item not found")
		}
		return nil, status.Error(codes.Internal, "failed to get availability")
	}

	return &availabilityv1.GetAvailabilityResponse{
		ItemName:    info.ItemName,
		Date:        dateStr,
		Available:   info.Available,
		BookedCount: info.BookedCount,
		Total:       info.Total,
	}, nil
}

func (s *AvailabilityService) GetAvailabilityBulk(ctx context.Context, req *availabilityv1.GetAvailabilityBulkRequest) (
	*availabilityv1.GetAvailabilityBulkResponse, error) {
	items := req.GetItems()
	dates := req.GetDates()
	if len(items) == 0 {
		return nil, status.Error(codes.InvalidArgument, "items is required")
	}
	if len(dates) == 0 {
		return nil, status.Error(codes.InvalidArgument, "dates is required")
	}

	results := make([]*availabilityv1.Availability, 0, len(items)*len(dates))
	for _, rawItem := range items {
		itemName := strings.TrimSpace(rawItem)
		if itemName == "" {
			// Skip unknown items rather than failing the whole request.
			continue
		}

		for _, dateStr := range dates {
			dateStr = strings.TrimSpace(dateStr)
			date, err := time.Parse("2006-01-02", dateStr)
			if err != nil {
				return nil, status.Errorf(codes.InvalidArgument, "invalid date format: %s", dateStr)
			}

			info, err := s.db.GetItemAvailabilityByName(ctx, itemName, date)
			if err != nil {
				if !errors.Is(err, sql.ErrNoRows) {
					return nil, status.Error(codes.Internal, "failed to get availability")
				}
				// Skip unknown or inactive items rather than failing the whole request.
				continue
			}

			results = append(results, &availabilityv1.Availability{
				ItemName:    info.ItemName,
				Date:        dateStr,
				Available:   info.Available,
				BookedCount: info.BookedCount,
				Total:       info.Total,
			})
		}
	}

	return &availabilityv1.GetAvailabilityBulkResponse{Results: results}, nil
}

func (s *AvailabilityService) ListItems(
	ctx context.Context,
	_ *availabilityv1.ListItemsRequest,
) (*availabilityv1.ListItemsResponse, error) {
	items, err := s.db.GetCurrentItems(ctx)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to list items")
	}
	out := make([]*availabilityv1.Item, 0, len(items))
	for _, it := range items {
		if !it.IsActive {
			continue
		}
		out = append(out, &availabilityv1.Item{
			Id:            it.ID,
			Name:          it.Name,
			TotalQuantity: it.TotalQuantity,
		})
	}
	return &availabilityv1.ListItemsResponse{Items: out}, nil
}
