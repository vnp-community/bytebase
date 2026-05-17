package store

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/pkg/errors"
)

func TestIsRetryable(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "serialization failure",
			err:  &pgconn.PgError{Code: "40001"},
			want: true,
		},
		{
			name: "unique violation",
			err:  &pgconn.PgError{Code: "23505"},
			want: false,
		},
		{
			name: "wrapped pg error",
			err:  errors.Wrap(&pgconn.PgError{Code: "08000"}, "connection error"),
			want: true,
		},
		{
			name: "connection refused string",
			err:  errors.New("dial tcp: connection refused"),
			want: true,
		},
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isRetryable(tt.err); got != tt.want {
				t.Errorf("isRetryable() = %v, want %v", got, tt.want)
			}
		})
	}
}
