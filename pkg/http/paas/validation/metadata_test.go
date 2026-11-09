package validation

import (
	"strings"
	"testing"

	t "github.com/dkoshenkov/paas-common/pkg/http/paas/types"
)

func TestMetadata(tst *testing.T) {
	images := t.MetadataTypesComponentImages{AppId: "app", ComponentId: "comp", Deployments: []t.MetadataTypesDeploymentImages{{Name: "web", Containers: []t.MetadataTypesContainerImage{{Name: "app", Type: "CONTAINER", Image: "repo/app:v1"}, {Name: "init", Type: "INIT", Image: "repo/init:v1"}}}}}
	if err := ComponentImages(images); err != nil {
		tst.Fatal(err)
	}
	images.Deployments[0].Containers[0].Type = "SIDECAR"
	if ComponentImages(images) == nil {
		tst.Fatal("invalid container type accepted")
	}
	images.Deployments[0].Containers[0].Type = "CONTAINER"
	images.Deployments[0].Containers[0].Image = strings.Repeat("x", MaxHistoryPageBytes+1)
	if ComponentImages(images) == nil {
		tst.Fatal("oversized image accepted")
	}
	if ComponentImages(t.MetadataTypesComponentImages{AppId: "app", ComponentId: "comp", Deployments: []t.MetadataTypesDeploymentImages{}}) != nil {
		tst.Fatal("empty deployments rejected")
	}
	if ComponentImagesQuery(t.MetadataMetadataComponentImagesQuery{AppId: "a/", ComponentId: "b"}) == nil {
		tst.Fatal("invalid ID accepted")
	}
	db := t.MetadataTypesAppDatabaseMetadata{AppId: "app", SecretName: "paas-app-db", Keys: []string{"database", "host", "password", "port", "username"}}
	if err := AppDatabaseMetadata(db); err != nil {
		tst.Fatal(err)
	}
	db.SecretName = "paas-migrator-db"
	if AppDatabaseMetadata(db) == nil {
		tst.Fatal("migrator secret accepted")
	}
	db.SecretName = "paas-app-db"
	db.Keys[0] = "password"
	if AppDatabaseMetadata(db) == nil {
		tst.Fatal("duplicate key accepted")
	}
}
