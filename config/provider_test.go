package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestProviderAddress(t *testing.T) {
	tests := []struct {
		name     string
		provider Provider
		want     string
	}{
		{
			name:     "without alias",
			provider: Provider{Name: "aws"},
			want:     "aws",
		},
		{
			name:     "with alias",
			provider: Provider{Name: "aws", Alias: "west"},
			want:     "aws.west",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.provider.Address()
			require.Equal(t, tt.want, got, "Address() = %q, want %q", got, tt.want)
		})
	}
}
