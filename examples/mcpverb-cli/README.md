# mcpverb-cli: a guarded CLI over a real MCP server

The other MCP example, [`mcpverb/`](../mcpverb/main.go), imports the engine and starts its own server in-process, so it runs with nothing installed. This one is the product path instead: **KDL policy plus a committed lock, no Go at all**, against a server this repository does not control.

The upstream is [`@modelcontextprotocol/server-everything`](https://www.npmjs.com/package/@modelcontextprotocol/server-everything), the protocol's own reference implementation, started over stdio by `npx`. It exercises the stdio transport, which the in-process example cannot.

## Run it

```sh
cd examples/mcpverb-cli
umbra lock          # the online step: start the server, read tools/list, freeze it
umbra build --out ./mcpdemo
./mcpdemo ops everything --help
```

`lock` is separate because `specverb.lock` pins the umbra module version, which is yours rather than this repository's. The **tool** lock is committed, so the surface below is readable without running anything.

## What it shows

```
$ ./mcpdemo ops everything --help
COMMANDS:
   echo                    call echo
   get-sum                 call get-sum
   get-structured-content  call get-structured-content
```

Three leaves from fourteen upstream tools.

```sh
$ ./mcpdemo ops everything echo --message "hello from a guarded CLI"
'Echo: hello from a guarded CLI'

$ ./mcpdemo ops everything get-sum -a 20 -b 22
The sum of 20 and 22 is 42.
```

`-a` and `-b` are typed floats because the server's own JSON Schema says so. The guardfile never mentions them, and `umbra skew` fails if that schema moves.

```sh
$ ./mcpdemo ops everything get-env          # never call get-env
mcpdemo: unknown verb "get-env" ...   (exit 5)

$ ./mcpdemo ops everything get-tiny-image   # named by no sentence at all
mcpdemo: unknown verb "get-tiny-image" ...  (exit 5)
```

**Both are absent, and identically so.** `get-env` is denied on purpose, since a tool that reads the process environment is the obvious thing to close. The other ten are simply not granted. Deny is absence, so a reader of `--help` cannot tell which tools exist upstream, and an agent spends no context on a verb it may not call. See [descriptors](../../docs/specverb-descriptors.md).

## Things worth trying

- `./mcpdemo ops everything get-sum -a 1 -b 2 --dry-run` prints the resolved tool and arguments without firing.
- `./mcpdemo ops everything get-structured-content --location Chicago --query 'temperature'` projects the result with JMESPath. Its `--help` reads `Choose city (one of: New York, Chicago, Los Angeles)`, because an enum is the one constraint a caller cannot infer from the type, so it is carried into the flag.
- `umbra skew` diffs the committed tool lock against the live server, exit 3 on drift, including a moved `_meta`.
- Add `can call get-env` beside the `never` and `umbra lock` fails closed rather than resolving the contradiction for you.

## See also

- [docs/mcpverb.md](../../docs/mcpverb.md) - the dialect, its guards, and the lock.
- [docs/umbra-cli.md](../../docs/umbra-cli.md) - the driver and its five verbs.
