package validation

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/netip"
	"strings"
	"unicode"
	"unicode/utf8"

	t "github.com/dkoshenkov/paas-common/pkg/http/paas/types"
)

const (
	ApplicationPurpose = "APPLICATION"
	MigratorPurpose    = "MIGRATOR"
)

// ApplicationDatabaseName derives the stable PostgreSQL identifier for registration and reads.
func ApplicationDatabaseName(appID string) (string, error) {
	if !catalogID(appID) {
		return "", errors.New("invalid application ID")
	}
	if len(appID) == 32 && strings.Trim(appID, "0123456789abcdef") == "" {
		return "app_" + appID, nil
	}
	sum := sha256.Sum256([]byte("legacy\x00" + appID))
	return "app_" + hex.EncodeToString(sum[:16]), nil
}

func CredentialsRequest(q t.DatabaseCredentialsRequest) error {
	if !catalogID(q.AppId) || (q.Purpose != ApplicationPurpose && q.Purpose != MigratorPurpose) {
		return errors.New("invalid credentials target")
	}
	return nil
}

// CredentialsResponse verifies the requested principal and avoids putting values in errors.
func CredentialsResponse(q t.DatabaseCredentialsRequest, r t.DatabaseCredentialsResponse) error {
	if err := CredentialsRequest(q); err != nil {
		return err
	}
	db, _ := ApplicationDatabaseName(q.AppId)
	login := db
	if q.Purpose == MigratorPurpose {
		login += "_migrator"
	}
	if r.AppId != q.AppId || r.Purpose != q.Purpose || r.Database != db || r.Username != login {
		return errors.New("credentials target mismatch")
	}
	if _, err := netip.ParseAddr(r.Host); !dnsName(r.Host) && err != nil {
		return errors.New("invalid database host")
	}
	if r.Port != 5432 || !validCredential(r.Password) {
		return errors.New("invalid database credentials")
	}
	return nil
}

func validCredential(s string) bool {
	return s != "" && utf8.ValidString(s) && !strings.ContainsFunc(s, func(r rune) bool { return unicode.IsControl(r) || unicode.In(r, unicode.Cf, unicode.Zl, unicode.Zp) })
}
