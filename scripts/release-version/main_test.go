package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func run(t *testing.T, dir string, args ...string) string {
	t.Helper()
	c := exec.Command("git", args...)
	c.Dir = dir
	c.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=test@example.test", "GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=test@example.test")
	out, err := c.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v %s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func repo(t *testing.T, version string) (options, string) {
	t.Helper()
	dir := t.TempDir()
	run(t, dir, "init", "-b", "main")
	if err := os.MkdirAll(filepath.Join(dir, "configs"), 0o755); err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(map[string]string{".": version})
	if err := os.WriteFile(filepath.Join(dir, "configs", ".release-please-manifest.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	run(t, dir, "add", ".")
	run(t, dir, "commit", "-m", "initial")
	base := run(t, dir, "rev-parse", "HEAD")
	run(t, dir, "tag", "v"+version)
	return options{repo: dir, ref: "refs/heads/main", baseTag: "v" + version, baseSHA: base, manifest: "configs/.release-please-manifest.json", head: base}, base
}

func commit(t *testing.T, o *options, message string) string {
	t.Helper()
	run(t, o.repo, "commit", "--allow-empty", "-m", message)
	o.head = run(t, o.repo, "rev-parse", "HEAD")
	return o.head
}

func TestVersionVectors(t *testing.T) {
	for _, tc := range []struct{ base, msg, want string }{{"1.1.0", "fix: x", "1.1.1"}, {"1.1.0", "fix(api): x", "1.1.1"}, {"1.1.0", "feat: x", "1.2.0"}, {"1.1.0", "chore: x", "1.2.0"}, {"1.1.0", "fix!: x", "1.2.0"}, {"1.1.0", "fix: x\n\nBREAKING CHANGE: x", "1.2.0"}, {"1.1.0", "unknown message", "1.2.0"}, {"0.3.2", "fix!: x", "0.4.0"}} {
		t.Run(tc.base+" "+tc.msg, func(t *testing.T) {
			o, _ := repo(t, tc.base)
			commit(t, &o, tc.msg)
			got, err := calculate(o)
			if err != nil || got.Version != tc.want {
				t.Fatalf("%+v %v", got, err)
			}
		})
	}
}

func TestMixedAndVerifiedReleaseCommit(t *testing.T) {
	o, _ := repo(t, "1.1.0")
	id := commit(t, &o, "chore: release 1.1.0")
	o.releases = []string{id}
	got, err := calculate(o)
	if err != nil || got.Status != "no_release" {
		t.Fatal(got, err)
	}
	o.releases = nil
	got, err = calculate(o)
	if err != nil || got.Version != "1.2.0" {
		t.Fatal("release-like unverified commit", got, err)
	}
	o.releases = []string{id}
	commit(t, &o, "fix: x")
	commit(t, &o, "chore: x")
	got, err = calculate(o)
	if err != nil || got.Version != "1.2.0" || got.UserCommits != 2 {
		t.Fatal(got, err)
	}
	again, err := calculate(o)
	if err != nil || got != again {
		t.Fatal("nondeterministic", again, err)
	}
}

func TestCLIProducesJSON(t *testing.T) {
	o, _ := repo(t, "1.1.0")
	commit(t, &o, "fix: x")
	cmd := exec.Command("go", "run", ".", "--repo", o.repo, "--head", o.head, "--base-tag", o.baseTag, "--base-sha", o.baseSHA, "--manifest", o.manifest)
	data, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	var got result
	if err = json.Unmarshal(data, &got); err != nil || got.Status != "release" || got.Version != "1.1.1" || got.Head != o.head {
		t.Fatalf("CLI result %s: %v", data, err)
	}
}

func TestRejectsConflictsAndOverrides(t *testing.T) {
	for _, message := range []string{"fix: x\n\nRelease-As: 2.0.0", "fix: x\n\nBEGIN_COMMIT_OVERRIDE\nfeat: hidden\nEND_COMMIT_OVERRIDE"} {
		t.Run(message, func(t *testing.T) {
			o, _ := repo(t, "1.1.0")
			commit(t, &o, message)
			if _, err := calculate(o); err == nil {
				t.Fatal("override accepted")
			}
		})
	}
	t.Run("manifest", func(t *testing.T) {
		o, _ := repo(t, "1.1.0")
		if err := os.WriteFile(filepath.Join(o.repo, o.manifest), []byte(`{".":"1.0.0"}`), 0o644); err != nil {
			t.Fatal(err)
		}
		run(t, o.repo, "add", o.manifest)
		commit(t, &o, "fix: x")
		if _, err := calculate(o); err == nil {
			t.Fatal("conflicting manifest accepted")
		}
	})
	t.Run("duplicate manifest", func(t *testing.T) {
		o, _ := repo(t, "1.1.0")
		if err := os.WriteFile(filepath.Join(o.repo, o.manifest), []byte(`{".":"2.0.0",".":"1.1.0"}`), 0o644); err != nil {
			t.Fatal(err)
		}
		run(t, o.repo, "add", o.manifest)
		commit(t, &o, "fix: x")
		if _, err := calculate(o); err == nil {
			t.Fatal("duplicate manifest accepted")
		}
	})
	t.Run("newer published tag", func(t *testing.T) {
		o, _ := repo(t, "1.1.0")
		commit(t, &o, "feat: x")
		run(t, o.repo, "tag", "v1.2.0")
		if _, err := calculate(o); err == nil {
			t.Fatal("newer reachable tag accepted")
		}
	})
	t.Run("candidate tag on unrelated branch", func(t *testing.T) {
		o, _ := repo(t, "1.1.0")
		run(t, o.repo, "switch", "--orphan", "other")
		run(t, o.repo, "commit", "--allow-empty", "-m", "other")
		run(t, o.repo, "tag", "v1.1.1")
		run(t, o.repo, "switch", "main")
		commit(t, &o, "fix: x")
		if _, err := calculate(o); err == nil {
			t.Fatal("existing candidate tag accepted")
		}
	})
	t.Run("numeric overflow", func(t *testing.T) {
		o, _ := repo(t, "1.4294967295.0")
		commit(t, &o, "feat: x")
		if _, err := calculate(o); err == nil {
			t.Fatal("overflow accepted")
		}
	})
	t.Run("v2", func(t *testing.T) {
		o, _ := repo(t, "2.0.0")
		commit(t, &o, "fix: x")
		if _, err := calculate(o); err == nil {
			t.Fatal("v2 accepted")
		}
	})
	t.Run("stale", func(t *testing.T) {
		o, _ := repo(t, "1.1.0")
		old := o.head
		commit(t, &o, "fix: x")
		o.head = old
		if _, err := calculate(o); err == nil {
			t.Fatal("stale target accepted")
		}
	})
	t.Run("non-main branch", func(t *testing.T) {
		o, _ := repo(t, "1.1.0")
		run(t, o.repo, "switch", "-c", "topic")
		commit(t, &o, "fix: x")
		o.ref = "refs/heads/topic"
		if _, err := calculate(o); err == nil {
			t.Fatal("non-main branch accepted")
		}
	})
	t.Run("wrong boundary", func(t *testing.T) {
		o, _ := repo(t, "1.1.0")
		id := commit(t, &o, "fix: x")
		o.baseSHA = id
		if _, err := calculate(o); err == nil {
			t.Fatal("wrong tag accepted")
		}
	})
}

func TestBootstrapAndIncompleteHistory(t *testing.T) {
	t.Run("bootstrap", func(t *testing.T) {
		dir := t.TempDir()
		run(t, dir, "init", "-b", "main")
		run(t, dir, "commit", "--allow-empty", "-m", "feat: initial")
		root := run(t, dir, "rev-parse", "HEAD")
		o := options{repo: dir, ref: "refs/heads/main", head: root, bootstrapRoots: stringsFlag{root}}
		got, err := calculate(o)
		if err != nil || got.Version != "1.0.0" {
			t.Fatal(got, err)
		}
		run(t, dir, "tag", "v0.1.0")
		if _, err = calculate(o); err == nil {
			t.Fatal("bootstrap with tag accepted")
		}
	})
	t.Run("bootstrap v2 tag", func(t *testing.T) {
		dir := t.TempDir()
		run(t, dir, "init", "-b", "main")
		run(t, dir, "commit", "--allow-empty", "-m", "feat: initial")
		root := run(t, dir, "rev-parse", "HEAD")
		run(t, dir, "tag", "v2.0.0")
		if _, err := calculate(options{repo: dir, ref: "refs/heads/main", head: root, bootstrapRoots: stringsFlag{root}}); err == nil {
			t.Fatal("bootstrap with v2 tag accepted")
		}
	})
	t.Run("bootstrap multiple roots", func(t *testing.T) {
		dir := t.TempDir()
		run(t, dir, "init", "-b", "main")
		run(t, dir, "commit", "--allow-empty", "-m", "initial")
		first := run(t, dir, "rev-parse", "HEAD")
		run(t, dir, "switch", "--orphan", "other")
		run(t, dir, "commit", "--allow-empty", "-m", "other")
		second := run(t, dir, "rev-parse", "HEAD")
		run(t, dir, "switch", "main")
		run(t, dir, "merge", "--allow-unrelated-histories", "--no-ff", "other", "-m", "merge other")
		head := run(t, dir, "rev-parse", "HEAD")
		o := options{repo: dir, ref: "refs/heads/main", head: head, bootstrapRoots: stringsFlag{first, second}}
		got, err := calculate(o)
		if err != nil || got.Version != "1.0.0" || got.UserCommits != 3 {
			t.Fatal(got, err)
		}
		o.bootstrapRoots = stringsFlag{first}
		if _, err = calculate(o); err == nil {
			t.Fatal("missing root accepted")
		}
	})
	t.Run("bootstrap rejects override", func(t *testing.T) {
		dir := t.TempDir()
		run(t, dir, "init", "-b", "main")
		run(t, dir, "commit", "--allow-empty", "-m", "fix: x\n\nRelease-As: 2.0.0")
		root := run(t, dir, "rev-parse", "HEAD")
		if _, err := calculate(options{repo: dir, ref: "refs/heads/main", head: root, bootstrapRoots: stringsFlag{root}}); err == nil {
			t.Fatal("bootstrap override accepted")
		}
	})
	t.Run("shallow", func(t *testing.T) {
		o, _ := repo(t, "1.1.0")
		commit(t, &o, "fix: x")
		dir := filepath.Join(t.TempDir(), "clone")
		run(t, t.TempDir(), "clone", "--depth=1", "file://"+o.repo, dir)
		o.repo = dir
		if _, err := calculate(o); err == nil {
			t.Fatal("shallow history accepted")
		}
	})
	t.Run("missing object", func(t *testing.T) {
		o, _ := repo(t, "1.1.0")
		commit(t, &o, "fix: x")
		blob := run(t, o.repo, "rev-parse", "HEAD:"+o.manifest)
		if err := os.Remove(filepath.Join(o.repo, ".git", "objects", blob[:2], blob[2:])); err != nil {
			t.Fatal(err)
		}
		if _, err := calculate(o); err == nil {
			t.Fatal("missing object accepted")
		}
	})
	t.Run("replacement ref", func(t *testing.T) {
		o, _ := repo(t, "1.1.0")
		id := commit(t, &o, "fix: x")
		run(t, o.repo, "replace", o.baseSHA, id)
		if _, err := calculate(o); err == nil {
			t.Fatal("replacement ref accepted")
		}
	})
}
