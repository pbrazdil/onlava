package runtime

import (
	"context"
	"testing"
	"time"
)

func TestEveryCronPlanAlignsToUTCGrid(t *testing.T) {
	plan := everyCronPlan{interval: 6 * time.Hour}
	got := plan.Next(time.Date(2026, time.April, 14, 7, 10, 0, 0, time.UTC))
	want := time.Date(2026, time.April, 14, 12, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("Next() = %s, want %s", got, want)
	}
}

func TestParseCronScheduleSupportsNamesAndSteps(t *testing.T) {
	plan, err := parseCronSchedule("*/15 9-17 * * MON-FRI")
	if err != nil {
		t.Fatalf("parseCronSchedule returned error: %v", err)
	}
	got := plan.Next(time.Date(2026, time.April, 13, 8, 59, 0, 0, time.UTC))
	want := time.Date(2026, time.April, 13, 9, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("Next() = %s, want %s", got, want)
	}
}

func TestValidateCronJobRequiresExactlyOneScheduleMode(t *testing.T) {
	err := validateCronJob(&CronJob{
		ID:       "tick",
		Every:    time.Minute,
		Schedule: "* * * * *",
		Invoke:   func(context.Context) error { return nil },
	})
	if err == nil {
		t.Fatal("validateCronJob returned nil error")
	}
}
