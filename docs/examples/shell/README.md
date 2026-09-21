# POSIX shell

The simplest integration is a pipeline. This example uses `jq` to select
actionable fields.

```sh
set -eu

CAPGO_BIN="${CAPGO_BIN:-capgo}"
json_file="$(mktemp)"
trap 'rm -f "$json_file"' EXIT
status=0
"$CAPGO_BIN" -profile nws -compact < alert.xml >"$json_file" || status=$?

if [ ! -s "$json_file" ]; then
  exit "$status"
fi

json="$(cat "$json_file")"

printf '%s\n' "$json" | jq -r '.actions[] | [.event, .language] | @tsv'
if [ "$status" -ne 0 ]; then
  printf '%s\n' "$json" | jq -r '.issues[] | [.rule, .path, .message] | @tsv' >&2
  exit "$status"
fi
```

The command writes diagnostics to standard error, so callers that need the JSON document should capture standard output separately from standard error.
