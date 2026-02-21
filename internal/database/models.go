package database

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
)

// JSONMap is a map[string]string that serializes to/from JSON in SQLite.
type JSONMap map[string]string

func (j JSONMap) Value() (driver.Value, error) {
	if j == nil {
		return "null", nil
	}
	b, err := json.Marshal(j)
	return string(b), err
}

func (j *JSONMap) Scan(src any) error {
	if src == nil {
		*j = nil
		return nil
	}
	var bytes []byte
	switch v := src.(type) {
	case string:
		bytes = []byte(v)
	case []byte:
		bytes = v
	default:
		return fmt.Errorf("unsupported type for JSONMap: %T", src)
	}
	return json.Unmarshal(bytes, j)
}

// Sandbox persists the container ID, metadata, and its assigned host ports.
type Sandbox struct {
	ID    string `gorm:"primaryKey"` // Docker container ID
	Name  string
	Image string
	Ports JSONMap `gorm:"type:json"` // e.g. {"80/tcp": "32768"}
}
