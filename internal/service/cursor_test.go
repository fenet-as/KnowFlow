package service

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCursor_EncodeDecode(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Microsecond)
	id := uuid.New()

	encoded := EncodeCursor(now, id)
	assert.NotEmpty(t, encoded)

	decodedTime, decodedID, err := DecodeCursor(encoded)
	require.NoError(t, err)
	require.NotNil(t, decodedTime)
	require.NotNil(t, decodedID)

	assert.Equal(t, id, *decodedID)
	assert.True(t, now.Equal(*decodedTime), "expected %v, got %v", now, *decodedTime)
}

func TestCursor_Empty(t *testing.T) {
	decodedTime, decodedID, err := DecodeCursor("")
	require.NoError(t, err)
	assert.Nil(t, decodedTime)
	assert.Nil(t, decodedID)
}

func TestCursor_Malformed(t *testing.T) {
	tests := []struct {
		name   string
		cursor string
	}{
		{"invalid base64", "!!!not-base64!!!"},
		{"missing comma", "MjAyNi0xMS0wN1QxMDoxNTowMFo="},
		{"invalid timestamp", "aW52YWxpZC10aW1lLGZhYmFjZDY1LTAwMDAtNDAwMC04MDAwLTAwMDAwMDAwMDAwMQ=="},
		{"invalid uuid", "MjAyNi0xMS0wN1QxMDoxNTowMFosbm90LWEtdXVpZA=="},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			decodedTime, decodedID, err := DecodeCursor(tc.cursor)
			assert.Error(t, err)
			assert.Nil(t, decodedTime)
			assert.Nil(t, decodedID)
		})
	}
}
