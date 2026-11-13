package productcatalog

import (
	"testing"
	"vk-ai-aggregator/internal/domain"
)

func TestWorkspaceMotionRouteHasExplicitTransport(t *testing.T) {
	route := VideoRoute{Alias: string(domain.VideoRouteKling26Motion), Enabled: true, RequiresStartImage: true, RequiresReferenceVideo: true}
	if !WorkspaceVideoRouteSupported(route) {
		t.Fatal("Motion transport must be supported explicitly")
	}
	route.Alias = string(domain.VideoRouteVeo31Fast)
	if WorkspaceVideoRouteSupported(route) {
		t.Fatal("other reference routes must remain closed")
	}
	route.Alias = string(domain.VideoRouteKling26Motion)
	route.Enabled = false
	if WorkspaceVideoRouteSupported(route) {
		t.Fatal("disabled Motion route exposed")
	}
}
