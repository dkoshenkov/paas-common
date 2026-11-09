package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func outside(t *testing.T) string {
	t.Helper()
	p, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestReleaseFilesPreserveHistoryAndEscape(t *testing.T) {
	o, _ := repo(t, "1.1.0")
	old := []byte("\xef\xbb\xbf# My release notes\r\n\r\n## 1.1.0\r\nOld record\r\n")
	if err := os.WriteFile(filepath.Join(o.repo, "CHANGELOG.md"), old, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(o.repo, o.manifest), []byte(`{".":"1.1.0","other":"7.2.1"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	run(t, o.repo, "add", "CHANGELOG.md", o.manifest)
	run(t, o.repo, "commit", "--amend", "--no-edit")
	o.baseSHA = run(t, o.repo, "rev-parse", "HEAD")
	run(t, o.repo, "tag", "-f", o.baseTag)
	commit(t, &o, "fix(api): safe `x` <img> @team\u202e")
	// A dirty worktree is not the source of release files.
	if err := os.WriteFile(filepath.Join(o.repo, "CHANGELOG.md"), []byte("uncommitted"), 0o644); err != nil {
		t.Fatal(err)
	}
	o.outputDir = filepath.Join(outside(t), "result")
	r, err := calculate(o)
	if err != nil {
		t.Fatal(err)
	}
	if err = writeReleaseFiles(o, r); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(o.outputDir, "CHANGELOG.md"))
	if err != nil {
		t.Fatal(err)
	}
	insertion := bytes.Index(got, []byte("\n## 1.1.1\r\n"))
	if insertion < 0 || !bytes.HasPrefix(got, old[:len("\xef\xbb\xbf# My release notes\r\n")]) || !bytes.HasSuffix(got, old[len("\xef\xbb\xbf# My release notes\r\n"):]) {
		t.Fatalf("previous bytes not preserved: %q", got)
	}
	if !bytes.Contains(got, []byte("`` fix(api): safe `x` <img> @team[U+202E] ``")) {
		t.Fatalf("subject not safely fenced and readable: %s", got)
	}
	manifest, err := os.ReadFile(filepath.Join(o.outputDir, manifestPath))
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]string
	if json.Unmarshal(manifest, &m) != nil || m["."] != "1.1.1" || m["other"] != "7.2.1" {
		t.Fatalf("manifest %s", manifest)
	}
	if err = writeReleaseFiles(o, r); err != nil {
		t.Fatal("identical rerun:", err)
	}
	if err = os.WriteFile(filepath.Join(o.outputDir, "CHANGELOG.md"), []byte("modified"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err = writeReleaseFiles(o, r); err == nil {
		t.Fatal("conflicting output accepted")
	}
}

func TestReleaseOutputNoReleaseAndPaths(t *testing.T) {
	o, _ := repo(t, "1.1.0")
	base := outside(t)
	o.outputDir = filepath.Join(base, "absent")
	if err := checkNoReleaseOutput(o); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(o.outputDir); !os.IsNotExist(err) {
		t.Fatal("no_release created output", err)
	}
	if err := os.Mkdir(o.outputDir, 0o700); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(o.outputDir, "keep")
	if err := os.WriteFile(marker, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := checkNoReleaseOutput(o); err == nil {
		t.Fatal("existing output accepted")
	}
	if b, err := os.ReadFile(marker); err != nil || string(b) != "keep" {
		t.Fatal("existing output changed")
	}
	o.outputDir = filepath.Join(o.repo, "bad")
	if err := validateOutputPath(o); err == nil {
		t.Fatal("source output accepted")
	}
	link := filepath.Join(base, "link")
	if err := os.Symlink(base, link); err != nil {
		t.Fatal(err)
	}
	o.outputDir = filepath.Join(link, "out")
	if err := validateOutputPath(o); err == nil {
		t.Fatal("symlink parent accepted")
	}
}

func TestBareRepositoryCLIOutput(t *testing.T) {
	o, _ := repo(t, "1.1.0")
	commit(t, &o, "feat: new capability")
	base := outside(t)
	bare := filepath.Join(base, "source.git")
	run(t, base, "clone", "--bare", o.repo, bare)
	output := filepath.Join(base, "out")
	cmd := exec.Command("go", "run", ".", "--repo", bare, "--head", o.head, "--base-tag", o.baseTag, "--base-sha", o.baseSHA, "--manifest", o.manifest, "--output-dir", output)
	data, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("bare CLI: %s %v", data, err)
	}
	var got result
	if json.Unmarshal(data, &got) != nil || got.Version != "1.2.0" || got.OutputDir != output {
		t.Fatalf("CLI result: %s", data)
	}
	notes, err := os.ReadFile(filepath.Join(output, "CHANGELOG.md"))
	if err != nil || !bytes.Contains(notes, []byte("feat: new capability")) {
		t.Fatalf("missing notes: %s %v", notes, err)
	}
	cmd = exec.Command("go", "run", ".", "--repo", bare, "--head", o.head, "--base-tag", o.baseTag, "--base-sha", o.baseSHA, "--manifest", o.manifest, "--output-dir", output)
	if data, err = cmd.CombinedOutput(); err != nil {
		t.Fatalf("bare repeat: %s %v", data, err)
	}
}

func TestBootstrapChangelogIncludesAllRoots(t *testing.T) {
	dir := t.TempDir()
	run(t, dir, "init", "-b", "main")
	run(t, dir, "commit", "--allow-empty", "-m", "root one")
	first := run(t, dir, "rev-parse", "HEAD")
	run(t, dir, "switch", "--orphan", "other")
	run(t, dir, "commit", "--allow-empty", "-m", "root two")
	second := run(t, dir, "rev-parse", "HEAD")
	run(t, dir, "switch", "main")
	run(t, dir, "merge", "--allow-unrelated-histories", "--no-ff", "other", "-m", "merge other")
	head := run(t, dir, "rev-parse", "HEAD")
	o := options{repo: dir, ref: "refs/heads/main", head: head, bootstrapRoots: stringsFlag{first, second}, outputDir: filepath.Join(outside(t), "out")}
	r, err := calculate(o)
	if err != nil {
		t.Fatal(err)
	}
	if err = writeReleaseFiles(o, r); err != nil {
		t.Fatal(err)
	}
	notes, err := os.ReadFile(filepath.Join(o.outputDir, "CHANGELOG.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{"root one", "root two", "merge other", first, second} {
		if !strings.Contains(string(notes), value) {
			t.Fatalf("missing %q: %s", value, notes)
		}
	}
}

func TestChangelogWithoutH1PreservesBOMAndBody(t *testing.T) {
	o, _ := repo(t, "1.1.0")
	old := []byte("\xef\xbb\xbfExisting notes\r\nlegacy entry\r\n")
	if err := os.WriteFile(filepath.Join(o.repo, "CHANGELOG.md"), old, 0o644); err != nil {
		t.Fatal(err)
	}
	run(t, o.repo, "add", "CHANGELOG.md")
	run(t, o.repo, "commit", "--amend", "--no-edit")
	o.baseSHA = run(t, o.repo, "rev-parse", "HEAD")
	run(t, o.repo, "tag", "-f", o.baseTag)
	commit(t, &o, "fix: update")
	o.outputDir = filepath.Join(outside(t), "out")
	r, err := calculate(o)
	if err != nil {
		t.Fatal(err)
	}
	if err = writeReleaseFiles(o, r); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(o.outputDir, "CHANGELOG.md"))
	if err != nil || !bytes.HasPrefix(got, []byte("\xef\xbb\xbf")) || !bytes.HasSuffix(got, old[3:]) {
		t.Fatalf("old content altered: %q %v", got, err)
	}
}

func TestNoReleaseCLIExistingOutput(t *testing.T) {
	o, _ := repo(t, "1.1.0")
	dir := filepath.Join(outside(t), "out")
	args := []string{"run", ".", "--repo", o.repo, "--head", o.head, "--base-tag", o.baseTag, "--base-sha", o.baseSHA, "--manifest", o.manifest, "--output-dir", dir}
	cmd := exec.Command("go", args...)
	data, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("no release: %s %v", data, err)
	}
	var r result
	if json.Unmarshal(data, &r) != nil || r.Status != "no_release" {
		t.Fatalf("no release output %s", data)
	}
	if _, err = os.Lstat(dir); !os.IsNotExist(err) {
		t.Fatal("unexpected directory", err)
	}
	if err = os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(dir, "keep")
	if err = os.WriteFile(marker, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd = exec.Command("go", args...)
	if data, err = cmd.CombinedOutput(); err == nil {
		t.Fatalf("existing output accepted: %s", data)
	}
	if data, err = os.ReadFile(marker); err != nil || string(data) != "keep" {
		t.Fatal("existing output changed")
	}
}

func TestRejectExistingCandidateChangelogSection(t *testing.T) {
	for _, tc := range []struct {
		name, content string
		conflict      bool
	}{
		{"plain", "# Notes\n\n## 1.1.1\nold release\n", true},
		{"linked", "# Notes\n\n## [1.1.1](https://example.test/compare) (2026-01-01)\n", true},
		{"prefixed", "# Notes\r\n## v1.1.1\r\n", true},
		{"fenced sample", "# Notes\n```markdown\n## 1.1.1\n```\n", false},
		{"other version", "# Notes\n## 1.1.10\n", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			o, _ := repo(t, "1.1.0")
			if err := os.WriteFile(filepath.Join(o.repo, "CHANGELOG.md"), []byte(tc.content), 0o644); err != nil {
				t.Fatal(err)
			}
			run(t, o.repo, "add", "CHANGELOG.md")
			run(t, o.repo, "commit", "--amend", "--no-edit")
			o.baseSHA = run(t, o.repo, "rev-parse", "HEAD")
			run(t, o.repo, "tag", "-f", o.baseTag)
			commit(t, &o, "fix: update")
			o.outputDir = filepath.Join(outside(t), "out")
			r, err := calculate(o)
			if err != nil {
				t.Fatal(err)
			}
			err = writeReleaseFiles(o, r)
			if (err != nil) != tc.conflict {
				t.Fatalf("conflict=%v err=%v", tc.conflict, err)
			}
			if tc.conflict {
				if _, err = os.Lstat(o.outputDir); !os.IsNotExist(err) {
					t.Fatal("created output on conflict")
				}
			}
		})
	}
}
