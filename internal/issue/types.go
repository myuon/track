package issue

import (
	"fmt"
	"strings"
	"time"
)

const (
	StatusTodo       = "todo"
	StatusReady      = "ready"
	StatusInProgress = "in_progress"
	StatusDone       = "done"
	StatusArchived   = "archived"
)

var validStatuses = map[string]struct{}{
	StatusTodo:       {},
	StatusReady:      {},
	StatusInProgress: {},
	StatusDone:       {},
	StatusArchived:   {},
}

var validPriorities = map[string]struct{}{
	"p0": {},
	"p1": {},
	"p2": {},
	"p3": {},
}

type Item struct {
	ID         string
	Title      string
	Status     string
	Priority   string
	Assignee   string
	Due        string
	Labels     []string
	NextAction string
	Body       string
	CreatedAt  string
	UpdatedAt  string
}

func ValidateStatus(v string) error {
	if _, ok := validStatuses[v]; !ok {
		return fmt.Errorf("invalid status: %s", v)
	}
	return nil
}

func ValidatePriority(v string) error {
	if _, ok := validPriorities[v]; !ok {
		return fmt.Errorf("invalid priority: %s", v)
	}
	return nil
}

func ValidateDue(v string) error {
	if v == "" {
		return nil
	}
	_, err := time.Parse("2006-01-02", v)
	if err != nil {
		return fmt.Errorf("invalid due date (YYYY-MM-DD): %s", v)
	}
	return nil
}

func NormalizeAssignee(v string) string {
	if v == "none" {
		return ""
	}
	return strings.TrimSpace(v)
}
