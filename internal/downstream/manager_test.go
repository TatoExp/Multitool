package downstream

import (
	"os"
	"testing"
)

func TestExpandEnv(t *testing.T) {
	// Set up test env vars.
	os.Setenv("EXISTS", "hello")
	os.Setenv("EMPTY", "")
	defer func() {
		os.Unsetenv("EXISTS")
		os.Unsetenv("EMPTY")
	}()

	tests := []struct {
		name string
		in   string
		want string
	}{
		{"no vars", "plain text", "plain text"},
		{"simple $var", "$EXISTS world", "hello world"},
		{"brace ${var}", "${EXISTS}", "hello"},
		{"missing var", "$MISSING", ""},
		{"missing with default", "${MISSING:-fallback}", "fallback"},
		{"empty with default", "${EMPTY:-fallback}", "fallback"},
		{"set with default", "${EXISTS:-fallback}", "hello"},
		{"literal dollar", "5$$", "5$$"},
		{"mixed", "prefix_${EXISTS}_suffix", "prefix_hello_suffix"},
		{"multiple", "$EXISTS and ${EXISTS}", "hello and hello"},
		{"no closing brace", "${EXISTS", "${EXISTS"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := expandEnv(tc.in)
			if got != tc.want {
				t.Fatalf("expandEnv(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
