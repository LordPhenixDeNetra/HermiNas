package schemamgr

import "testing"

func columnByName(cols []Column, name string) (Column, bool) {
	for _, c := range cols {
		if c.Name == name {
			return c, true
		}
	}
	return Column{}, false
}

func TestInferColumnsMapsBasicJSONTypes(t *testing.T) {
	sample := map[string]any{
		"level":    "error",
		"count":    float64(3),
		"ratio":    float64(0.5),
		"ok":       true,
		"nothing":  nil,
		"metadata": map[string]any{"nested": "value"},
		"tags":     []any{"a", "b"},
	}

	cols := InferColumns(sample)
	if len(cols) != len(sample) {
		t.Fatalf("expected %d columns, got %d", len(sample), len(cols))
	}

	cases := map[string]struct {
		wantType     string
		wantNullable bool
	}{
		"level":    {"String", false},
		"count":    {"Int64", false},
		"ratio":    {"Float64", false},
		"ok":       {"Bool", false},
		"nothing":  {"String", true},
		"metadata": {"String", false},
		"tags":     {"String", false},
	}

	for name, want := range cases {
		col, ok := columnByName(cols, name)
		if !ok {
			t.Fatalf("expected column %q", name)
		}
		if col.Type != want.wantType {
			t.Errorf("%s: expected type %s, got %s", name, want.wantType, col.Type)
		}
		if col.Nullable != want.wantNullable {
			t.Errorf("%s: expected nullable=%v, got %v", name, want.wantNullable, col.Nullable)
		}
	}
}

func TestInferColumnsIsSortedByName(t *testing.T) {
	cols := InferColumns(map[string]any{"zebra": "z", "alpha": "a", "middle": "m"})
	if len(cols) != 3 || cols[0].Name != "alpha" || cols[1].Name != "middle" || cols[2].Name != "zebra" {
		t.Fatalf("expected sorted [alpha middle zebra], got %+v", cols)
	}
}
