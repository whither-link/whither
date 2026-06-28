package resolve

import (
	"errors"
	"strings"
	"testing"
)

func TestFullyURLDecode(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"Anna%27s_Archive", "Anna's_Archive"},
		{"anna%27s%20archive", "anna's archive"},
		// double-encoded: %2527 → %27 → '
		{"Anna%2527s_Archive", "Anna's_Archive"},
		// non-ASCII percent-encoded
		{"%C3%A9l%C3%A8ve", "élève"},
		// already plain
		{"Anna's_Archive", "Anna's_Archive"},
		// invalid encoding stops at last valid decode
		{"foo%ZZbar", "foo%ZZbar"},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			if got := fullyURLDecode(tc.in); got != tc.want {
				t.Errorf("fullyURLDecode(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestValidateInput(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		if err := validateInput("Anna's Archive"); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
	t.Run("empty", func(t *testing.T) {
		err := validateInput("")
		if !errors.Is(err, ErrBadInput) {
			t.Errorf("got %v, want ErrBadInput", err)
		}
	})
	t.Run("whitespace only", func(t *testing.T) {
		err := validateInput("   ")
		if !errors.Is(err, ErrBadInput) {
			t.Errorf("got %v, want ErrBadInput", err)
		}
	})
	t.Run("too long", func(t *testing.T) {
		err := validateInput(strings.Repeat("a", maxTitleBytes+1))
		if !errors.Is(err, ErrBadInput) {
			t.Errorf("got %v, want ErrBadInput", err)
		}
	})
	t.Run("null byte", func(t *testing.T) {
		err := validateInput("foo\x00bar")
		if !errors.Is(err, ErrBadInput) {
			t.Errorf("got %v, want ErrBadInput", err)
		}
	})
}

func TestNormalizeForKey(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"Anna's_Archive", "anna's_archive"},
		{"anna's archive", "anna's_archive"},
		{"Anna's Archive", "anna's_archive"},
		{"UPPER", "upper"},
		{"already_lower", "already_lower"},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			if got := normalizeForKey(tc.in); got != tc.want {
				t.Errorf("normalizeForKey(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
