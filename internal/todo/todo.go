// Package todo defines the todo record and its validation rules.
package todo

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	// MaxTitleLength is the maximum accepted title length in characters.
	MaxTitleLength = 200
	// MaxNotesLength is the maximum accepted notes length in characters.
	MaxNotesLength = 2000
)

// Todo is a single todo record. ID and the timestamps are assigned by the
// store and never by the caller.
type Todo struct {
	ID        int64
	Title     string
	Notes     string
	Done      bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Input is user-supplied todo content before validation.
type Input struct {
	Title string
	Notes string
}

// ValidationError reports which submitted fields were rejected.
type ValidationError struct {
	Fields map[string]string
}

func (e *ValidationError) Error() string {
	return "invalid todo input"
}

// Validate trims surrounding whitespace and checks the field limits. It
// returns the normalized input together with a *ValidationError when the
// submission is not acceptable.
func Validate(in Input) (Input, error) {
	in.Title = strings.TrimSpace(in.Title)
	in.Notes = strings.TrimSpace(in.Notes)

	fields := make(map[string]string)
	switch {
	case in.Title == "":
		fields["title"] = "Title is required."
	case utf8.RuneCountInString(in.Title) > MaxTitleLength:
		fields["title"] = fmt.Sprintf("Title must be %d characters or fewer.", MaxTitleLength)
	}
	if utf8.RuneCountInString(in.Notes) > MaxNotesLength {
		fields["notes"] = fmt.Sprintf("Notes must be %d characters or fewer.", MaxNotesLength)
	}

	if len(fields) > 0 {
		return in, &ValidationError{Fields: fields}
	}
	return in, nil
}
