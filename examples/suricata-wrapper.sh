#!/bin/bash
suricata -r /dev/stdin
exec jq -c . eve.json > deduped-eve.json
