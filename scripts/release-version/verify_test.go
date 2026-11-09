package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Build a PR commit only inside a throwaway repository; verifier itself never checks it out.
func releasePR(t *testing.T) (options, result) {
	t.Helper()
	o, _ := repo(t, "1.1.0")
	commit(t, &o, "fix: valid user change")
	r, err := calculate(o)
	if err != nil {
		t.Fatal(err)
	}
	files, err := releaseFiles(o, r)
	if err != nil {
		t.Fatal(err)
	}
	run(t, o.repo, "switch", "-c", "release-test")
	for name, data := range files {
		file := filepath.Join(o.repo, filepath.FromSlash(name))
		if err = os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(file, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	run(t, o.repo, "add", "configs/.release-please-manifest.json", "CHANGELOG.md")
	run(t, o.repo, "commit", "-m", "chore: prepare release")
	pr := run(t, o.repo, "rev-parse", "HEAD")
	run(t, o.repo, "switch", "main")
	return options{repo: o.repo, ref: o.ref, head: o.head, verifyPR: true, prHead: pr, project: "paas-ctl", releaseAnchor: "1.1.0@" + o.baseSHA}, r
}

func TestVerifyReleasePR(t *testing.T) {
	t.Run("valid exact object, no checkout", func(t *testing.T) {
		o, want := releasePR(t)
		before := run(t, o.repo, "rev-parse", "HEAD")
		got, err := verifyReleasePR(o)
		if err != nil || got.Status != "verified" || got.Version != want.Version || got.PRHead != o.prHead {
			t.Fatal(got, err)
		}
		if run(t, o.repo, "rev-parse", "HEAD") != before {
			t.Fatal("verifier changed checkout")
		}
		cmd := exec.Command("go", "run", ".", "--verify-pr", "--repo", o.repo, "--head", o.head, "--pr-head", o.prHead, "--project", o.project, "--release-anchor", o.releaseAnchor)
		data, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("CLI: %s %v", data, err)
		}
		if json.Unmarshal(data, &got) != nil || got.Status != "verified" {
			t.Fatalf("CLI JSON: %s", data)
		}
	})
	t.Run("bare Git repository", func(t *testing.T) {
		o, _ := releasePR(t)
		dir := outside(t)
		bare := filepath.Join(dir, "target.git")
		run(t, dir, "clone", "--bare", o.repo, bare)
		o.repo = bare
		got, err := verifyReleasePR(o)
		if err != nil || got.Status != "verified" {
			t.Fatal(got, err)
		}
	})
	t.Run("untrusted option", func(t *testing.T) {
		o, _ := releasePR(t)
		o.baseTag = "v1.1.0"
		if _, err := verifyReleasePR(o); err == nil {
			t.Fatal("PR supplied boundary")
		}
	})
	t.Run("anchor required", func(t *testing.T) {
		o, _ := releasePR(t)
		o.releaseAnchor = ""
		if _, err := verifyReleasePR(o); err == nil {
			t.Fatal("unanchored tag accepted")
		}
	})
	t.Run("common published anchor cannot be replaced", func(t *testing.T) {
		o, _ := releasePR(t)
		o.project = "paas-common"
		if _, err := verifyReleasePR(o); err == nil {
			t.Fatal("forged common 1.1.0 anchor accepted")
		}
	})
	t.Run("tag moved across feat", func(t *testing.T) {
		o, _ := repo(t, "1.1.0")
		feature := commit(t, &o, "feat: important feature")
		commit(t, &o, "fix: small fix")
		r, err := calculate(o)
		if err != nil {
			t.Fatal(err)
		}
		files, err := releaseFiles(o, r)
		if err != nil {
			t.Fatal(err)
		}
		run(t, o.repo, "switch", "-c", "release-test")
		for name, data := range files {
			file := filepath.Join(o.repo, filepath.FromSlash(name))
			if err = os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
				t.Fatal(err)
			}
			if err = os.WriteFile(file, data, 0o644); err != nil {
				t.Fatal(err)
			}
		}
		run(t, o.repo, "add", ".")
		run(t, o.repo, "commit", "-m", "chore: release")
		pr := run(t, o.repo, "rev-parse", "HEAD")
		run(t, o.repo, "switch", "main")
		run(t, o.repo, "tag", "-f", "v1.1.0", feature)
		verify := options{repo: o.repo, ref: o.ref, head: o.head, verifyPR: true, prHead: pr, project: "paas-ctl", releaseAnchor: "1.1.0@" + o.baseSHA}
		if _, err = verifyReleasePR(verify); err == nil || !strings.Contains(err.Error(), "release tag does not match confirmed boundary") {
			t.Fatalf("moved tag was not rejected against independent anchor: %v", err)
		}
	})
	t.Run("wrong parent", func(t *testing.T) {
		o, _ := releasePR(t)
		run(t, o.repo, "switch", "release-test")
		run(t, o.repo, "commit", "--allow-empty", "-m", "another commit")
		o.prHead = run(t, o.repo, "rev-parse", "HEAD")
		run(t, o.repo, "switch", "main")
		if _, err := verifyReleasePR(o); err == nil {
			t.Fatal("multiple commits accepted")
		}
	})
	t.Run("hidden merge history", func(t *testing.T) {
		o, _ := releasePR(t)
		tree := run(t, o.repo, "rev-parse", o.prHead+"^{tree}")
		run(t, o.repo, "switch", "--orphan", "hidden")
		run(t, o.repo, "commit", "--allow-empty", "-m", "foreign history")
		foreign := run(t, o.repo, "rev-parse", "HEAD")
		o.prHead = run(t, o.repo, "commit-tree", tree, "-p", o.head, "-p", foreign, "-m", "chore: release")
		run(t, o.repo, "switch", "main")
		if _, err := verifyReleasePR(o); err == nil {
			t.Fatal("merge introducing hidden history accepted")
		}
	})
	t.Run("stale base", func(t *testing.T) {
		o, _ := releasePR(t)
		run(t, o.repo, "commit", "--allow-empty", "-m", "fix: later")
		if _, err := verifyReleasePR(o); err == nil {
			t.Fatal("stale base accepted")
		}
	})
	for _, tc := range []struct {
		name   string
		mutate func(*testing.T, options)
	}{
		{"extra file", func(t *testing.T, o options) {
			if err := os.WriteFile(filepath.Join(o.repo, "extra"), []byte("unexpected"), 0o644); err != nil {
				t.Fatal(err)
			}
			run(t, o.repo, "add", "extra")
		}},
		{"nested extra file", func(t *testing.T, o options) {
			dir := filepath.Join(o.repo, ".github", "workflows")
			if err := os.MkdirAll(dir, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "unsafe.yml"), []byte("unexpected"), 0o644); err != nil {
				t.Fatal(err)
			}
			run(t, o.repo, "add", ".github/workflows/unsafe.yml")
		}},
		{"tampered content", func(t *testing.T, o options) {
			if err := os.WriteFile(filepath.Join(o.repo, "CHANGELOG.md"), []byte("forged"), 0o644); err != nil {
				t.Fatal(err)
			}
			run(t, o.repo, "add", "CHANGELOG.md")
		}},
		{"oversized blob", func(t *testing.T, o options) {
			if err := os.WriteFile(filepath.Join(o.repo, "CHANGELOG.md"), []byte(strings.Repeat("x", 2<<20)), 0o644); err != nil {
				t.Fatal(err)
			}
			run(t, o.repo, "add", "CHANGELOG.md")
		}},
		{"executable", func(t *testing.T, o options) {
			if err := os.Chmod(filepath.Join(o.repo, "CHANGELOG.md"), 0o755); err != nil {
				t.Fatal(err)
			}
			run(t, o.repo, "update-index", "--chmod=+x", "CHANGELOG.md")
		}},
		{"symlink", func(t *testing.T, o options) {
			file := filepath.Join(o.repo, "CHANGELOG.md")
			if err := os.Remove(file); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink("configs/.release-please-manifest.json", file); err != nil {
				t.Fatal(err)
			}
			run(t, o.repo, "add", "CHANGELOG.md")
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			o, _ := releasePR(t)
			run(t, o.repo, "switch", "release-test")
			tc.mutate(t, o)
			run(t, o.repo, "commit", "--amend", "--no-edit")
			o.prHead = run(t, o.repo, "rev-parse", "HEAD")
			run(t, o.repo, "switch", "main")
			if _, err := verifyReleasePR(o); err == nil {
				t.Fatal("untrusted PR accepted")
			}
		})
	}
	t.Run("override in PR commit", func(t *testing.T) {
		o, _ := releasePR(t)
		run(t, o.repo, "switch", "release-test")
		run(t, o.repo, "commit", "--amend", "-m", "chore: release\n\nRelease-As: 2.0.0")
		o.prHead = run(t, o.repo, "rev-parse", "HEAD")
		run(t, o.repo, "switch", "main")
		if _, err := verifyReleasePR(o); err == nil {
			t.Fatal("override accepted")
		}
	})
	t.Run("no release", func(t *testing.T) {
		o, _ := repo(t, "1.1.0")
		run(t, o.repo, "switch", "-c", "release-test")
		if err := os.WriteFile(filepath.Join(o.repo, "CHANGELOG.md"), []byte("forged"), 0o644); err != nil {
			t.Fatal(err)
		}
		run(t, o.repo, "add", "CHANGELOG.md")
		run(t, o.repo, "commit", "-m", "chore: fake release")
		pr := run(t, o.repo, "rev-parse", "HEAD")
		run(t, o.repo, "switch", "main")
		o.verifyPR = true
		o.prHead = pr
		o.project = "paas-ctl"
		o.releaseAnchor = "1.1.0@" + o.baseSHA
		o.baseTag = ""
		o.baseSHA = ""
		o.manifest = ""
		if _, err := verifyReleasePR(o); err == nil || !strings.Contains(err.Error(), "no user changes") {
			t.Fatalf("no_release not checked: %v", err)
		}
	})
}

func TestVerifyReleasePRBootstrapUsesTrustedRoots(t *testing.T) {
	dir := t.TempDir()
	run(t, dir, "init", "-b", "main")
	run(t, dir, "commit", "--allow-empty", "-m", "initial")
	root := run(t, dir, "rev-parse", "HEAD")
	trusted := options{repo: dir, ref: "refs/heads/main", head: root, bootstrapRoots: stringsFlag{root}}
	r, err := calculate(trusted)
	if err != nil {
		t.Fatal(err)
	}
	files, err := releaseFiles(trusted, r)
	if err != nil {
		t.Fatal(err)
	}
	run(t, dir, "switch", "-c", "release-test")
	for name, data := range files {
		file := filepath.Join(dir, filepath.FromSlash(name))
		if err = os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(file, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	run(t, dir, "add", ".")
	run(t, dir, "commit", "-m", "chore: release")
	pr := run(t, dir, "rev-parse", "HEAD")
	run(t, dir, "switch", "main")
	o := options{repo: dir, ref: "refs/heads/main", head: root, verifyPR: true, prHead: pr, project: "paas-ctl"}
	got, err := verifyReleasePRProfile(o, []string{root}, true)
	if err != nil || got.Version != "1.0.0" {
		t.Fatal(got, err)
	}
	if _, err = verifyReleasePRProfile(o, []string{strings.Repeat("a", 40)}, true); err == nil {
		t.Fatal("unconfirmed roots accepted")
	}
	if _, err = verifyReleasePRProfile(o, nil, false); err == nil {
		t.Fatal("bootstrap without approval accepted")
	}
	// A manifest in the PR cannot change the trusted bootstrap decision.
	o.bootstrapRoots = stringsFlag{root}
	if _, err = verifyReleasePRProfile(o, []string{root}, true); err == nil {
		t.Fatal("caller-supplied bootstrap accepted")
	}
}
