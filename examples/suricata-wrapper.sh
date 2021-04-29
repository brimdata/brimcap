#!/bin/bash
suricata -r /dev/stdin
cat eve.json | jq -c . > deduped-eve.json
