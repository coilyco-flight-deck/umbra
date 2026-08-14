# audit example

The minimum useful umbra program. Wires `audit.NewWriter` with `verb.Wrap` so every invocation of the `hello` subcommand lands one JSONL row in `$TMPDIR/umbra-demo.jsonl`.

```
$ go run ./examples/audit hello world
hello, world
$ cat $TMPDIR/umbra-demo.jsonl | tail -1 | jq .verb
"hello"
```
