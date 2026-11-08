package embed

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFakeEmbedder(t *testing.T) {
	fake := NewFakeEmbedder(768)
	assert.Equal(t, 768, fake.Dimension())

	ctx := context.Background()

	// 1. Test embedding generation
	vecs, err := fake.Embed(ctx, []string{"alpha", "beta"}, EmbedKindDocument)
	require.NoError(t, err)
	assert.Len(t, vecs, 2)
	assert.Len(t, vecs[0], 768)
	assert.Len(t, vecs[1], 768)
	assert.Equal(t, 1, fake.Calls())
	assert.Equal(t, EmbedKindDocument, fake.LastKind())
	assert.Equal(t, []string{"alpha", "beta"}, fake.LastTexts())

	// 2. Determinism: same input produces identical vector
	vecs2, err := fake.Embed(ctx, []string{"alpha"}, EmbedKindDocument)
	require.NoError(t, err)
	assert.Equal(t, vecs[0], vecs2[0])

	// 3. Different input produces different vector
	assert.NotEqual(t, vecs[0], vecs[1])

	// 4. Injected error works
	testErr := errors.New("injected network error")
	fake.SetError(testErr)
	_, err = fake.Embed(ctx, []string{"gamma"}, EmbedKindQuery)
	assert.ErrorIs(t, err, testErr)
}
