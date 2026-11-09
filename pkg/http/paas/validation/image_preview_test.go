package validation

import (
	"testing"

	t "github.com/dkoshenkov/paas-common/pkg/http/paas/types"
)

func TestImagePreviewRequest(tst *testing.T) {
	good := t.ImagepreviewTypesImagePreviewRequest{AppId: "app", ComponentId: "component", DeploymentName: "web", ContainerName: "api", Image: "example/app:v2"}
	if err := ImagePreviewRequest(good); err != nil {
		tst.Fatal(err)
	}
	for _, mutate := range []func(*t.ImagepreviewTypesImagePreviewRequest){func(q *t.ImagepreviewTypesImagePreviewRequest) { q.AppId = "bad/" }, func(q *t.ImagepreviewTypesImagePreviewRequest) { q.ContainerName = "bad/name" }, func(q *t.ImagepreviewTypesImagePreviewRequest) { q.Image = "not an image" }} {
		q := good
		mutate(&q)
		if ImagePreviewRequest(q) == nil {
			tst.Fatalf("invalid request accepted: %+v", q)
		}
	}
}
