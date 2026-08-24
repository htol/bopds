package config

import (
	"reflect"
	"testing"
)

func TestLoad_LibraryPathList(t *testing.T) {
	tests := []struct {
		name  string
		value string
		set   bool
		want  []string
	}{
		{
			name:  "path-style list",
			value: "/r1:/r2",
			set:   true,
			want:  []string{"/r1", "/r2"},
		},
		{
			name:  "single value",
			value: "/r1",
			set:   true,
			want:  []string{"/r1"},
		},
		{
			name:  "empty segments dropped",
			value: "/r1::/r2",
			set:   true,
			want:  []string{"/r1", "/r2"},
		},
		{
			name: "unset defaults to ./lib",
			set:  false,
			want: []string{"./lib"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.set {
				t.Setenv("LIBRARY_PATH", tt.value)
			} else {
				t.Setenv("LIBRARY_PATH", "")
			}

			cfg := Load()

			if !reflect.DeepEqual(cfg.Library.Paths, tt.want) {
				t.Errorf("Expected Paths %v, got %v", tt.want, cfg.Library.Paths)
			}
		})
	}
}
