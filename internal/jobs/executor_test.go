package jobs

import "testing"

func TestNormalizeImageRef(t *testing.T) {
	tests := []struct {
		name           string
		imageRef        string
		registryPrefix  string
		want            string
	}{
		{
			name:          "empty prefix keeps short ref",
			imageRef:       "nginx:alpine",
			registryPrefix: "",
			want:           "nginx:alpine",
		},
		{
			name:          "prefix expands short ref",
			imageRef:       "nginx:alpine",
			registryPrefix: "docker.io/library",
			want:           "docker.io/library/nginx:alpine",
		},
		{
			name:          "prefix expands openclaw short ref",
			imageRef:       "openclaw:latest",
			registryPrefix: "ghcr.io/openclaw",
			want:           "ghcr.io/openclaw/openclaw:latest",
		},
		{
			name:          "explicit registry wins",
			imageRef:       "ghcr.io/openclaw/openclaw:latest",
			registryPrefix: "docker.io/library",
			want:           "ghcr.io/openclaw/openclaw:latest",
		},
		{
			name:          "localhost registry wins",
			imageRef:       "localhost:5000/openclaw:latest",
			registryPrefix: "docker.io/library",
			want:           "localhost:5000/openclaw:latest",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeImageRef(tt.imageRef, tt.registryPrefix)
			if got != tt.want {
				t.Fatalf("normalizeImageRef(%q, %q) = %q, want %q", tt.imageRef, tt.registryPrefix, got, tt.want)
			}
		})
	}
}
