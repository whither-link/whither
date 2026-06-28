package resolve

// scrapeInfobox parses article HTML and returns the first official website URL
// found in an infobox "Website" row (F4).
//
// This is a stub. HTML parsing via golang.org/x/net/html is deferred;
// the function always returns "" so the resolver falls through to the article
// fallback. Set WHITHER_INFOBOX_ENABLED=true once the parser is implemented.
func scrapeInfobox(_ []byte) string {
	return ""
}
