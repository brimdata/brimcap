#!/bin/bash
TMPFILE=$(mktemp)
cat - > "$TMPFILE"
nfpcapd -r "$TMPFILE" -l .
rm "$TMPFILE"
for file in nfcapd.*
do
  nfdump -r $file -o csv | head -n -3 | zq -i csv -f ndjson - > ${file}.ndjson
done
