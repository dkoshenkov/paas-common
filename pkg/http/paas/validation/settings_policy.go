package validation

import (
	"errors"
	"net/netip"
	"regexp"
	"strings"

	t "github.com/dkoshenkov/paas-common/pkg/http/paas/types"
)

var (
	label       = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)
	domainASCII = regexp.MustCompile(`^[a-zA-Z0-9.-]+$`)
	keyName     = regexp.MustCompile(`^[-A-Za-z0-9_.]+$`)
)

func dnsLabel(s string) bool { return len(s) <= 63 && label.MatchString(s) }
func dnsName(s string) bool {
	if len(s) > 253 {
		return false
	}
	for part := range strings.SplitSeq(s, ".") {
		if !dnsLabel(part) {
			return false
		}
	}
	return true
}

func duplicate[T comparable](seen map[T]bool, key T) bool {
	if seen[key] {
		return true
	}
	seen[key] = true
	return false
}

func grantKeys(grant t.SettingsTypesDatabaseGrant) error {
	if grant.WholeSecret != nil {
		if !*grant.WholeSecret || grant.Keys != nil {
			return errors.New("whole_secret requires true and forbids keys")
		}
		return nil
	}
	if grant.Keys == nil || len(*grant.Keys) == 0 {
		return errors.New("keys must select at least one Secret key")
	}
	seen := make(map[string]bool, len(*grant.Keys))
	for _, s := range *grant.Keys {
		if len(s) > 253 || !keyName.MatchString(s) || s == "." || strings.HasPrefix(s, "..") || duplicate(seen, s) {
			return errors.New("invalid or duplicate Secret key")
		}
	}
	return nil
}

func databaseGrants(items []t.SettingsTypesDatabaseGrant) error {
	if items == nil {
		return errors.New("database_grants must be an array")
	}
	seen := make(map[[3]string]bool, len(items))
	for _, g := range items {
		if !dnsName(g.DeploymentName) || !dnsLabel(g.ContainerName) {
			return errors.New("invalid Deployment or container name")
		}
		if g.ContainerKind != "container" && g.ContainerKind != "init" {
			return errors.New("unknown container kind")
		}
		if duplicate(seen, [3]string{g.DeploymentName, g.ContainerKind, g.ContainerName}) {
			return errors.New("duplicate database grant target")
		}
		if err := grantKeys(g); err != nil {
			return err
		}
	}
	return nil
}

func serviceGrants(items []t.SettingsTypesServiceGrant) error {
	if items == nil {
		return errors.New("service_grants must be an array")
	}
	seen := make(map[t.SettingsTypesServiceGrant]bool, len(items))
	for _, g := range items {
		if !catalogID(g.TargetAppId) || !catalogID(g.TargetComponentId) || !dnsLabel(g.ServiceName) {
			return errors.New("invalid target Service reference")
		}
		if duplicate(seen, g) {
			return errors.New("duplicate Service grant")
		}
	}
	return nil
}

func exactDNSName(domain string) (string, bool) {
	if !domainASCII.MatchString(domain) {
		return "", false
	}
	name := strings.ToLower(strings.TrimSuffix(domain, "."))
	_, err := netip.ParseAddr(name)
	return name, err != nil && dnsName(name)
}

func external(e t.SettingsTypesExternal) error {
	if e.Domains == nil {
		return errors.New("external.domains must be an array")
	}
	switch e.Mode {
	case "none", "all":
		if len(e.Domains) != 0 {
			return errors.New("domains are only valid for allowlist mode")
		}
	case "allowlist":
		seen := make(map[string]bool, len(e.Domains))
		for _, domain := range e.Domains {
			name, ok := exactDNSName(domain)
			if !ok || duplicate(seen, name) {
				return errors.New("invalid or duplicate exact DNS name")
			}
		}
	default:
		return errors.New("unknown external mode")
	}
	return nil
}

func componentPolicy(p t.SettingsTypesComponentPolicy) error {
	if err := databaseGrants(p.DatabaseGrants); err != nil {
		return err
	}
	if err := serviceGrants(p.ServiceGrants); err != nil {
		return err
	}
	return external(p.External)
}
