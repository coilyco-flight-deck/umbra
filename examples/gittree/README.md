# gittree example

`gittree.CheckClean` is the clean+synced gate that refuses repo-shaped verbs on a dirty tree (uncommitted changes, untracked files, detached HEAD, no upstream). umbra uses this so every audit row reconstructs from git history.

```
$ cd /path/to/clean/repo && go run /path/to/umbra/examples/gittree build
ok: tree is clean, pretend-build runs

$ touch /path/to/clean/repo/dirt && go run /path/to/umbra/examples/gittree build
refused: working tree is dirty
```
