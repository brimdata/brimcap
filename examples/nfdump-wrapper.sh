#!/bin/bash
export LD_LIBRARY_PATH="/usr/local/lib"
TMPFILE=$(mktemp)
cat - > "$TMPFILE"
/usr/local/bin/nfpcapd -r "$TMPFILE" -l .
rm "$TMPFILE"
for file in nfcapd.*
do
  /usr/local/bin/nfdump -r $file -o csv | head -n -3 > ${file}.csv
done
