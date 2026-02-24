package database

import (
	"gorm.io/gorm"
)

// Repository provides CRUD operations for persisted sandboxes.
type Repository struct {
	db *gorm.DB
}

// NewRepository creates a Repository backed by the given database.
func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// Save creates or updates a sandbox record.
func (r *Repository) Save(s Sandbox) error {
	return r.db.Save(&s).Error
}

// FindByID returns a sandbox by its container ID, or nil if not found.
func (r *Repository) FindByID(id string) (*Sandbox, error) {
	var s Sandbox
	if err := r.db.First(&s, "id = ?", id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &s, nil
}

// FindAll returns all persisted sandboxes.
func (r *Repository) FindAll() ([]Sandbox, error) {
	var sandboxes []Sandbox
	if err := r.db.Find(&sandboxes).Error; err != nil {
		return nil, err
	}
	return sandboxes, nil
}

// UpdatePorts updates the port mappings for an existing sandbox.
func (r *Repository) UpdatePorts(id string, ports JSONMap) error {
	return r.db.Model(&Sandbox{}).Where("id = ?", id).Update("ports", ports).Error
}

// FindByName returns a sandbox by its name, or nil if not found.
func (r *Repository) FindByName(name string) (*Sandbox, error) {
	var s Sandbox
	if err := r.db.First(&s, "name = ?", name).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &s, nil
}

// Delete removes a sandbox record by its container ID.
func (r *Repository) Delete(id string) error {
	return r.db.Delete(&Sandbox{}, "id = ?", id).Error
}

// SaveCommand creates a new command record.
func (r *Repository) SaveCommand(cmd Command) error {
	return r.db.Create(&cmd).Error
}

// FindCommandByID returns a command by ID, or nil if not found.
func (r *Repository) FindCommandByID(id string) (*Command, error) {
	var cmd Command
	if err := r.db.First(&cmd, "id = ?", id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &cmd, nil
}

// FindCommandsBySandbox returns all commands for a sandbox, ordered by started_at.
func (r *Repository) FindCommandsBySandbox(sandboxID string) ([]Command, error) {
	var cmds []Command
	if err := r.db.Where("sandbox_id = ?", sandboxID).Order("started_at ASC").Find(&cmds).Error; err != nil {
		return nil, err
	}
	return cmds, nil
}

// UpdateCommandFinished marks a command as finished with its exit code.
func (r *Repository) UpdateCommandFinished(id string, exitCode int, finishedAt int64) error {
	return r.db.Model(&Command{}).Where("id = ?", id).Updates(map[string]any{
		"exit_code":   exitCode,
		"finished_at": finishedAt,
	}).Error
}

// DeleteCommandsBySandbox removes all command records for a sandbox.
func (r *Repository) DeleteCommandsBySandbox(sandboxID string) error {
	return r.db.Where("sandbox_id = ?", sandboxID).Delete(&Command{}).Error
}

// --- Worker operations ---

// SaveWorker creates or updates a worker record.
func (r *Repository) SaveWorker(w Worker) error {
	return r.db.Save(&w).Error
}

// FindWorkerByID returns a worker by ID, or nil if not found.
func (r *Repository) FindWorkerByID(id string) (*Worker, error) {
	var w Worker
	if err := r.db.First(&w, "id = ?", id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &w, nil
}

// FindWorkerByURL returns a worker by URL, or nil if not found.
func (r *Repository) FindWorkerByURL(url string) (*Worker, error) {
	var w Worker
	if err := r.db.First(&w, "url = ?", url).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &w, nil
}

// FindActiveWorkers returns all workers with status "active".
func (r *Repository) FindActiveWorkers() ([]Worker, error) {
	var workers []Worker
	if err := r.db.Where("status = ?", "active").Find(&workers).Error; err != nil {
		return nil, err
	}
	return workers, nil
}

// DeleteWorker removes a worker record by ID.
func (r *Repository) DeleteWorker(id string) error {
	return r.db.Delete(&Worker{}, "id = ?", id).Error
}

// UpdateWorkerStatus changes a worker's status.
func (r *Repository) UpdateWorkerStatus(id, status string) error {
	return r.db.Model(&Worker{}).Where("id = ?", id).Update("status", status).Error
}

// FindSandboxWorker returns the sandbox joined with its worker URL.
// Returns the sandbox and the worker URL, or nil if not found.
func (r *Repository) FindSandboxWorker(sandboxID string) (*Sandbox, string, error) {
	var sb Sandbox
	if err := r.db.First(&sb, "id = ?", sandboxID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, "", nil
		}
		return nil, "", err
	}
	if sb.WorkerID == "" {
		return &sb, "", nil
	}
	w, err := r.FindWorkerByID(sb.WorkerID)
	if err != nil {
		return nil, "", err
	}
	if w == nil {
		return &sb, "", nil
	}
	return &sb, w.URL, nil
}
