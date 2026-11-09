package main

import (
	"bytes"
	"errors"
	"os/exec"
	"strconv"
	"strings"
)

// Profiles are maintained in trusted verifier code, never in PR content.
var bootstrapProfiles = map[string][]string{
	"paas-ctl":    {"0344eb7fe3b6dad5ab47bf637352c79c8a2e7b8d"},
	"paas-daemon": {"9f5bdc37f04f660dd0726c08166489db48e3fd6c"},
}

const commonInitialReleaseSHA = "7b22045ec8240932024a263c742f46a7529afcb6"

func verifyReleasePR(o options) (result, error) {
	if o.project != "paas-common" && o.project != "paas-ctl" && o.project != "paas-daemon" {
		return result{}, errors.New("unknown trusted project")
	}
	return verifyReleasePRProfile(o, bootstrapProfiles[o.project], o.project != "paas-common")
}

func verifyReleasePRProfile(o options, roots []string, bootstrapAllowed bool) (result, error) {
	var zero result
	if !o.verifyPR || o.baseTag != "" || o.baseSHA != "" || o.manifest != "" || o.outputDir != "" || len(o.bootstrapRoots) != 0 || len(o.releases) != 0 || !shaRE.MatchString(o.prHead) {
		return zero, errors.New("invalid PR verification inputs")
	}
	pr, err := commitSHA(o, o.prHead)
	if err != nil || pr != o.prHead {
		return zero, errors.New("invalid PR commit")
	}
	sizeText, err := git(o, "cat-file", "-s", pr)
	if err != nil {
		return zero, err
	}
	size, err := strconv.ParseInt(sizeText, 10, 64)
	if err != nil || size > 1<<20 {
		return zero, errors.New("PR commit object is too large")
	}
	parents, err := git(o, "rev-list", "--parents", "-n", "1", pr)
	if err != nil {
		return zero, err
	}
	parts := strings.Fields(parents)
	if len(parts) != 2 || parts[0] != pr || parts[1] != o.head {
		return zero, errors.New("PR must be a single commit on the exact main SHA")
	}
	message, err := git(o, "show", "-s", "--format=%B", pr)
	if err != nil {
		return zero, err
	}
	if overrideRE.MatchString(message) {
		return zero, errors.New("untrusted PR commit override")
	}
	base := o
	base.verifyPR = false
	base.prHead = ""
	base.project = ""
	data, exists, err := sourceFile(base, manifestPath)
	if err != nil {
		return zero, err
	}
	if exists {
		manifest, err := parseManifest(string(data))
		if err != nil {
			return zero, err
		}
		previous := manifest["."]
		if _, err = parseVersion(previous); err != nil {
			return zero, err
		}
		anchorVersion, anchorSHA, hasAnchor := strings.Cut(o.releaseAnchor, "@")
		if o.project == "paas-common" && previous == "1.1.0" {
			if !hasAnchor && o.releaseAnchor == "" {
				anchorVersion, anchorSHA, hasAnchor = previous, commonInitialReleaseSHA, true
			}
			if !hasAnchor || anchorVersion != previous || anchorSHA != commonInitialReleaseSHA {
				return zero, errors.New("common initial release anchor mismatch")
			}
		}
		if !hasAnchor || anchorVersion != previous || !shaRE.MatchString(anchorSHA) {
			return zero, errors.New("independently verified release anchor required")
		}
		base.baseTag = "v" + previous
		base.baseSHA = anchorSHA
		base.manifest = manifestPath
	} else {
		if o.releaseAnchor != "" {
			return zero, errors.New("bootstrap must not have a release anchor")
		}
		if !bootstrapAllowed || len(roots) == 0 {
			return zero, errors.New("unapproved or unknown bootstrap boundary")
		}
		base.bootstrapRoots = append(stringsFlag(nil), roots...)
	}
	calculated, err := calculate(base)
	if err != nil {
		return zero, err
	}
	if calculated.Status != "release" {
		return zero, errors.New("release PR has no user changes to release")
	}
	expected, err := releaseFiles(base, calculated)
	if err != nil {
		return zero, err
	}
	// First detect any other path without materializing an untrusted diff of
	// arbitrary size. The second query can return only the two fixed names.
	_, err = git(base, "diff-tree", "--quiet", "--no-ext-diff", "--no-textconv", "--no-renames", "-r", base.head, pr, "--", ":(top)**", ":(top,exclude)CHANGELOG.md", ":(top,exclude)configs/.release-please-manifest.json")
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) && exit.ExitCode() == 1 {
			return zero, errors.New("PR changes unexpected files")
		}
		return zero, err
	}
	paths, err := gitBytes(base, "diff-tree", "--no-commit-id", "--no-ext-diff", "--no-textconv", "--no-renames", "--name-only", "-r", "-z", base.head, pr, "--", ":(top)CHANGELOG.md", ":(top)configs/.release-please-manifest.json")
	if err != nil {
		return zero, err
	}
	names := strings.Split(strings.TrimSuffix(string(paths), "\x00"), "\x00")
	if len(names) != len(expected) {
		return zero, errors.New("PR changes unexpected files")
	}
	seen := map[string]bool{}
	for _, name := range names {
		want, ok := expected[name]
		if !ok || seen[name] {
			return zero, errors.New("PR changes unexpected files")
		}
		seen[name] = true
		entry, err := git(base, "ls-tree", pr, "--", name)
		if err != nil {
			return zero, err
		}
		meta, file, ok := strings.Cut(entry, "\t")
		fields := strings.Fields(meta)
		if !ok || file != name || len(fields) != 3 || fields[0] != "100644" || fields[1] != "blob" {
			return zero, errors.New("PR output must be a non-executable regular blob")
		}
		if !shaRE.MatchString(fields[2]) {
			return zero, errors.New("invalid PR blob identity")
		}
		sizeText, err := git(base, "cat-file", "-s", fields[2])
		if err != nil {
			return zero, err
		}
		size, err := strconv.ParseInt(sizeText, 10, 64)
		if err != nil || size != int64(len(want)) {
			return zero, errors.New("PR output size differs from trusted calculation")
		}
		actual, err := gitBytes(base, "show", pr+":"+name)
		if err != nil {
			return zero, err
		}
		if !bytes.Equal(actual, want) {
			return zero, errors.New("PR output differs from trusted calculation")
		}
	}
	again, err := calculate(base)
	if err != nil || again != calculated {
		return zero, errors.New("trusted release inputs changed during verification")
	}
	calculated.Status = "verified"
	calculated.PRHead = pr
	return calculated, nil
}
