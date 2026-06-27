#!/usr/bin/env bash
# Fetch live Wikimedia API responses into testdata/wiki
#
# wikidata-p856-novalue.json fixture is intentionally not fetched

set -euo pipefail

REPO="$(cd "$(dirname "$0")/.." && pwd)"
OUT="$REPO/testdata/wiki"
UA="Whither/dev"

fetch() {
  local name="$1"
  local url="$2"
  printf '  %-48s' "$name"
  curl -sSfL --max-time 15 -H "User-Agent: $UA" "$url" -o "$OUT/$name"
  echo "ok"
}

fetch mediawiki-normalize-found.json \
  "https://en.wikipedia.org/w/api.php?action=query&format=json&formatversion=2&redirects=1&prop=pageprops&titles=anna%27s%20archive"

fetch mediawiki-normalize-missing.json \
  "https://en.wikipedia.org/w/api.php?action=query&format=json&formatversion=2&redirects=1&prop=pageprops&titles=ThisPageDoesNotExistXYZZY99999"

fetch mediawiki-normalize-disambig.json \
  "https://en.wikipedia.org/w/api.php?action=query&format=json&formatversion=2&redirects=1&prop=pageprops&titles=Mercury"

fetch mediawiki-normalize-redirect.json \
  "https://en.wikipedia.org/w/api.php?action=query&format=json&formatversion=2&redirects=1&prop=pageprops&titles=The%20Pirate%20Bay"

fetch mediawiki-opensearch-hit.json \
  "https://en.wikipedia.org/w/api.php?action=opensearch&format=json&namespace=0&limit=3&search=Anna%27s%20Archive"

fetch mediawiki-opensearch-empty.json \
  "https://en.wikipedia.org/w/api.php?action=opensearch&format=json&namespace=0&limit=5&search=xyzzynonexistent12345"

fetch wikidata-p856-single.json \
  "https://www.wikidata.org/w/api.php?action=wbgetclaims&format=json&entity=Q364&property=P856"

fetch wikidata-p856-multi.json \
  "https://www.wikidata.org/w/api.php?action=wbgetclaims&format=json&entity=Q461&property=P856"

fetch wikidata-p856-empty.json \
  "https://www.wikidata.org/w/api.php?action=wbgetclaims&format=json&entity=Q1&property=P856"
