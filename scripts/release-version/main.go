// Command release-version calculates an offline release candidate without modifying Git.
// Example: go run ./scripts/release-version --repo /path/to/repo --ref refs/heads/main --head <full-sha> --base-tag v1.1.0 --base-sha <full-sha> --manifest configs/.release-please-manifest.json
// Bootstrap: repeat --bootstrap-root <full-root-sha> for all reachable roots;
// this replaces --base-tag/--base-sha/--manifest and proposes 1.0.0.
// --verified-release-commit may be repeated only with SHAs whose release PR provenance
// the trusted caller has independently verified; commit text alone is not evidence.
// Verify: --verify-pr --project paas-ctl --head <trusted-main-sha> --pr-head <pr-sha>
// --release-anchor <version>@<sha> comes from trusted configuration, never PR data.
// Tests: go test -race ./scripts/release-version
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

func parseManifest(data string) (map[string]string, error) {
	d := json.NewDecoder(strings.NewReader(data))
	token, err := d.Token()
	if err != nil || token != json.Delim('{') {
		return nil, errors.New("invalid release manifest")
	}
	entries := map[string]string{}
	for d.More() {
		key, err := d.Token()
		if err != nil {
			return nil, errors.New("invalid release manifest")
		}
		name, ok := key.(string)
		if !ok {
			return nil, errors.New("invalid release manifest")
		}
		if _, exists := entries[name]; exists {
			return nil, errors.New("duplicate manifest entry")
		}
		var value string
		if d.Decode(&value) != nil {
			return nil, errors.New("invalid release manifest")
		}
		entries[name] = value
	}
	if _, err = d.Token(); err != nil {
		return nil, errors.New("invalid release manifest")
	}
	if _, err = d.Token(); err != io.EOF {
		return nil, errors.New("invalid release manifest")
	}
	return entries, nil
}

type options struct {
	repo, ref, head, baseTag, baseSHA, manifest, outputDir string
	bootstrapRoots, releases                               stringsFlag
	verifyPR                                               bool
	prHead, project, releaseAnchor                         string
}
type stringsFlag []string

func (v *stringsFlag) String() string     { return strings.Join(*v, ",") }
func (v *stringsFlag) Set(s string) error { *v = append(*v, s); return nil }

type result struct {
	Status      string `json:"status"`
	Version     string `json:"version,omitempty"`
	Previous    string `json:"previous,omitempty"`
	Base        string `json:"base"`
	Head        string `json:"head"`
	UserCommits int    `json:"user_commits"`
	OutputDir   string `json:"output_dir,omitempty"`
	PRHead      string `json:"pr_head,omitempty"`
}
type version struct{ major, minor, patch uint64 }

var (
	semverRE   = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)
	shaRE      = regexp.MustCompile(`^[0-9a-f]{40}$|^[0-9a-f]{64}$`)
	patchRE    = regexp.MustCompile(`^fix(?:\([^()\r\n]+\))?: .+`)
	breakingRE = regexp.MustCompile(`(?m)^BREAKING[ -]CHANGE:`)
	overrideRE = regexp.MustCompile(`(?im)^[ \t]*(?:release-as\s*:|begin_commit_override\b|end_commit_override\b)`)
)

func parseVersion(s string) (version, error) {
	m := semverRE.FindStringSubmatch(s)
	if m == nil {
		return version{}, errors.New("invalid stable version")
	}
	var v version
	var err error
	v.major, err = strconv.ParseUint(m[1], 10, 32)
	if err != nil {
		return v, err
	}
	v.minor, err = strconv.ParseUint(m[2], 10, 32)
	if err != nil {
		return v, err
	}
	v.patch, err = strconv.ParseUint(m[3], 10, 32)
	if err != nil {
		return v, err
	}
	if v.major >= 2 {
		return v, errors.New("automatic v2 release is not approved")
	}
	return v, nil
}
func (v version) String() string { return fmt.Sprintf("%d.%d.%d", v.major, v.minor, v.patch) }
func gitBytes(o options, args ...string) ([]byte, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = o.repo
	for _, item := range os.Environ() {
		if !strings.HasPrefix(item, "GIT_") {
			cmd.Env = append(cmd.Env, item)
		}
	}
	cmd.Env = append(cmd.Env, "GIT_NO_REPLACE_OBJECTS=1", "GIT_NO_LAZY_FETCH=1", "GIT_OPTIONAL_LOCKS=0", "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_TERMINAL_PROMPT=0")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	b, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git %s failed: %w: %s", args[0], err, strings.TrimSpace(stderr.String()))
	}
	return b, nil
}

func git(o options, args ...string) (string, error) {
	b, err := gitBytes(o, args...)
	return strings.TrimSpace(string(b)), err
}

func commitSHA(o options, ref string) (string, error) {
	return git(o, "rev-parse", "--verify", "--end-of-options", ref+"^{commit}")
}

func finish(o options, r result) (result, error) {
	head, err := commitSHA(o, o.ref)
	if err != nil || head != o.head {
		return result{}, errors.New("branch changed during calculation")
	}
	return r, nil
}

func complete(o options) error {
	shallow, err := git(o, "rev-parse", "--is-shallow-repository")
	if err != nil || shallow != "false" {
		return errors.New("shallow or unavailable history")
	}
	replacements, err := git(o, "for-each-ref", "--format=%(refname)", "refs/replace/")
	if err != nil || replacements != "" {
		return errors.New("replacement refs make history ambiguous")
	}
	grafts, err := git(o, "rev-parse", "--git-path", "info/grafts")
	if err != nil {
		return err
	}
	if !path.IsAbs(grafts) {
		grafts = path.Join(o.repo, grafts)
	}
	if data, e := os.ReadFile(grafts); e == nil && strings.TrimSpace(string(data)) != "" {
		return errors.New("grafted history is not accepted")
	} else if e != nil && !os.IsNotExist(e) {
		return errors.New("cannot verify grafts")
	}
	// Missing objects include partial-clone gaps; no network fetch is permitted.
	missing, err := git(o, "-c", "remote.origin.promisor=false", "rev-list", "--objects", "--missing=print", o.head)
	if err != nil {
		return errors.New("incomplete history")
	}
	for _, line := range strings.Split(missing, "\n") {
		if strings.HasPrefix(line, "?") {
			return errors.New("missing Git object")
		}
	}
	return nil
}

func calculate(o options) (result, error) {
	var zero result
	if o.repo == "" || o.ref != "refs/heads/main" || !shaRE.MatchString(o.head) {
		return zero, errors.New("repository, refs/heads/main and exact head SHA required")
	}
	head, err := commitSHA(o, o.ref)
	if err != nil || head != o.head {
		return zero, errors.New("branch head does not match requested SHA")
	}
	if err = complete(o); err != nil {
		return zero, err
	}
	releases := map[string]bool{}
	for _, id := range o.releases {
		if !shaRE.MatchString(id) || releases[id] {
			return zero, errors.New("invalid or duplicate verified release commit")
		}
		releases[id] = true
	}
	bootstrap := len(o.bootstrapRoots) > 0
	if bootstrap {
		if o.baseTag != "" || o.baseSHA != "" || o.manifest != "" || len(releases) != 0 {
			return zero, errors.New("invalid bootstrap inputs")
		}
		confirmed := map[string]bool{}
		for _, root := range o.bootstrapRoots {
			if !shaRE.MatchString(root) || confirmed[root] {
				return zero, errors.New("invalid bootstrap root")
			}
			confirmed[root] = true
		}
		tags, err := git(o, "tag", "--list", "v*")
		if err != nil {
			return zero, err
		}
		for _, tag := range strings.Split(tags, "\n") {
			if semverRE.MatchString(strings.TrimPrefix(tag, "v")) {
				return zero, errors.New("bootstrap requires no existing stable release tags")
			}
		}
		roots, err := git(o, "rev-list", "--max-parents=0", o.head)
		if err != nil {
			return zero, errors.New("cannot read bootstrap roots")
		}
		actual := strings.Split(roots, "\n")
		if len(actual) != len(confirmed) {
			return zero, errors.New("bootstrap roots do not match full history")
		}
		for _, root := range actual {
			if !confirmed[root] {
				return zero, errors.New("bootstrap roots do not match full history")
			}
		}
		sort.Strings(actual)
		if _, err = git(o, "cat-file", "-e", o.head+":configs/.release-please-manifest.json"); err == nil {
			return zero, errors.New("bootstrap manifest already exists")
		}
		ids, err := git(o, "rev-list", o.head)
		if err != nil {
			return zero, err
		}
		all := strings.Split(ids, "\n")
		for _, id := range all {
			msg, err := git(o, "show", "-s", "--format=%B", id)
			if err != nil {
				return zero, err
			}
			if overrideRE.MatchString(msg) {
				return zero, errors.New("untrusted release override")
			}
		}
		return finish(o, result{Status: "release", Version: "1.0.0", Base: strings.Join(actual, ","), Head: o.head, UserCommits: len(all)})
	}
	if o.baseTag == "" || !strings.HasPrefix(o.baseTag, "v") || !shaRE.MatchString(o.baseSHA) || o.manifest == "" || path.Clean(o.manifest) != o.manifest || path.IsAbs(o.manifest) || strings.HasPrefix(o.manifest, "../") || strings.Contains(o.manifest, ":") {
		return zero, errors.New("confirmed tag, SHA and manifest path required")
	}
	prev, err := parseVersion(strings.TrimPrefix(o.baseTag, "v"))
	if err != nil {
		return zero, err
	}
	base, err := commitSHA(o, "refs/tags/"+o.baseTag)
	if err != nil || base != o.baseSHA {
		return zero, errors.New("release tag does not match confirmed boundary")
	}
	if _, err = git(o, "merge-base", "--is-ancestor", base, o.head); err != nil {
		return zero, errors.New("release boundary is not an ancestor")
	}
	tags, err := git(o, "tag", "--list", "v*")
	if err != nil {
		return zero, err
	}
	for _, tag := range strings.Split(tags, "\n") {
		m := semverRE.FindStringSubmatch(strings.TrimPrefix(tag, "v"))
		if m == nil {
			continue
		}
		major, e1 := strconv.ParseUint(m[1], 10, 32)
		minor, e2 := strconv.ParseUint(m[2], 10, 32)
		patch, e3 := strconv.ParseUint(m[3], 10, 32)
		if e1 != nil || e2 != nil || e3 != nil {
			return zero, errors.New("invalid release tag version")
		}
		if major < prev.major || major == prev.major && minor < prev.minor || major == prev.major && minor == prev.minor && patch <= prev.patch {
			continue
		}
		tagCommit, err := commitSHA(o, "refs/tags/"+tag)
		if err != nil {
			return zero, errors.New("invalid release tag")
		}
		_, err = git(o, "merge-base", "--is-ancestor", tagCommit, o.head)
		if err == nil {
			return zero, errors.New("a newer release tag is already reachable")
		}
		var exit *exec.ExitError
		if !errors.As(err, &exit) || exit.ExitCode() != 1 {
			return zero, errors.New("cannot verify release tags")
		}
	}
	for _, revision := range []string{base, o.head} {
		data, err := git(o, "show", revision+":"+o.manifest)
		if err != nil {
			return zero, errors.New("release manifest is unavailable")
		}
		manifest, err := parseManifest(data)
		if err != nil || manifest["."] != prev.String() {
			return zero, errors.New("manifest conflicts with release boundary")
		}
	}
	ids, err := git(o, "rev-list", "--reverse", base+".."+o.head)
	if err != nil {
		return zero, err
	}
	seen := map[string]bool{}
	bump := 0
	count := 0
	if ids != "" {
		for _, id := range strings.Split(ids, "\n") {
			seen[id] = true
			if releases[id] {
				continue
			}
			msg, err := git(o, "show", "-s", "--format=%B", id)
			if err != nil {
				return zero, err
			}
			if overrideRE.MatchString(msg) {
				return zero, errors.New("untrusted release override")
			}
			count++
			subject, _, _ := strings.Cut(msg, "\n")
			if patchRE.MatchString(subject) && !breakingRE.MatchString(msg) {
				if bump < 1 {
					bump = 1
				}
			} else {
				bump = 2
			}
		}
	}
	for id := range releases {
		if !seen[id] {
			return zero, errors.New("verified release commit outside calculated range")
		}
	}
	out := result{Status: "no_release", Previous: prev.String(), Base: base, Head: o.head, UserCommits: count}
	if bump == 0 {
		return finish(o, out)
	}
	if bump == 1 {
		if prev.patch == math.MaxUint32 {
			return zero, errors.New("patch version exceeds supported limit")
		}
		prev.patch++
	} else {
		if prev.minor == math.MaxUint32 {
			return zero, errors.New("minor version exceeds supported limit")
		}
		prev.minor++
		prev.patch = 0
	}
	out.Status = "release"
	out.Version = prev.String()
	_, err = git(o, "show-ref", "--verify", "--quiet", "refs/tags/v"+out.Version)
	if err == nil {
		return zero, errors.New("candidate release tag already exists")
	}
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 1 {
		return zero, errors.New("cannot verify candidate tag")
	}
	return finish(o, out)
}

func main() {
	var o options
	flag.StringVar(&o.repo, "repo", ".", "repository path")
	flag.StringVar(&o.ref, "ref", "refs/heads/main", "exact target branch ref")
	flag.StringVar(&o.head, "head", "", "exact target commit SHA")
	flag.StringVar(&o.baseTag, "base-tag", "", "confirmed previous release tag")
	flag.StringVar(&o.baseSHA, "base-sha", "", "confirmed tag commit SHA")
	flag.StringVar(&o.manifest, "manifest", "", "manifest path at target commit")
	flag.StringVar(&o.outputDir, "output-dir", "", "write release files outside the source repository")
	flag.BoolVar(&o.verifyPR, "verify-pr", false, "verify release PR Git objects without checkout")
	flag.StringVar(&o.prHead, "pr-head", "", "exact untrusted PR head SHA")
	flag.StringVar(&o.project, "project", "", "trusted project name")
	flag.StringVar(&o.releaseAnchor, "release-anchor", "", "independently verified version@commit for an existing release")
	flag.Var(&o.bootstrapRoots, "bootstrap-root", "confirmed root commit for 1.0.0 bootstrap (repeat for each root)")
	flag.Var(&o.releases, "verified-release-commit", "trusted release PR commit SHA (repeatable)")
	flag.Parse()
	if flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "unexpected positional arguments")
		os.Exit(2)
	}
	var r result
	var err error
	if o.verifyPR {
		r, err = verifyReleasePR(o)
	} else {
		if o.prHead != "" || o.project != "" || o.releaseAnchor != "" {
			fmt.Fprintln(os.Stderr, "PR options require --verify-pr")
			os.Exit(2)
		}
		r, err = calculate(o)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if o.outputDir != "" {
		if r.Status == "release" {
			err = writeReleaseFiles(o, r)
			if err == nil {
				r.OutputDir = o.outputDir
			}
		} else {
			err = checkNoReleaseOutput(o)
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
	if err = json.NewEncoder(os.Stdout).Encode(r); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
