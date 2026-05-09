package app

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type HolidayCalendarFileProvider struct {
	basePath string
	mu       sync.RWMutex
	cache    map[int]map[string]string
}

type HolidayCalendarYearFile struct {
	Year     int               `json:"year"`
	Holidays []HolidayCalendar `json:"holidays"`
}

type HolidayCalendar struct {
	Date        string `json:"date"`
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description,omitempty"`
}

func NewHolidayCalendarFileProvider(basePath string) *HolidayCalendarFileProvider {
	return &HolidayCalendarFileProvider{
		basePath: basePath,
		cache:    map[int]map[string]string{},
	}
}

func (p *HolidayCalendarFileProvider) IsNonTradingDay(ctx context.Context, date time.Time) (bool, string, error) {
	select {
	case <-ctx.Done():
		return false, "", ctx.Err()
	default:
	}
	year := date.Year()
	holidays, err := p.loadYear(year)
	if err != nil {
		return false, "", err
	}
	reason, ok := holidays[date.Format("2006-01-02")]
	if !ok {
		return false, "", nil
	}
	return true, reason, nil
}

func (p *HolidayCalendarFileProvider) loadYear(year int) (map[string]string, error) {
	p.mu.RLock()
	if cached, ok := p.cache[year]; ok {
		p.mu.RUnlock()
		return cached, nil
	}
	p.mu.RUnlock()

	path := filepath.Join(p.basePath, fmt.Sprintf("%d.json", year))
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("missing holiday config for year %d at %s: %w", year, path, err)
	}
	var parsed HolidayCalendarYearFile
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("invalid holiday config %s: %w", path, err)
	}
	mapped := make(map[string]string, len(parsed.Holidays))
	for _, item := range parsed.Holidays {
		day := item.Date
		if day == "" {
			continue
		}
		reason := item.Type
		if reason == "" {
			reason = "PUBLIC_HOLIDAY"
		}
		mapped[day] = reason
	}

	p.mu.Lock()
	p.cache[year] = mapped
	p.mu.Unlock()
	return mapped, nil
}
