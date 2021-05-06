#!/bin/bash -e
suricata -r /dev/stdin
exec jq -c . eve.json > deduped-eve.json
