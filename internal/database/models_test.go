package database

import (
	"encoding/json"
	"testing"
)

func TestJSONMapValue(t *testing.T) {
	var nilMap JSONMap
	v, err := nilMap.Value()
	if err != nil {
		t.Fatalf("Value() error: %v", err)
	}
	if v != "null" {
		t.Fatalf("Value() = %v, want null", v)
	}

	m := JSONMap{"3000/tcp": "32768"}
	v, err = m.Value()
	if err != nil {
		t.Fatalf("Value() error: %v", err)
	}

	var decoded map[string]string
	if err := json.Unmarshal([]byte(v.(string)), &decoded); err != nil {
		t.Fatalf("unmarshal value: %v", err)
	}
	if decoded["3000/tcp"] != "32768" {
		t.Fatalf("decoded value mismatch: %v", decoded)
	}
}

func TestJSONMapScan(t *testing.T) {
	var m JSONMap

	if err := m.Scan(nil); err != nil {
		t.Fatalf("Scan(nil) error: %v", err)
	}
	if m != nil {
		t.Fatalf("Scan(nil) should keep map nil")
	}

	if err := m.Scan("{\"80/tcp\":\"32000\"}"); err != nil {
		t.Fatalf("Scan(string) error: %v", err)
	}
	if m["80/tcp"] != "32000" {
		t.Fatalf("Scan(string) mismatch: %v", m)
	}

	if err := m.Scan([]byte("{\"443/tcp\":\"32001\"}")); err != nil {
		t.Fatalf("Scan([]byte) error: %v", err)
	}
	if m["443/tcp"] != "32001" {
		t.Fatalf("Scan([]byte) mismatch: %v", m)
	}

	if err := m.Scan(123); err == nil {
		t.Fatalf("Scan(invalid type) expected error")
	}
}
