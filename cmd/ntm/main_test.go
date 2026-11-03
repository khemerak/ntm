package main

import "testing"

func TestSanitizeURL(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "removes playlist parameters",
			in:   "https://www.youtube.com/watch?v=abc123&list=PL123&index=2",
			want: "https://www.youtube.com/watch?v=abc123",
		},
		{
			name: "removes radio parameters",
			in:   "https://www.youtube.com/watch?v=abc123&start_radio=1&rv=abc123",
			want: "https://www.youtube.com/watch?v=abc123",
		},
		{
			name: "removes share tracking",
			in:   "https://youtu.be/abc123?si=tracking&t=30",
			want: "https://youtu.be/abc123?t=30",
		},
		{
			name: "adds missing scheme",
			in:   "youtu.be/abc123",
			want: "https://youtu.be/abc123",
		},
		{
			name: "keeps a clean url",
			in:   "https://www.youtube.com/watch?v=abc123",
			want: "https://www.youtube.com/watch?v=abc123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sanitizeURL(tt.in); got != tt.want {
				t.Fatalf("sanitizeURL(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
