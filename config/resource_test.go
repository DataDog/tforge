package config

import (
	"testing"
)

func TestResourceAddress(t *testing.T) {
	tests := []struct {
		name string
		res  Resource
		want string
	}{
		{
			name: "aws instance",
			res:  Resource{Type: "aws_instance", Name: "web"},
			want: "aws_instance.web",
		},
		{
			name: "s3 bucket",
			res:  Resource{Type: "aws_s3_bucket", Name: "data"},
			want: "aws_s3_bucket.data",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.res.Address()
			if got != tt.want {
				t.Fatalf("Address() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestResourceRef(t *testing.T) {
	tests := []struct {
		name string
		res  Resource
		attr string
		want string
	}{
		{
			name: "id attribute",
			res:  Resource{Type: "aws_instance", Name: "web"},
			attr: "id",
			want: "${aws_instance.web.id}",
		},
		{
			name: "public_ip attribute",
			res:  Resource{Type: "aws_instance", Name: "web"},
			attr: "public_ip",
			want: "${aws_instance.web.public_ip}",
		},
		{
			name: "arn attribute",
			res:  Resource{Type: "aws_s3_bucket", Name: "data"},
			attr: "arn",
			want: "${aws_s3_bucket.data.arn}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.res.Ref(tt.attr)
			if got != tt.want {
				t.Fatalf("Ref(%q) = %q, want %q", tt.attr, got, tt.want)
			}
		})
	}
}

func TestDataSourceAddress(t *testing.T) {
	tests := []struct {
		name string
		ds   DataSource
		want string
	}{
		{
			name: "aws ami",
			ds:   DataSource{Type: "aws_ami", Name: "latest"},
			want: "data.aws_ami.latest",
		},
		{
			name: "aws vpc",
			ds:   DataSource{Type: "aws_vpc", Name: "main"},
			want: "data.aws_vpc.main",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.ds.Address()
			if got != tt.want {
				t.Fatalf("Address() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDataSourceRef(t *testing.T) {
	tests := []struct {
		name string
		ds   DataSource
		attr string
		want string
	}{
		{
			name: "id attribute",
			ds:   DataSource{Type: "aws_ami", Name: "latest"},
			attr: "id",
			want: "${data.aws_ami.latest.id}",
		},
		{
			name: "cidr_block attribute",
			ds:   DataSource{Type: "aws_vpc", Name: "main"},
			attr: "cidr_block",
			want: "${data.aws_vpc.main.cidr_block}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.ds.Ref(tt.attr)
			if got != tt.want {
				t.Fatalf("Ref(%q) = %q, want %q", tt.attr, got, tt.want)
			}
		})
	}
}
