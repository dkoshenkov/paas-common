package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
)

const manifestPath = "configs/.release-please-manifest.json"

// sourceFile reads only a regular file from the verified commit, never worktree.
func sourceFile(o options, name string) ([]byte, bool, error) {
	entry, err := git(o, "ls-tree", o.head, "--", name)
	if err != nil {
		return nil, false, err
	}
	if entry == "" {
		return nil, false, nil
	}
	meta, file, ok := strings.Cut(entry, "\t")
	fields := strings.Fields(meta)
	if !ok || file != name || len(fields) != 3 || fields[1] != "blob" || fields[0] != "100644" && fields[0] != "100755" {
		return nil, false, errors.New("release input is not a regular file")
	}
	data, err := gitBytes(o, "show", o.head+":"+name)
	if err != nil {
		return nil, false, err
	}
	if len(data) > 16<<20 {
		return nil, false, errors.New("release input too large")
	}
	return data, true, nil
}

// Release messages are plain text, not trusted Markdown, HTML, or mentions.
func safeSubject(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case unicode.IsControl(r) || unicode.Is(unicode.Cf, r):
			fmt.Fprintf(&b, "[U+%04X]", r)
		default:
			b.WriteRune(r)
		}
	}
	if b.Len() == 0 {
		b.WriteString("(empty subject)")
	}
	value := b.String()
	longest, current := 0, 0
	for _, r := range value {
		if r == '`' {
			current++
			if current > longest {
				longest = current
			}
		} else {
			current = 0
		}
	}
	fence := strings.Repeat("`", longest+1)
	return fence + " " + value + " " + fence
}

// A BOM and all original line endings/content are kept byte-for-byte.
func changelogInsertion(old []byte) (int, string) {
	start := 0
	for start < len(old) {
		end := bytes.IndexByte(old[start:], '\n')
		if end < 0 {
			end = len(old)
		} else {
			end += start + 1
		}
		line := old[start:end]
		if start == 0 {
			line = bytes.TrimPrefix(line, []byte("\xef\xbb\xbf"))
		}
		if bytes.HasPrefix(line, []byte("# ")) || bytes.HasPrefix(line, []byte("#\t")) || bytes.Equal(bytes.TrimRight(line, "\r\n"), []byte("#")) {
			ending := "\n"
			if bytes.HasSuffix(line, []byte("\r\n")) {
				ending = "\r\n"
			}
			return end, ending
		}
		start = end
	}
	return 0, "\n"
}

func hasReleaseSection(old []byte, version string) bool {
	quoted := regexp.QuoteMeta(version)
	heading := regexp.MustCompile(`^ {0,3}##[\t ]+(?:\[v?` + quoted + `\](?:\([^\r\n)]*\))?|v?` + quoted + `)(?:[\t ]|$)`)
	var fence byte
	var length int
	for i, line := range bytes.Split(old, []byte{'\n'}) {
		if i == 0 {
			line = bytes.TrimPrefix(line, []byte("\xef\xbb\xbf"))
		}
		line = bytes.TrimSuffix(line, []byte{'\r'})
		trimmed := bytes.TrimLeft(line, " ")
		if len(line)-len(trimmed) <= 3 && len(trimmed) > 0 && (trimmed[0] == '`' || trimmed[0] == '~') {
			char := trimmed[0]
			n := 0
			for n < len(trimmed) && trimmed[n] == char {
				n++
			}
			if n >= 3 {
				if fence == 0 {
					fence = char
					length = n
					continue
				}
				if char == fence && n >= length && len(bytes.TrimSpace(trimmed[n:])) == 0 {
					fence = 0
					length = 0
					continue
				}
			}
		}
		if fence == 0 && heading.Match(line) {
			return true
		}
	}
	return false
}

func releaseFiles(o options, r result) (map[string][]byte, error) {
	if o.manifest != "" && o.manifest != manifestPath {
		return nil, errors.New("release output requires the standard manifest path")
	}
	if len(o.bootstrapRoots) > 0 {
		entry, err := git(o, "ls-tree", o.head, "--", "configs")
		if err != nil {
			return nil, err
		}
		if entry != "" && !strings.HasPrefix(entry, "040000 tree ") {
			return nil, errors.New("configs in source is not a directory")
		}
	}
	entries := map[string]string{}
	if len(o.bootstrapRoots) == 0 {
		data, ok, err := sourceFile(o, manifestPath)
		if err != nil || !ok {
			return nil, errors.New("manifest source unavailable")
		}
		entries, err = parseManifest(string(data))
		if err != nil {
			return nil, err
		}
		if entries["."] != r.Previous {
			return nil, errors.New("manifest changed")
		}
	}
	entries["."] = r.Version
	manifest, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return nil, err
	}
	manifest = append(manifest, '\n')
	previous, exists, err := sourceFile(o, "CHANGELOG.md")
	if err != nil {
		return nil, err
	}
	if exists && hasReleaseSection(previous, r.Version) {
		return nil, errors.New("candidate release already exists in changelog")
	}
	if !exists {
		previous = []byte("# Changelog\n")
	}
	at, eol := changelogInsertion(previous)
	if at == 0 && bytes.HasPrefix(previous, []byte("\xef\xbb\xbf")) {
		at = 3
	}
	var section bytes.Buffer
	if at > 0 && (at != 3 || !bytes.HasPrefix(previous, []byte("\xef\xbb\xbf"))) && previous[at-1] != '\n' {
		section.WriteString(eol)
	}
	fmt.Fprintf(&section, "%s## %s%s%s", eol, r.Version, eol, eol)
	revision := o.head
	if len(o.bootstrapRoots) == 0 {
		revision = r.Base + ".." + o.head
	}
	ids, err := git(o, "rev-list", "--reverse", revision)
	if err != nil {
		return nil, err
	}
	excluded := map[string]bool{}
	for _, id := range o.releases {
		excluded[id] = true
	}
	count := 0
	if ids != "" {
		for _, id := range strings.Split(ids, "\n") {
			if excluded[id] {
				continue
			}
			subject, err := git(o, "show", "-s", "--format=%s", id)
			if err != nil {
				return nil, err
			}
			fmt.Fprintf(&section, "- %s (`%s`)%s", safeSubject(subject), id, eol)
			count++
		}
	}
	if count != r.UserCommits {
		return nil, errors.New("release range changed")
	}
	section.WriteString(eol)
	var changelog bytes.Buffer
	changelog.Write(previous[:at])
	changelog.Write(section.Bytes())
	changelog.Write(previous[at:])
	return map[string][]byte{"CHANGELOG.md": changelog.Bytes(), manifestPath: manifest}, nil
}

func pathWithoutSymlinks(name string) error {
	if !filepath.IsAbs(name) || filepath.Clean(name) != name {
		return errors.New("output path must be absolute and clean")
	}
	current := string(filepath.Separator)
	for _, part := range strings.Split(strings.TrimPrefix(name, current), string(filepath.Separator)) {
		if part == "" {
			continue
		}
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return errors.New("output path contains a symlink or non-directory")
		}
	}
	return nil
}

func inside(parent, child string) bool {
	rel, err := filepath.Rel(parent, child)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func validateOutputPath(o options) error {
	if o.outputDir == "" || !filepath.IsAbs(o.outputDir) || filepath.Clean(o.outputDir) != o.outputDir {
		return errors.New("output directory must be absolute and clean")
	}
	parent := filepath.Dir(o.outputDir)
	if err := pathWithoutSymlinks(parent); err != nil {
		return err
	}
	bare, err := git(o, "rev-parse", "--is-bare-repository")
	if err != nil {
		return err
	}
	if bare == "false" {
		root, err := git(o, "rev-parse", "--show-toplevel")
		if err != nil {
			return err
		}
		root, err = filepath.EvalSymlinks(root)
		if err != nil {
			return err
		}
		if inside(root, o.outputDir) {
			return errors.New("output directory must be outside source repository")
		}
	} else if bare != "true" {
		return errors.New("unknown repository type")
	}
	common, err := git(o, "rev-parse", "--git-common-dir")
	if err != nil {
		return err
	}
	if !filepath.IsAbs(common) {
		common = filepath.Join(o.repo, common)
	}
	common, err = filepath.EvalSymlinks(common)
	if err != nil {
		return err
	}
	if inside(common, o.outputDir) {
		return errors.New("output directory must be outside Git storage")
	}
	return nil
}

func checkNoReleaseOutput(o options) error {
	if err := validateOutputPath(o); err != nil {
		return err
	}
	_, err := os.Lstat(o.outputDir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	return errors.New("no release but output destination already exists")
}

func verifyExisting(dir string, files map[string][]byte) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	if len(entries) != 2 {
		return errors.New("output directory conflicts with generated files")
	}
	for _, entry := range entries {
		if entry.Name() != "CHANGELOG.md" && entry.Name() != "configs" {
			return errors.New("unexpected output file")
		}
	}
	for name, want := range files {
		full := filepath.Join(dir, filepath.FromSlash(name))
		if err = pathWithoutSymlinks(filepath.Dir(full)); err != nil {
			return err
		}
		info, err := os.Lstat(full)
		if err != nil || !info.Mode().IsRegular() {
			return errors.New("output file is missing or not regular")
		}
		got, err := os.ReadFile(full)
		if err != nil || !bytes.Equal(got, want) {
			return errors.New("output file differs from generated result")
		}
	}
	configs, err := os.ReadDir(filepath.Join(dir, "configs"))
	if err != nil || len(configs) != 1 || configs[0].Name() != ".release-please-manifest.json" {
		return errors.New("unexpected configs output")
	}
	return nil
}

func writeReleaseFiles(o options, r result) error {
	if err := validateOutputPath(o); err != nil {
		return err
	}
	files, err := releaseFiles(o, r)
	if err != nil {
		return err
	}
	// Recheck the same calculation before publishing any result to the caller.
	current, err := calculate(o)
	if err != nil || current != r {
		return errors.New("release inputs changed during generation")
	}
	info, err := os.Lstat(o.outputDir)
	if err == nil {
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return errors.New("output destination is not a directory")
		}
		return verifyExisting(o.outputDir, files)
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	stage, err := os.MkdirTemp(filepath.Dir(o.outputDir), ".release-output-")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(stage) }()
	if err = os.Mkdir(filepath.Join(stage, "configs"), 0o755); err != nil {
		return err
	}
	for name, data := range files {
		if err = os.WriteFile(filepath.Join(stage, filepath.FromSlash(name)), data, 0o644); err != nil {
			return err
		}
	}
	current, err = calculate(o)
	if err != nil || current != r {
		return errors.New("release inputs changed during generation")
	}
	// Reserve the output path exclusively. Hard links publish complete staged
	// files without replacing a path that another process has created.
	if err = os.Mkdir(o.outputDir, 0o700); err != nil {
		return fmt.Errorf("cannot reserve output: %w", err)
	}
	if err = os.Mkdir(filepath.Join(o.outputDir, "configs"), 0o700); err != nil {
		return err
	}
	for name := range files {
		if err = os.Link(filepath.Join(stage, filepath.FromSlash(name)), filepath.Join(o.outputDir, filepath.FromSlash(name))); err != nil {
			return fmt.Errorf("cannot publish output: %w", err)
		}
	}
	return nil
}
