package platform_test

import (
	"testing"
	"time"

	"github.com/fenet-as/KnowFlow/internal/platform"
	"github.com/stretchr/testify/assert"
)

func TestParseRetryAfter(t *testing.T) {
	tests := []struct {
		name       string
		header     string
		wantOK     bool
		minVal     time.Duration
		maxVal     time.Duration
	}{
		{
			name:   "empty header",
			header: "",
			wantOK: false,
		},
		{
			name:   "invalid string",
			header: "not-a-number",
			wantOK: false,
		},
		{
			name:   "negative integer",
			header: "-5",
			wantOK: false,
		},
		{
			name:   "integer seconds 10",
			header: "10",
			wantOK: true,
			minVal: 10 * time.Second,
			maxVal: 10 * time.Second,
		},
		{
			name:   "float seconds 2.5",
			header: "2.5",
			wantOK: true,
			minVal: 2500 * time.Millisecond,
			maxVal: 2500 * time.Millisecond,
		},
		{
			name:   "RFC1123 date in the future",
			header: time.Now().Add(5 * time.Second).UTC().Format(time.RFC1123),
			wantOK: true,
			minVal: 3 * time.Second,
			maxVal: 7 * time.Second,
		},
		{
			name:   "date in the past",
			header: time.Now().Add(-5 * time.Second).UTC().Format(time.RFC1123),
			wantOK: true,
			minVal: 0,
			maxVal: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d, ok := platform.ParseRetryAfter(tt.header)
			assert.Equal(t, tt.wantOK, ok)
			if ok {
				assert.GreaterOrEqual(t, d, tt.minVal)
				assert.LessOrEqual(t, d, tt.maxVal)
			}
		})
	}
}
