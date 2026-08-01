package settings

import "encoding/json"

const redacted = "***REDACTED***"

// SecretString never reveals its value through the normal Go formatting or
// serialization paths (String, GoString, JSON, text). Reveal is the only
// way out, and it is meant to be called at the point of use (e.g. building
// a TLS config or an LLM client), never logged.
type SecretString struct {
	value string
}

func NewSecretString(v string) SecretString {
	return SecretString{value: v}
}

func (s SecretString) String() string   { return redacted }
func (s SecretString) GoString() string { return "settings.SecretString(" + redacted + ")" }

func (s SecretString) MarshalJSON() ([]byte, error) {
	return json.Marshal(redacted)
}

func (s SecretString) MarshalText() ([]byte, error) {
	return []byte(redacted), nil
}

func (s *SecretString) UnmarshalJSON(b []byte) error {
	var raw string
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	s.value = raw
	return nil
}

func (s *SecretString) UnmarshalText(b []byte) error {
	s.value = string(b)
	return nil
}

// Reveal returns the underlying secret. Call sites must never log or print
// the result directly.
func (s SecretString) Reveal() string { return s.value }
