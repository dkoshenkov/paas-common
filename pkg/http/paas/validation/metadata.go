package validation

import (
	"fmt"

	t "github.com/dkoshenkov/paas-common/pkg/http/paas/types"
)

func ComponentImagesQuery(q t.MetadataMetadataComponentImagesQuery) error {
	if !catalogID(q.AppId) || !catalogID(q.ComponentId) {
		return fmt.Errorf("app_id and component_id: invalid ID")
	}
	return nil
}

func AppDatabaseMetadataQuery(q t.MetadataMetadataAppDatabaseMetadataQuery) error {
	if !catalogID(q.AppId) {
		return fmt.Errorf("app_id: invalid ID")
	}
	return nil
}

// ComponentImages validates the projection; callers must compare IDs with the request.
func ComponentImages(v t.MetadataTypesComponentImages) error {
	if !catalogID(v.AppId) || !catalogID(v.ComponentId) || v.Deployments == nil {
		return fmt.Errorf("invalid component images identity or deployments")
	}
	deployments := make(map[string]bool)
	for _, d := range v.Deployments {
		if !dnsName(d.Name) || duplicate(deployments, d.Name) || d.Containers == nil {
			return fmt.Errorf("invalid deployment")
		}
		containers := make(map[string]bool)
		for _, c := range d.Containers {
			if !dnsLabel(c.Name) || duplicate(containers, c.Name) || (c.Type != "CONTAINER" && c.Type != "INIT") || !catalogText(c.Image, MaxHistoryPageBytes) {
				return fmt.Errorf("invalid container")
			}
		}
	}
	return historyWireSize(v, MaxHistoryPageBytes)
}

// AppDatabaseMetadata validates the current application Secret profile; callers
// must compare app_id with the request and verify the authoritative mapping.
func AppDatabaseMetadata(v t.MetadataTypesAppDatabaseMetadata) error {
	if !catalogID(v.AppId) || v.SecretName != "paas-app-db" || len(v.Keys) != 5 {
		return fmt.Errorf("invalid database metadata")
	}
	expected := map[string]bool{"database": true, "host": true, "password": true, "port": true, "username": true}
	for _, key := range v.Keys {
		if !expected[key] {
			return fmt.Errorf("invalid or duplicate database key")
		}
		delete(expected, key)
	}
	return historyWireSize(v, MaxHistoryPageBytes)
}
