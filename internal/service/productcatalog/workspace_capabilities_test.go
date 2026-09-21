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
			if !reflect.DeepEqual(c.API, providermodels.Capabilities(model.ID).API) {
				t.Fatal("web controls must not rewrite native API capabilities")
			}
			op := model.Operations[0]
			switch model.Kind {
			case "image":
				v := c.Application.Image
				if op.Inputs.Images.Enabled {
					if v.Images.Support != providermodels.Supported || v.Images.MaxCount == nil || *v.Images.MaxCount != op.Image.MaxReferenceImages || op.Inputs.Images.MaxCount != op.Image.MaxReferenceImages || op.Inputs.Images.MaxBytes != productcatalog.WebReferenceMaxBytes {
						t.Fatal("reference metadata differs from upload limits")
					}
				} else if v.Images.Support != providermodels.Unsupported || v.Images.MaxCount == nil || *v.Images.MaxCount != 0 {
					t.Fatal("disabled reference input advertised")
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
