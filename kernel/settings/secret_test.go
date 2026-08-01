package settings

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// TestSecretStringNoLeak is the Go half of the M0.4 anti-leak test
// (test_secrets_no_leak): it must be impossible to recover the raw secret
// through any of Go's normal formatting or serialization paths.
func TestSecretStringNoLeak(t *testing.T) {
	const fake = "sk-supersecrettoken1234567890"
	s := NewSecretString(fake)

	outputs := []string{
		fmt.Sprintf("%v", s),
		fmt.Sprintf("%s", s),
		fmt.Sprintf("%#v", s),
		s.String(),
		s.GoString(),
	}

	jsonBytes, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
	outputs = append(outputs, string(jsonBytes))

	textBytes, err := s.MarshalText()
	if err != nil {
		t.Fatalf("MarshalText failed: %v", err)
	}
	outputs = append(outputs, string(textBytes))

	for _, out := range outputs {
		if strings.Contains(out, fake) {
			t.Fatalf("secret leaked in output: %q", out)
		}
	}

	if s.Reveal() != fake {
		t.Fatal("Reveal() should return the underlying secret")
	}
}
