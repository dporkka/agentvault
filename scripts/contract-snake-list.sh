#!/usr/bin/env bash
# contract-snake-list.sh — emit unique snake_case JSON field names extracted
# from Go struct tags. Only fields that are NOT already snake_case are emitted.
# Usage: contract-snake-list.sh <file.go>
if [[ -z "$1" ]]; then
  echo "usage: contract-snake-list.sh <file.go>" >&2
  exit 1
fi
awk '
{
  line = $0
  while (match(line, /json:"[^"]+"/)) {
    tag = substr(line, RSTART, RLENGTH)
    sub(/^json:"/, "", tag)
    sub(/".*$/, "", tag)
    split(tag, parts, ",")
    s = parts[1]
    # only convert if there are uppercase letters
    if (s ~ /[A-Z]/) {
      out = ""
      for (i = 1; i <= length(s); i++) {
        c = substr(s, i, 1)
        if (c ~ /[A-Z]/) {
          if (out != "") out = out "_"
          out = out tolower(c)
        } else {
          out = out c
        }
      }
      print out
    }
    line = substr(line, RSTART + RLENGTH)
  }
}
' "$1" | sort -u