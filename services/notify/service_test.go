package notify

import (
	"testing"
	"time"
)

func clockAt(h, m int) *time.Time {
	t := time.Date(2000, 1, 1, h, m, 0, 0, time.UTC)
	return &t
}

func TestQuietHoursAcrossMidnight(t *testing.T) {
	r := recipient{quietFrom: clockAt(22, 0), quietTo: clockAt(7, 0)}
	at := func(h int) time.Time { return time.Date(2026, 10, 30, h, 30, 0, 0, riyadh) }
	if !r.quiet(at(23)) || !r.quiet(at(2)) || !r.quiet(at(6)) {
		t.Error("night hours should be quiet")
	}
	if r.quiet(at(7)) || r.quiet(at(12)) || r.quiet(at(21)) {
		t.Error("day hours should not be quiet")
	}
	day := recipient{quietFrom: clockAt(13, 0), quietTo: clockAt(16, 0)}
	if !day.quiet(at(14)) || day.quiet(at(17)) {
		t.Error("same-day window")
	}
	if (recipient{}).quiet(at(3)) {
		t.Error("no window means never quiet")
	}
}

func TestWants(t *testing.T) {
	r := recipient{critical: []string{"push", "sms"}, warning: []string{"push"}}
	if !r.wants("critical", "sms") || r.wants("warning", "sms") || !r.wants("warning", "push") {
		t.Error("channel choice by severity")
	}
}
