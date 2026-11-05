package api

import (
	"testing"

	"github.com/alecthomas/assert/v2"
)

func TestProfile_Get(t *testing.T) {
	client, err := NewClient("http://127.0.0.1:7890")
	assert.NoError(t, err)

	result, err := NewProfile(client).Get()

	assert.NoError(t, err)
	assert.NotZero(t, result)
	assert.NotEqual(t, 0, result.ID)
	t.Logf("%+v", result)
}
