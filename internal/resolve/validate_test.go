package resolve

import "testing"

func TestValidateTarget(t *testing.T) {
	cases := []struct {
		url  string
		want bool // true = valid
	}{
		{"https://example.com", true},
		{"http://example.com/path?q=1", true},
		{"https://example.com/wiki/Anna%27s_Archive", true},

		// scheme violations
		{"javascript:alert(1)", false},
		{"data:text/html,<h1>hi</h1>", false},
		{"ftp://example.com", false},
		{"", false},

		// not absolute
		{"//example.com/path", false},
		{"/relative/path", false},
		{"relative", false},

		// empty host
		{"https:///path", false},

		// control characters
		{"https://example.com\nX-Inject: foo", false},
		{"https://example.com\r\nX-Inject: foo", false},
		{"https://example.com\x00", false},
		{"https://example.com\tfoo", false},

		// leading/trailing whitespace
		{" https://example.com", false},
		{"https://example.com ", false},
	}

	for _, tc := range cases {
		t.Run(tc.url, func(t *testing.T) {
			err := validateTarget(tc.url)
			if tc.want && err != nil {
				t.Errorf("validateTarget(%q) = %v, want nil", tc.url, err)
			}
			if !tc.want && err == nil {
				t.Errorf("validateTarget(%q) = nil, want error", tc.url)
			}
		})
	}
}
