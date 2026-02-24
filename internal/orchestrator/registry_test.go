package orchestrator_test

import (
	"testing"

	"open-sandbox/internal/database"
	"open-sandbox/internal/orchestrator"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestRegistry() *orchestrator.WorkerRegistry {
	db := database.New(":memory:")
	repo := database.NewRepository(db)
	return orchestrator.NewRegistry(repo)
}

func TestRegistry_RegisterAndAll(t *testing.T) {
	reg := newTestRegistry()

	id1, err := reg.Register("http://10.0.0.1:9090", "key1")
	require.NoError(t, err)
	assert.NotEmpty(t, id1)
	assert.Contains(t, id1, "wrk_")

	id2, err := reg.Register("http://10.0.0.2:9090", "key2")
	require.NoError(t, err)
	assert.NotEqual(t, id1, id2)

	all := reg.All()
	assert.Len(t, all, 2)
}

func TestRegistry_Deregister(t *testing.T) {
	reg := newTestRegistry()

	id, err := reg.Register("http://10.0.0.1:9090", "key")
	require.NoError(t, err)

	assert.Len(t, reg.All(), 1)

	require.NoError(t, reg.Deregister(id))
	assert.Len(t, reg.All(), 0)
}

func TestRegistry_ReActivate(t *testing.T) {
	reg := newTestRegistry()

	id1, err := reg.Register("http://10.0.0.1:9090", "key")
	require.NoError(t, err)

	require.NoError(t, reg.Deregister(id1))
	assert.Len(t, reg.All(), 0)

	// Re-register same URL should re-activate, return same ID.
	id2, err := reg.Register("http://10.0.0.1:9090", "new-key")
	require.NoError(t, err)
	assert.Equal(t, id1, id2)
	assert.Len(t, reg.All(), 1)
}

func TestRegistry_RoundRobin(t *testing.T) {
	reg := newTestRegistry()

	reg.Register("http://w1:9090", "k1")
	reg.Register("http://w2:9090", "k2")
	reg.Register("http://w3:9090", "k3")

	// Collect 9 calls — should distribute evenly.
	counts := map[string]int{}
	for range 9 {
		w, err := reg.Next()
		require.NoError(t, err)
		counts[w.URL]++
	}

	assert.Equal(t, 3, counts["http://w1:9090"])
	assert.Equal(t, 3, counts["http://w2:9090"])
	assert.Equal(t, 3, counts["http://w3:9090"])
}

func TestRegistry_NextNoWorkers(t *testing.T) {
	reg := newTestRegistry()

	_, err := reg.Next()
	assert.ErrorIs(t, err, orchestrator.ErrNoWorkers)
}

func TestRegistry_Lookup(t *testing.T) {
	reg := newTestRegistry()

	id, _ := reg.Register("http://10.0.0.1:9090", "key")

	w, err := reg.Lookup(id)
	require.NoError(t, err)
	assert.Equal(t, "http://10.0.0.1:9090", w.URL)
}

func TestRegistry_LookupNotFound(t *testing.T) {
	reg := newTestRegistry()

	_, err := reg.Lookup("wrk_nonexistent")
	assert.ErrorIs(t, err, orchestrator.ErrNoWorkers)
}
