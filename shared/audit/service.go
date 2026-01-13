package audit

import (
	"bytes"
	"context"
	"fmt"
	"sync"
	"time"
)

// Config holds configuration for the audit service.
type Config struct {
	// DataRetentionDays is how many days to keep data before deletion.
	// Default: 31 days.
	DataRetentionDays int

	// ExportOnStart if true, runs export immediately on service start.
	ExportOnStart bool

	// BotName identifies this bot in reports.
	BotName string
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		DataRetentionDays: 31,
		ExportOnStart:     false,
		BotName:           "bot",
	}
}

// DataCleaner interface for cleaning old data.
type DataCleaner interface {
	// DeleteOldBookings deletes bookings older than duration.
	DeleteOldBookings(ctx context.Context, olderThan time.Duration) (int64, error)
}

// Service handles monthly audit exports and data cleanup.
type Service struct {
	config   *Config
	exporter TableExporter
	writer   func() ExcelWriter // factory for creating new Excel writers
	notifier Notifier
	cleaner  DataCleaner
	logger   Logger
	stopCh   chan struct{}
	wg       sync.WaitGroup
	mu       sync.Mutex
	running  bool
}

// NewService creates a new audit service.
func NewService(
	config *Config,
	exporter TableExporter,
	writerFactory func() ExcelWriter,
	notifier Notifier,
	cleaner DataCleaner,
	logger Logger,
) *Service {
	if config == nil {
		config = DefaultConfig()
	}
	if config.DataRetentionDays <= 0 {
		config.DataRetentionDays = 31
	}

	return &Service{
		config:   config,
		exporter: exporter,
		writer:   writerFactory,
		notifier: notifier,
		cleaner:  cleaner,
		logger:   logger,
		stopCh:   make(chan struct{}),
	}
}

// Start begins the audit scheduler.
func (s *Service) Start() {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	s.running = true
	s.mu.Unlock()

	if s.config.ExportOnStart {
		go s.RunExportAndCleanup()
	}

	s.wg.Add(1)
	go s.loop()

	if s.logger != nil {
		s.logger.Info("Audit service started",
			"retention_days", s.config.DataRetentionDays,
		)
	}
}

// Stop gracefully stops the audit service.
func (s *Service) Stop() {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return
	}
	s.running = false
	s.mu.Unlock()

	close(s.stopCh)
	s.wg.Wait()

	if s.logger != nil {
		s.logger.Info("Audit service stopped")
	}
}

func (s *Service) loop() {
	defer s.wg.Done()

	// Calculate time until next 1st of month at 00:00
	nextRun := s.nextFirstOfMonth()
	timer := time.NewTimer(time.Until(nextRun))
	defer timer.Stop()

	if s.logger != nil {
		s.logger.Info("Next audit scheduled", "time", nextRun)
	}

	for {
		select {
		case <-s.stopCh:
			return
		case <-timer.C:
			s.RunExportAndCleanup()

			// Schedule next run
			nextRun = s.nextFirstOfMonth()
			timer.Reset(time.Until(nextRun))

			if s.logger != nil {
				s.logger.Info("Next audit scheduled", "time", nextRun)
			}
		}
	}
}

func (s *Service) nextFirstOfMonth() time.Time {
	now := time.Now()
	// First day of next month at 00:01
	nextMonth := time.Date(now.Year(), now.Month()+1, 1, 0, 1, 0, 0, now.Location())
	return nextMonth
}

// RunExportAndCleanup performs the export and cleanup immediately.
func (s *Service) RunExportAndCleanup() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	// Export data first
	if err := s.exportData(ctx); err != nil {
		if s.logger != nil {
			s.logger.Error("Failed to export audit data", "error", err)
		}
	}

	// Then cleanup old data
	if err := s.cleanupOldData(ctx); err != nil {
		if s.logger != nil {
			s.logger.Error("Failed to cleanup old data", "error", err)
		}
	}
}

func (s *Service) exportData(ctx context.Context) error {
	if s.exporter == nil || s.writer == nil {
		return fmt.Errorf("exporter or writer not configured")
	}

	tables, err := s.exporter.GetTableNames(ctx)
	if err != nil {
		return fmt.Errorf("get table names: %w", err)
	}

	if len(tables) == 0 {
		if s.logger != nil {
			s.logger.Info("No tables to export")
		}
		return nil
	}

	excel := s.writer()
	if excel == nil {
		return fmt.Errorf("failed to create excel writer")
	}

	for _, tableName := range tables {
		data, columns, err := s.exporter.GetTableData(ctx, tableName)
		if err != nil {
			if s.logger != nil {
				s.logger.Error("Failed to get table data", "table", tableName, "error", err)
			}
			continue
		}

		if err := excel.AddSheet(tableName); err != nil {
			if s.logger != nil {
				s.logger.Error("Failed to add sheet", "table", tableName, "error", err)
			}
			continue
		}

		if err := excel.WriteHeader(columns); err != nil {
			if s.logger != nil {
				s.logger.Error("Failed to write header", "table", tableName, "error", err)
			}
			continue
		}

		for _, row := range data {
			rowData := make([]interface{}, len(columns))
			for i, col := range columns {
				rowData[i] = row[col]
			}
			if err := excel.WriteRow(rowData); err != nil {
				if s.logger != nil {
					s.logger.Error("Failed to write row", "table", tableName, "error", err)
				}
			}
		}

		if s.logger != nil {
			s.logger.Debug("Exported table", "table", tableName, "rows", len(data))
		}
	}

	// Save to buffer
	var buf bytes.Buffer
	if err := excel.Save(&buf); err != nil {
		return fmt.Errorf("save excel: %w", err)
	}

	// Send to managers
	if s.notifier != nil {
		filename := GenerateFilenameForPreviousMonth()
		caption := fmt.Sprintf("📊 Ежемесячный отчёт %s", s.config.BotName)

		if err := s.notifier.SendDocument(ctx, filename, &buf, caption); err != nil {
			return fmt.Errorf("send document: %w", err)
		}

		if s.logger != nil {
			s.logger.Info("Audit report sent", "filename", filename)
		}
	}

	return nil
}

func (s *Service) cleanupOldData(ctx context.Context) error {
	if s.cleaner == nil {
		return nil
	}

	retention := time.Duration(s.config.DataRetentionDays) * 24 * time.Hour
	deleted, err := s.cleaner.DeleteOldBookings(ctx, retention)
	if err != nil {
		return fmt.Errorf("delete old bookings: %w", err)
	}

	if s.logger != nil {
		s.logger.Info("Cleaned up old data",
			"deleted_count", deleted,
			"retention_days", s.config.DataRetentionDays,
		)
	}

	return nil
}

// ExportNow triggers an immediate export (useful for testing or manual runs).
func (s *Service) ExportNow() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	return s.exportData(ctx)
}

// CleanupNow triggers an immediate cleanup (useful for testing).
func (s *Service) CleanupNow() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	return s.cleanupOldData(ctx)
}
