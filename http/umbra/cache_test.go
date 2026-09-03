package umbra

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCacheKeyStableAndPathScoped(t *testing.T) {
	a1, err := cacheKey("/repo/a/forgejo.guardfile.kdl")
	if err != nil {
		t.Fatalf("cacheKey: %v", err)
	}
	a2, err := cacheKey("/repo/a/forgejo.guardfile.kdl")
	if err != nil {
		t.Fatalf("cacheKey: %v", err)
	}
	b, err := cacheKey("/repo/b/forgejo.guardfile.kdl")
	if err != nil {
		t.Fatalf("cacheKey: %v", err)
	}
	if a1 != a2 {
		t.Errorf("cacheKey not stable: %q != %q", a1, a2)
	}
	if a1 == b {
		t.Errorf("distinct guardfile paths collided on key %q", a1)
	}
	if len(a1) != 16 {
		t.Errorf("cacheKey len = %d, want 16", len(a1))
	}
}

func TestCacheKeyForGroupUsesRelativeMemberIdentity(t *testing.T) {
	a := &group{Binary: "ward", Members: []member{{Path: "cloud/read.kdl"}, {Path: "forge/write.kdl"}}}
	b := &group{Binary: "ward", Members: []member{{Path: "cloud/read.kdl", SourcePath: "/elsewhere/cloud/read.kdl"}, {Path: "forge/write.kdl", SourcePath: "/elsewhere/forge/write.kdl"}}}
	ak := cacheKeyForGroup(a)
	bk := cacheKeyForGroup(b)
	if ak != bk {
		t.Errorf("re-rooted groups have cache keys %q and %q", ak, bk)
	}
}

func TestStaleDetectsEveryInput(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "bin", "ward-kdl")
	if err := os.MkdirAll(filepath.Dir(bin), 0o750); err != nil {
		t.Fatal(err)
	}
	base := stamp{
		GuardfileHash:    "gf",
		SpecLockHash:     "spec",
		DepLockHash:      "dep",
		GeneratorVersion: "v1",
	}
	// A released pin: immutable, so the stamp alone decides staleness.
	releasedLock := &DepLock{CLIGuard: "v0.194.0"}

	// No binary yet -> stale.
	if !stale(dir, bin, releasedLock, base) {
		t.Error("expected stale when binary is missing")
	}
	if err := os.WriteFile(bin, []byte("#!/bin/true"), 0o600); err != nil {
		t.Fatal(err)
	}

	// Binary present but no stamp -> stale.
	if !stale(dir, bin, releasedLock, base) {
		t.Error("expected stale when stamp is missing")
	}
	if err := writeStamp(dir, base); err != nil {
		t.Fatal(err)
	}

	// Matching stamp -> fresh.
	if stale(dir, bin, releasedLock, base) {
		t.Error("expected fresh when all inputs match")
	}

	// Each differing field independently triggers a rebuild.
	for name, mut := range map[string]func(stamp) stamp{
		"guardfile": func(s stamp) stamp { s.GuardfileHash = "x"; return s },
		"speclock":  func(s stamp) stamp { s.SpecLockHash = "x"; return s },
		"deplock":   func(s stamp) stamp { s.DepLockHash = "x"; return s },
		"generator": func(s stamp) stamp { s.GeneratorVersion = "x"; return s },
		"ldversion": func(s stamp) stamp { s.LDVersion = "x"; return s },
	} {
		if !stale(dir, bin, releasedLock, mut(base)) {
			t.Errorf("expected stale when %s hash differs", name)
		}
	}

	// umbra#1046: a replace-target checkout's source is in no staleness input,
	// so an identical stamp must not be read as fresh.
	if !stale(dir, bin, &DepLock{CLIGuard: "replace=/path/to/umbra"}, base) {
		t.Error("expected stale under a local-replace dep lock, even with every hash matching")
	}
	if stale(dir, bin, &DepLock{CLIGuard: "v0.194.0"}, base) {
		t.Error("a released pin is immutable and must stay cacheable")
	}
}

func TestStampRoundTrip(t *testing.T) {
	dir := t.TempDir()
	want := stamp{GuardfileHash: "a", SpecLockHash: "b", DepLockHash: "c", GeneratorVersion: "v", BuiltAt: "now"}
	if err := writeStamp(dir, want); err != nil {
		t.Fatalf("writeStamp: %v", err)
	}
	got, ok := readStamp(dir)
	if !ok {
		t.Fatal("readStamp reported missing after write")
	}
	if *got != want {
		t.Errorf("stamp round-trip = %+v, want %+v", *got, want)
	}
}
