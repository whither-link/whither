package resolve

import (
	"testing"

	"github.com/whither-link/whither/internal/wiki"
)

func site(url, rank, langQual string) wiki.OfficialSite {
	return wiki.OfficialSite{URL: url, Rank: rank, LangQual: langQual}
}

func TestSelectP856(t *testing.T) {
	cases := []struct {
		name  string
		sites []wiki.OfficialSite
		lang  string
		want  string
	}{
		{
			name:  "single normal",
			sites: []wiki.OfficialSite{site("https://example.com", "normal", "")},
			lang:  "en",
			want:  "https://example.com",
		},
		{
			name: "preferred beats normal",
			sites: []wiki.OfficialSite{
				site("https://normal.example.com", "normal", ""),
				site("https://preferred.example.com", "preferred", ""),
			},
			lang: "en",
			want: "https://preferred.example.com",
		},
		{
			name: "all deprecated returns empty",
			sites: []wiki.OfficialSite{
				site("https://old.example.com", "deprecated", ""),
			},
			lang: "en",
			want: "",
		},
		{
			name: "deprecated filtered; normal used",
			sites: []wiki.OfficialSite{
				site("https://old.example.com", "deprecated", ""),
				site("https://current.example.com", "normal", ""),
			},
			lang: "en",
			want: "https://current.example.com",
		},
		{
			name: "lang qualifier match preferred over first",
			sites: []wiki.OfficialSite{
				site("https://first.example.com", "normal", ""),
				site("https://english.example.com", "normal", "Q1860"),
			},
			lang: "en",
			want: "https://english.example.com",
		},
		{
			name: "no lang match falls back to first candidate",
			sites: []wiki.OfficialSite{
				site("https://first.example.com", "normal", "Q188"),  // German
				site("https://second.example.com", "normal", "Q150"), // French
			},
			lang: "en",
			want: "https://first.example.com",
		},
		{
			name: "preferred rank with lang match",
			sites: []wiki.OfficialSite{
				site("https://preferred-de.example.com", "preferred", "Q188"),
				site("https://preferred-en.example.com", "preferred", "Q1860"),
				site("https://normal.example.com", "normal", ""),
			},
			lang: "en",
			want: "https://preferred-en.example.com",
		},
		{
			name:  "empty sites",
			sites: nil,
			lang:  "en",
			want:  "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := selectP856(tc.sites, tc.lang)
			if got != tc.want {
				t.Errorf("selectP856() = %q, want %q", got, tc.want)
			}
		})
	}
}
