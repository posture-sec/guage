package sanitize

import (
	"strings"
	"testing"
)

func TestOSC8HyperlinkConsumedWhole(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		in        string
		want      string
		wantLabel string
	}{
		{
			name:      "bel terminated",
			in:        "\x1b]8;;https://evil.example.com/payload\x07visible-label\x1b]8;;\x07",
			want:      "visible-label",
			wantLabel: "visible-label",
		},
		{
			name:      "st terminated",
			in:        "\x1b]8;;https://evil.example.com/payload\x1b\\visible-label\x1b]8;;\x1b\\",
			want:      "visible-label",
			wantLabel: "visible-label",
		},
		{
			name:      "embedded second esc",
			in:        "\x1b]8;;https://evil.example.com/\x1b[31mpayload\x1b\\visible-label\x1b]8;;\x1b\\",
			want:      "visible-label",
			wantLabel: "visible-label",
		},
		{
			name:      "unterminated tail",
			in:        "safe\x1b]8;;https://evil.example.com/unterminated-payload",
			want:      "safe",
			wantLabel: "safe",
		},
	}

	for _, tt := range cases {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := Text(tt.in)
			if got != tt.want {
				t.Fatalf("Text(%q) = %q, want %q", tt.in, got, tt.want)
			}
			if !strings.Contains(got, tt.wantLabel) {
				t.Errorf("output %q missing label %q", got, tt.wantLabel)
			}
			if strings.Contains(got, "]8;;") {
				t.Errorf("output %q contains ']8;;'", got)
			}
			if strings.Contains(got, "http") {
				t.Errorf("output %q contains 'http'", got)
			}
			if strings.Contains(got, "evil.example.com") {
				t.Errorf("output %q contains URI host 'evil.example.com'", got)
			}
		})
	}
}
