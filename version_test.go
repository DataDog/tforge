package tforge

import (
	"runtime/debug"
	"testing"
)

func TestResolveVersion(t *testing.T) {
	tests := []struct {
		name     string
		injected string
		info     *debug.BuildInfo
		infoOK   bool
		want     string
	}{
		{
			name:     "ldflag override wins",
			injected: "v1.2.3",
			info:     &debug.BuildInfo{Main: debug.Module{Version: "v9.9.9"}},
			infoOK:   true,
			want:     "v1.2.3",
		},
		{
			name:     "module-cache install: Main.Version is the tag",
			injected: "",
			info:     &debug.BuildInfo{Main: debug.Module{Version: "v0.3.0"}},
			infoOK:   true,
			want:     "v0.3.0",
		},
		{
			name:     "in-tree build: vcs.revision gives short SHA",
			injected: "",
			info: &debug.BuildInfo{
				Main: debug.Module{Version: "(devel)"},
				Settings: []debug.BuildSetting{
					{Key: "vcs.revision", Value: "abc1234deadbeef"},
				},
			},
			infoOK: true,
			want:   "dev+abc1234",
		},
		{
			name:     "in-tree build with dirty tree",
			injected: "",
			info: &debug.BuildInfo{
				Main: debug.Module{Version: "(devel)"},
				Settings: []debug.BuildSetting{
					{Key: "vcs.revision", Value: "abc1234deadbeef"},
					{Key: "vcs.modified", Value: "true"},
				},
			},
			infoOK: true,
			want:   "dev+abc1234+dirty",
		},
		{
			name:     "in-tree build, vcs.modified=false stays clean",
			injected: "",
			info: &debug.BuildInfo{
				Main: debug.Module{Version: "(devel)"},
				Settings: []debug.BuildSetting{
					{Key: "vcs.revision", Value: "abc1234deadbeef"},
					{Key: "vcs.modified", Value: "false"},
				},
			},
			infoOK: true,
			want:   "dev+abc1234",
		},
		{
			name:     "no BuildInfo: dev sentinel",
			injected: "",
			info:     nil,
			infoOK:   false,
			want:     "dev",
		},
		{
			name:     "BuildInfo present but no version and no revision: dev sentinel",
			injected: "",
			info:     &debug.BuildInfo{Main: debug.Module{Version: "(devel)"}},
			infoOK:   true,
			want:     "dev",
		},
		{
			name:     "vcs.revision too short to take 7 chars: dev sentinel",
			injected: "",
			info: &debug.BuildInfo{
				Main: debug.Module{Version: "(devel)"},
				Settings: []debug.BuildSetting{
					{Key: "vcs.revision", Value: "abc"},
				},
			},
			infoOK: true,
			want:   "dev",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stub := func() (*debug.BuildInfo, bool) {
				return tt.info, tt.infoOK
			}
			got := resolveVersion(tt.injected, stub)
			if got != tt.want {
				t.Errorf("resolveVersion(injected=%q) = %q, want %q", tt.injected, got, tt.want)
			}
		})
	}
}
