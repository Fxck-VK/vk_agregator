package miniapp

import "testing"

func TestAllowedAspectRatioForDimensions(t *testing.T) {
	tests := []struct {
		name    string
		width   int
		height  int
		allowed []string
		want    string
	}{
		{
			name:    "vertical_common",
			width:   720,
			height:  1280,
			allowed: []string{"16:9", "9:16", "1:1"},
			want:    "9:16",
		},
		{
			name:    "runway_portrait_mid",
			width:   832,
			height:  1104,
			allowed: []string{"16:9", "9:16", "4:3", "3:4", "1:1", "21:9"},
			want:    "3:4",
		},
		{
			name:    "runway_ultrawide",
			width:   1584,
			height:  672,
			allowed: []string{"16:9", "9:16", "4:3", "3:4", "1:1", "21:9"},
			want:    "21:9",
		},
		{
			name:    "portrait_falls_back_to_same_orientation",
			width:   832,
			height:  1104,
			allowed: []string{"16:9", "9:16", "1:1"},
			want:    "9:16",
		},
		{
			name:    "square",
			width:   1024,
			height:  1024,
			allowed: []string{"16:9", "9:16", "1:1"},
			want:    "1:1",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := allowedAspectRatioForDimensions(tc.width, tc.height, tc.allowed)
			if got != tc.want {
				t.Fatalf("aspect ratio = %q, want %q", got, tc.want)
			}
		})
	}
}
