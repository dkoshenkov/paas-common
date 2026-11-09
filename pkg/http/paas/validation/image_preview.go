package validation

import (
	"errors"

	"github.com/distribution/reference"

	t "github.com/dkoshenkov/paas-common/pkg/http/paas/types"
)

func ImagePreviewRequest(q t.ImagepreviewTypesImagePreviewRequest) error {
	if !catalogID(q.AppId) || !catalogID(q.ComponentId) || !dnsName(q.DeploymentName) || !dnsLabel(q.ContainerName) || !catalogText(q.Image, 1024) {
		return errors.New("invalid image preview request")
	}
	if _, err := reference.ParseNormalizedNamed(q.Image); err != nil {
		return errors.New("invalid image reference")
	}
	return nil
}
