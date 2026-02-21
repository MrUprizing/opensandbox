package database

import "gorm.io/gorm"

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

// Delete removes a sandbox record by its container ID.
func (r *Repository) Delete(id string) error {
	return r.db.Delete(&Sandbox{}, "id = ?", id).Error
}
