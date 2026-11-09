package validation

import (
	"fmt"
	"net"
	"strings"

	"golang.org/x/net/idna"

	paastypes "github.com/dkoshenkov/paas-common/pkg/http/paas/types"
)

var registrationIDNA = idna.New(idna.ValidateForRegistration(), idna.StrictDomainName(true), idna.VerifyDNSLength(true))

func DomainsListQuery(q paastypes.DomainsDomainsListQuery) error {
	if !catalogID(q.AppId) {
		return fmt.Errorf("app_id: invalid ID")
	}
	return nil
}

func DomainRegisterRequest(q paastypes.DomainsTypesDomainRequest) error {
	return domainRequest(q)
}

func DomainRemoveRequest(q paastypes.DomainsTypesDomainRequest) error {
	return domainRequest(q)
}

func domainRequest(q paastypes.DomainsTypesDomainRequest) error {
	if !catalogID(q.AppId) {
		return fmt.Errorf("app_id: invalid ID")
	}
	_, err := CanonicalDomainName(q.Domain)
	return err
}

// CanonicalDomainName accepts ASCII DNS names and validates any xn-- labels by
// requiring a strict IDNA round-trip. It does not convert Unicode input.
func CanonicalDomainName(domain string) (string, error) {
	if domain == "" || strings.TrimSpace(domain) != domain {
		return "", fmt.Errorf("domain: require an unpadded ASCII DNS name")
	}
	for i := 0; i < len(domain); i++ {
		if domain[i] > 0x7f {
			return "", fmt.Errorf("domain: Unicode is not accepted; use an ASCII A-label")
		}
	}
	canonical := strings.ToLower(domain)
	canonical = strings.TrimSuffix(canonical, ".")
	if len(canonical) == 0 || len(canonical) > 253 || net.ParseIP(canonical) != nil || canonical == "localhost" || strings.HasSuffix(canonical, ".localhost") {
		return "", fmt.Errorf("domain: invalid DNS name")
	}
	labels := strings.Split(canonical, ".")
	if len(labels) < 2 {
		return "", fmt.Errorf("domain: require at least two DNS labels")
	}
	for _, label := range labels {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return "", fmt.Errorf("domain: invalid DNS label")
		}
		for i := range len(label) {
			c := label[i]
			if (c < 'a' || c > 'z') && (c < '0' || c > '9') && c != '-' {
				return "", fmt.Errorf("domain: invalid DNS label")
			}
		}
	}
	unicodeName, err := registrationIDNA.ToUnicode(canonical)
	if err != nil {
		return "", fmt.Errorf("domain: invalid IDNA A-label: %w", err)
	}
	asciiName, err := registrationIDNA.ToASCII(unicodeName)
	if err != nil || asciiName != canonical {
		return "", fmt.Errorf("domain: invalid IDNA A-label")
	}
	return canonical, nil
}

func DomainSnapshot(snapshot paastypes.DomainsTypesDomainSnapshot) error {
	if !catalogID(snapshot.AppId) {
		return fmt.Errorf("app_id: invalid ID")
	}
	canonical, err := CanonicalDomainName(snapshot.Domain)
	if err != nil || canonical != snapshot.Domain {
		return fmt.Errorf("domain: require canonical DNS name")
	}
	return nil
}

func DomainsSnapshot(snapshot paastypes.DomainsTypesDomainsSnapshot) error {
	if !catalogID(snapshot.AppId) {
		return fmt.Errorf("app_id: invalid ID")
	}
	if snapshot.Domains == nil {
		return fmt.Errorf("domains: require an array")
	}
	if snapshot.TlsProfile != nil && *snapshot.TlsProfile != "staging" && *snapshot.TlsProfile != "production" {
		return fmt.Errorf("tls_profile: invalid profile")
	}
	for i, domain := range snapshot.Domains {
		canonical, err := CanonicalDomainName(domain)
		if err != nil || canonical != domain {
			return fmt.Errorf("domains[%d]: require canonical DNS name", i)
		}
		if i > 0 && snapshot.Domains[i-1] >= domain {
			return fmt.Errorf("domains[%d]: names must be unique and sorted", i)
		}
	}
	return nil
}
