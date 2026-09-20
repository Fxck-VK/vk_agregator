package productcatalog_test

import (
	"reflect"
	"testing"

	"vk-ai-aggregator/internal/service/productcatalog"
	"vk-ai-aggregator/internal/service/providermodels"
)

func TestWorkspaceCapabilitiesDescribeActualSubmissionSurface(t *testing.T) {
	catalog, err := productcatalog.WorkspacePreviewCatalog()
	if err != nil {
		t.Fatal(err)
	}
	for _, model := range catalog.Items {
		t.Run(model.ID, func(t *testing.T) {
			c := model.Capabilities
			if c == nil {
				t.Fatal("public model lost API/application capabilities")
			}
			expected := providermodels.Capabilities(model.ID)
			if model.Verification == "pending-verification" {
				candidate, ok := providermodels.MediaCandidateByID(model.ID)
				if !ok {
					t.Fatal("pending model lacks source-backed candidate metadata")
				}
				expected = &candidate.Capabilities
				for _, operation := range model.Operations {
					if operation.Enabled {
						t.Fatal("pending operation became executable")
					}
				}
			}
			if expected == nil || !reflect.DeepEqual(c.API, expected.API) {
				t.Fatal("web controls must not rewrite native API capabilities")
			}
			op := model.Operations[0]
			switch model.Kind {
			case "image":
				v := c.Application.Image
				if v.Images.Support != providermodels.Unsupported || v.Images.MaxCount == nil || *v.Images.MaxCount != 0 {
					t.Fatal("web image submission has no reference input")
				}
				if !reflect.DeepEqual(v.AspectRatios, op.Image.AllowedAspectRatios) || v.MaxOutputCount == nil || *v.MaxOutputCount != op.Image.MaxOutputCount {
					t.Fatal("image metadata differs from priced controls")
				}
			case "video":
				v := c.Application.Video
				if v.Images.Support != providermodels.Unsupported || v.Videos.Support != providermodels.Unsupported || v.Audio.Selectable || v.StartFrame != "unsupported" || v.EndFrame != "unsupported" {
					t.Fatal("web video submission must not advertise unwired controls")
				}
			case "text":
				v := c.Application.Text
				if v.Images.Support != providermodels.Unsupported || v.Videos.Support != providermodels.Unsupported || v.Files.Support != providermodels.Unsupported {
					t.Fatal("web text submission has no attachment input")
				}
			}
		})
	}
}
