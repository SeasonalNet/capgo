# Integration examples

`capgo` is implemented in Go. Go applications can import the library directly; applications written in other languages should invoke the `capgo` command and consume its JSON output.

## Build the command

From the repository root:

```sh
mkdir -p ./bin
go build -o ./bin/capgo ./cmd/capgo
export CAPGO_BIN="$PWD/bin/capgo"
```

In `validate` mode, the command accepts one XML path, or reads one CAP document from standard input when no path is supplied. It writes one JSON validation report to standard output. The JSON envelope has this stable shape:

```json
{
  "schema": "git.seasonalnet.org/SeasonalNet/capgo/validation/v1",
  "profile": "nws",
  "valid": true,
  "alert": { "identifier": "...", "sender": "..." },
  "actions": [
    { "language": "en-US", "event": "...", "areas": [] }
  ],
  "issues": []
}
```

`valid` is false when error-level findings exist. Validation mode still produces its report when profile validation fails and exits with status 1; diagnostics are also written to standard error. Malformed or unsafe XML also exits 1. Use `-compact` when a single-line document is preferred.

Supported profiles are `cap`, `capcp`, `ipaws`, and `nws`:

```sh
"$CAPGO_BIN" -profile nws -compact < alert.xml
```

Raw decode mode emits the complete typed message only when the selected
profile passes validation:

```sh
"$CAPGO_BIN" -mode decode -profile nws < alert.xml
```

The native Go API is required when an application needs channel-specific IPAWS validation or history-aware reference checks. The CLI examples below cover profile parsing and structured output.

## Language examples

- [Go native API](go/README.md)
- [Python](python/README.md)
- [Node.js](node/README.md)
- [Java](java/README.md)
- [C++](cpp/README.md)
- [Rust](rust/README.md)
- [C# / .NET](csharp/README.md)
- [POSIX shell](shell/README.md)
