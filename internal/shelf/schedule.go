package shelf

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"time"
)

type Schedule struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	StartDate string    `json:"startDate"`
	EndDate   string    `json:"endDate"`
	AtTime    string    `json:"atTime"`
	Notes     string    `json:"notes"`
	Done      bool      `json:"done"`
	CreatedAt time.Time `json:"createdAt"`
}
type ScheduleDraft struct {
	Title     string `json:"title"`
	StartDate string `json:"startDate"`
	EndDate   string `json:"endDate"`
	AtTime    string `json:"atTime"`
	Notes     string `json:"notes"`
}

func validateSchedule(d ScheduleDraft) (ScheduleDraft, error) {
	d.Title = strings.TrimSpace(d.Title)
	d.Notes = strings.TrimSpace(d.Notes)
	if d.Title == "" || len([]rune(d.Title)) > 200 {
		return d, errors.New("日程标题需要 1 到 200 字")
	}
	if len([]rune(d.Notes)) > 5000 {
		return d, errors.New("备注最多 5000 字")
	}
	if _, e := time.Parse("2006-01-02", d.StartDate); e != nil {
		return d, errors.New("请选择有效的开始日期")
	}
	if d.EndDate == "" {
		d.EndDate = d.StartDate
	}
	if _, e := time.Parse("2006-01-02", d.EndDate); e != nil {
		return d, errors.New("请选择有效的结束日期")
	}
	if d.EndDate < d.StartDate {
		return d, errors.New("结束日期不能早于开始日期")
	}
	if d.AtTime != "" {
		if _, e := time.Parse("15:04", d.AtTime); e != nil {
			return d, errors.New("请选择有效的时间")
		}
	}
	return d, nil
}
func (s *Store) SaveSchedule(id string, d ScheduleDraft) (Schedule, error) {
	d, err := validateSchedule(d)
	if err != nil {
		return Schedule{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	before := append([]Schedule{}, s.state.Schedules...)
	entry := Schedule{ID: id, Title: d.Title, StartDate: d.StartDate, EndDate: d.EndDate, AtTime: d.AtTime, Notes: d.Notes, CreatedAt: time.Now()}
	if id == "" {
		if len(before) >= 5000 {
			return Schedule{}, errors.New("最多保留 5000 条日程")
		}
		token := make([]byte, 16)
		if _, err = rand.Read(token); err != nil {
			return Schedule{}, err
		}
		entry.ID = hex.EncodeToString(token)
		s.state.Schedules = append(s.state.Schedules, entry)
	} else {
		found := false
		for n, old := range before {
			if old.ID == id {
				entry.Done = old.Done
				entry.CreatedAt = old.CreatedAt
				s.state.Schedules[n] = entry
				found = true
				break
			}
		}
		if !found {
			return Schedule{}, errors.New("日程不存在")
		}
	}
	if err = s.persist(); err != nil {
		s.state.Schedules = before
		return Schedule{}, err
	}
	return entry, nil
}
func (s *Store) SetScheduleDone(id string, done bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for n, v := range s.state.Schedules {
		if v.ID == id {
			s.state.Schedules[n].Done = done
			if err := s.persist(); err != nil {
				s.state.Schedules[n].Done = v.Done
				return err
			}
			return nil
		}
	}
	return errors.New("日程不存在")
}
