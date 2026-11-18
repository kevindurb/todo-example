package todo

import (
	"errors"
	"strings"
	"testing"
)

func TestValidateAcceptsValidTodo(t *testing.T) {
	got, err := Validate(Input{Title: "  Buy milk  ", Notes: "  two litres  "})
	if err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
	if got.Title != "Buy milk" {
		t.Errorf("Title = %q, want trimmed value", got.Title)
	}
	if got.Notes != "  two litres  " {
		t.Errorf("Notes = %q, want the submitted value preserved", got.Notes)
	}
}

func TestValidateRejectsBlankTitle(t *testing.T) {
	for _, title := range []string{"", "   ", "\t\n"} {
		_, err := Validate(Input{Title: title, Notes: "notes"})
		var invalid *ValidationError
		if !errors.As(err, &invalid) {
			t.Fatalf("Validate(%q) error = %v, want *ValidationError", title, err)
		}
		if _, ok := invalid.Fields["title"]; !ok {
			t.Errorf("Validate(%q) did not report the title field", title)
		}
	}
}

func TestValidateRejectsOversizedTitle(t *testing.T) {
	_, err := Validate(Input{Title: strings.Repeat("a", MaxTitleLength+1)})
	var invalid *ValidationError
	if !errors.As(err, &invalid) {
		t.Fatalf("error = %v, want *ValidationError", err)
	}
	if _, ok := invalid.Fields["title"]; !ok {
		t.Error("did not report the title field")
	}
}

func TestValidateRejectsOversizedNotes(t *testing.T) {
	_, err := Validate(Input{Title: "ok", Notes: strings.Repeat("a", MaxNotesLength+1)})
	var invalid *ValidationError
	if !errors.As(err, &invalid) {
		t.Fatalf("error = %v, want *ValidationError", err)
	}
	if _, ok := invalid.Fields["notes"]; !ok {
		t.Error("did not report the notes field")
	}
}

func TestValidateAcceptsFieldLimits(t *testing.T) {
	_, err := Validate(Input{
		Title: strings.Repeat("a", MaxTitleLength),
		Notes: strings.Repeat("b", MaxNotesLength),
	})
	if err != nil {
		t.Fatalf("Validate at the limits returned error: %v", err)
	}
}

func TestValidateCountsCharactersNotBytes(t *testing.T) {
	// Each rune is three bytes in UTF-8; at the character limit this is fine.
	_, err := Validate(Input{Title: strings.Repeat("é", MaxTitleLength)})
	if err != nil {
		t.Fatalf("Validate with multi-byte characters returned error: %v", err)
	}
}
