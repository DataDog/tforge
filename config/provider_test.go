package config

import (
	"testing"
)

func TestProviderFullSource(t *testing.T) {
	tests := []struct {
		name     string
		provider Provider
		want     string
	}{
		{
			name:     "with explicit source",
			provider: Provider{Name: "aws", Source: "hashicorp/aws"},
			want:     "hashicorp/aws",
		},
		{
			name:     "with custom source",
			provider: Provider{Name: "datadog", Source: "DataDog/datadog"},
			want:     "DataDog/datadog",
		},
		{
			name:     "without source defaults to hashicorp",
			provider: Provider{Name: "aws"},
			want:     "hashicorp/aws",
		},
		{
			name:     "without source defaults to hashicorp for google",
			provider: Provider{Name: "google"},
			want:     "hashicorp/google",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.provider.FullSource()
			if got != tt.want {
				t.Fatalf("FullSource() = %q, want %q", got, tt.want)
			}
		})
	}
}

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
			if got != tt.want {
				t.Fatalf("Address() = %q, want %q", got, tt.want)
			}
		})
	}
}
