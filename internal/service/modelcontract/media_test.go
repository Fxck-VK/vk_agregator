package modelcontract

import "testing"

func TestImageCombinationsAndDefault(t *testing.T) {
	c := readyTextContract()
	c.Operations[0].Kind = "image"
	c.Operations[0].Text = nil
	c.Operations[0].Image = &ImageOutput{Variants: []ImageVariant{{"1:1", "1K"}, {"16:9", "2K"}}, Formats: []string{"image/png"}, MaxOutputCount: 1, Mask: "unsupported"}
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
	c.Operations[0].Image.DefaultVariant = 2
	if err := c.Validate(); err == nil {
		t.Fatal("nonexistent default accepted")
	}
	c.Operations[0].Image.DefaultVariant = 0
	c.Operations[0].Image.Variants[1] = c.Operations[0].Image.Variants[0]
	if err := c.Validate(); err == nil {
		t.Fatal("duplicate image mode accepted")
	}
}

func TestVideoFramesNeedEnoughImageSlots(t *testing.T) {
	c := readyTextContract()
	op := &c.Operations[0]
	op.Kind = "video"
	op.Text = nil
	op.Video = &VideoOutput{Variants: []VideoVariant{{5, "720p", "16:9", 24, false}}, Formats: []string{"video/mp4"}, StartImage: "required", EndImage: "required"}
	if err := c.Validate(); err == nil {
		t.Fatal("frames enabled without input support")
	}
	op.Inputs.MaxTotalBytes = 8192
	op.Inputs.Images = Input{Support: Supported, Enabled: true, Processing: "native", MaxCount: 1, MaxBytes: 4096, Formats: []FileFormat{{".png", "image/png"}}}
	if err := c.Validate(); err == nil {
		t.Fatal("two required frames accepted in one image slot")
	}
	op.Inputs.Images.MaxCount = 2
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
	op.Video.Variants[0].DurationSec = 0
	if err := c.Validate(); err == nil {
		t.Fatal("unbounded video duration accepted")
	}
}

func TestAudioSpeechNeedsLanguagesAndVoices(t *testing.T) {
	c := readyTextContract()
	op := &c.Operations[0]
	op.Kind = "audio"
	op.Text = nil
	op.Audio = &AudioOutput{Tasks: []string{"speech"}, Formats: []string{"audio/mpeg"}, MaxDurationSec: 60}
	if err := c.Validate(); err == nil {
		t.Fatal("speech admitted without voices/languages")
	}
	op.Audio.Languages = []string{"ru"}
	op.Audio.Voices = []string{"example-voice"}
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
	op.Audio.Formats = []string{"image/png"}
	if err := c.Validate(); err == nil {
		t.Fatal("speech accepted an image output format")
	}
}
