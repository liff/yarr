package storage

import (
	"testing"
)

func testDB() *Storage {
	db, _ := New(":memory:")
	return db
}

func TestStorage(t *testing.T) {
	db, err := New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	if db == nil {
		t.Fatal("no db")
	}
}
