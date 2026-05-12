package touchpoint

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"math/rand"
	"testing"
)

func TestPrepareWeComImagesKeepsSmallPNG(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 16, 16))
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			img.Set(x, y, color.RGBA{R: 80, G: 140, B: 210, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("png.Encode() error = %v", err)
	}

	prepared, err := prepareWeComImages(buf.Bytes(), "answer.png", "image/png")
	if err != nil {
		t.Fatalf("prepareWeComImages() error = %v", err)
	}
	if len(prepared) != 1 || prepared[0].ContentType != "image/png" || prepared[0].Filename != "answer.png" {
		t.Fatalf("prepared = %+v, want original png metadata", prepared)
	}
	if !bytes.Equal(prepared[0].Bytes, buf.Bytes()) {
		t.Fatalf("small png was re-encoded")
	}
}

func TestPrepareWeComImagesCompressesLargePNG(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 900, 4600))
	rng := rand.New(rand.NewSource(17))
	for y := 0; y < 4600; y++ {
		for x := 0; x < 900; x++ {
			img.Set(x, y, color.RGBA{
				R: uint8(rng.Intn(256)),
				G: uint8(rng.Intn(256)),
				B: uint8(rng.Intn(256)),
				A: 255,
			})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("png.Encode() error = %v", err)
	}
	if buf.Len() <= weComImageMaxBytes {
		t.Fatalf("test image size = %d, want above WeCom limit", buf.Len())
	}

	prepared, err := prepareWeComImages(buf.Bytes(), "long-answer.png", "image/png")
	if err != nil {
		t.Fatalf("prepareWeComImages() error = %v", err)
	}
	if len(prepared) == 0 {
		t.Fatalf("prepared no images")
	}
	for _, image := range prepared {
		if image.ContentType != "image/jpeg" {
			t.Fatalf("prepared content_type = %q, want image/jpeg", image.ContentType)
		}
		if len(image.Bytes) > weComImageMaxBytes {
			t.Fatalf("prepared size = %d, want <= %d", len(image.Bytes), weComImageMaxBytes)
		}
		if len(image.Bytes) < weComImageMinBytes {
			t.Fatalf("prepared size = %d, want >= %d", len(image.Bytes), weComImageMinBytes)
		}
	}
}
