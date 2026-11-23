package repositories

import (
	"testing"
)

func TestListRejectsInvalidBoundsBeforeDatabaseAccess(t *testing.T) {
	repository := &GormPaymentRepository{}

	tests := []struct {
		name   string
		limit  int
		offset int
	}{
		{name: "zero limit", limit: 0},
		{name: "negative limit", limit: -1},
		{name: "limit above maximum", limit: MaxPaymentPageSize + 1},
		{name: "negative offset", limit: 1, offset: -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := repository.List(tt.limit, tt.offset); err == nil {
				t.Fatal("List() error = nil, want invalid bounds error")
			}
		})
	}
}
