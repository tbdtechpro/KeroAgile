package store_test

import (
	"testing"

	"github.com/tbdtechpro/KeroAgile/internal/store"
)

func testStore(t *testing.T) *store.Store {
	t.Helper()
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	s := store.New(db)
	t.Cleanup(func() { s.Close() })
	return s
}
