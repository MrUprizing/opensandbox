package orchestrator

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"open-sandbox/internal/database"
)

var ErrNoWorkers = errors.New("no active workers available")

// WorkerInfo holds in-memory worker metadata for scheduling.
type WorkerInfo struct {
	ID     string
	URL    string
	APIKey string
}

// WorkerRegistry manages worker registration and round-robin scheduling.
type WorkerRegistry struct {
	repo    *database.Repository
	mu      sync.RWMutex
	workers []WorkerInfo // cached active workers
	counter atomic.Uint64
}

// NewRegistry creates a WorkerRegistry and loads active workers from DB.
func NewRegistry(repo *database.Repository) *WorkerRegistry {
	r := &WorkerRegistry{repo: repo}
	r.reload()
	return r
}

// Register adds a new worker or re-activates an existing one.
// Returns the worker ID.
func (r *WorkerRegistry) Register(url, apiKey string) (string, error) {
	// Check if worker with this URL already exists.
	existing, err := r.repo.FindWorkerByURL(url)
	if err != nil {
		return "", err
	}

	if existing != nil {
		// Re-activate existing worker.
		existing.Status = "active"
		existing.APIKey = apiKey
		if err := r.repo.SaveWorker(*existing); err != nil {
			return "", err
		}
		r.reload()
		return existing.ID, nil
	}

	// Create new worker.
	id := generateWorkerID()
	w := database.Worker{
		ID:        id,
		URL:       url,
		APIKey:    apiKey,
		Status:    "active",
		CreatedAt: time.Now().UnixMilli(),
	}
	if err := r.repo.SaveWorker(w); err != nil {
		return "", err
	}
	r.reload()
	return id, nil
}

// Deregister marks a worker as inactive and reloads the cache.
func (r *WorkerRegistry) Deregister(id string) error {
	if err := r.repo.UpdateWorkerStatus(id, "inactive"); err != nil {
		return err
	}
	r.reload()
	return nil
}

// Next returns the next worker via round-robin.
func (r *WorkerRegistry) Next() (WorkerInfo, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if len(r.workers) == 0 {
		return WorkerInfo{}, ErrNoWorkers
	}

	idx := r.counter.Add(1) - 1
	w := r.workers[idx%uint64(len(r.workers))]
	return w, nil
}

// Lookup returns the worker info for a given worker ID.
func (r *WorkerRegistry) Lookup(workerID string) (WorkerInfo, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, w := range r.workers {
		if w.ID == workerID {
			return w, nil
		}
	}
	return WorkerInfo{}, ErrNoWorkers
}

// All returns all active workers.
func (r *WorkerRegistry) All() []WorkerInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]WorkerInfo, len(r.workers))
	copy(out, r.workers)
	return out
}

// reload refreshes the in-memory worker list from the database.
func (r *WorkerRegistry) reload() {
	workers, err := r.repo.FindActiveWorkers()
	if err != nil {
		return
	}

	infos := make([]WorkerInfo, 0, len(workers))
	for _, w := range workers {
		infos = append(infos, WorkerInfo{
			ID:     w.ID,
			URL:    w.URL,
			APIKey: w.APIKey,
		})
	}

	r.mu.Lock()
	r.workers = infos
	r.mu.Unlock()
}

func generateWorkerID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return "wrk_" + hex.EncodeToString(b)
}
