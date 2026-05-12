package touchpoint

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"image"
	"image/draw"
	"image/jpeg"
	"net/http"
	"path/filepath"
	"strings"

	_ "image/png"
)

const (
	weComImageMinBytes = 6
	weComImageMaxBytes = 2 * 1024 * 1024
)

type weComPreparedImage struct {
	Bytes       []byte
	Filename    string
	ContentType string
}

func decodeWeComImageBase64(value string) ([]byte, error) {
	raw, err := base64.StdEncoding.DecodeString(stripBase64Prefix(value))
	if err != nil {
		return nil, fmt.Errorf("WeCom image base64 is invalid: %w", err)
	}
	if len(raw) < weComImageMinBytes {
		return nil, fmt.Errorf("WeCom image is too small: %d bytes", len(raw))
	}
	return raw, nil
}

func prepareWeComImages(raw []byte, filename string, contentType string) ([]weComPreparedImage, error) {
	if len(raw) < weComImageMinBytes {
		return nil, fmt.Errorf("WeCom image is too small: %d bytes", len(raw))
	}
	normalized := normalizeWeComImageContentType(raw, filename, contentType)
	if len(raw) <= weComImageMaxBytes && isWeComSupportedImageType(normalized) {
		return []weComPreparedImage{{
			Bytes:       raw,
			Filename:    ensureImageFilename(filename, normalized, 0),
			ContentType: normalized,
		}}, nil
	}
	img, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("WeCom image must be JPG/PNG and <= %d bytes, or decodable for conversion: %w", weComImageMaxBytes, err)
	}
	return encodeWeComImagesAsJPEG(img, filename)
}

func validateWeComPreparedImage(req WeComImageRequest) error {
	raw, err := decodeWeComImageBase64(req.Base64)
	if err != nil {
		return err
	}
	contentType := normalizeWeComImageContentType(raw, req.Filename, req.ContentType)
	if !isWeComSupportedImageType(contentType) {
		return fmt.Errorf("WeCom image content_type %q is not supported; use image/png or image/jpeg", firstNonEmpty(req.ContentType, http.DetectContentType(raw)))
	}
	if len(raw) > weComImageMaxBytes {
		return fmt.Errorf("WeCom image is too large: %d bytes exceeds %d bytes", len(raw), weComImageMaxBytes)
	}
	return nil
}

func normalizeWeComImageContentType(raw []byte, filename string, contentType string) string {
	detected := strings.ToLower(http.DetectContentType(raw))
	switch detected {
	case "image/jpg":
		return "image/jpeg"
	case "image/jpeg", "image/png":
		return detected
	}
	declared := strings.ToLower(strings.TrimSpace(contentType))
	switch declared {
	case "image/jpg":
		return "image/jpeg"
	case "image/jpeg", "image/png":
		return declared
	}
	switch strings.ToLower(filepath.Ext(filename)) {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	default:
		return detected
	}
}

func isWeComSupportedImageType(contentType string) bool {
	return contentType == "image/png" || contentType == "image/jpeg"
}

func encodeWeComImagesAsJPEG(img image.Image, filename string) ([]weComPreparedImage, error) {
	for _, quality := range []int{88, 82, 74, 66, 58} {
		data, err := encodeJPEGSegment(img, 0, img.Bounds().Dy(), quality)
		if err != nil {
			return nil, err
		}
		if len(data) >= weComImageMinBytes && len(data) <= weComImageMaxBytes {
			return []weComPreparedImage{{
				Bytes:       data,
				Filename:    ensureImageFilename(filename, "image/jpeg", 0),
				ContentType: "image/jpeg",
			}}, nil
		}
	}
	return splitWeComImageAsJPEG(img, filename)
}

func splitWeComImageAsJPEG(img image.Image, filename string) ([]weComPreparedImage, error) {
	bounds := img.Bounds()
	height := bounds.Dy()
	if bounds.Dx() <= 0 || height <= 0 {
		return nil, errors.New("WeCom image has invalid dimensions")
	}
	out := []weComPreparedImage{}
	for y := 0; y < height; {
		segmentHeight, data, err := largestWeComJPEGSegment(img, y, height-y)
		if err != nil {
			return nil, err
		}
		out = append(out, weComPreparedImage{
			Bytes:       data,
			Filename:    ensureImageFilename(filename, "image/jpeg", len(out)+1),
			ContentType: "image/jpeg",
		})
		y += segmentHeight
	}
	return out, nil
}

func largestWeComJPEGSegment(img image.Image, y int, remainingHeight int) (int, []byte, error) {
	lo, hi := 1, remainingHeight
	bestHeight := 0
	var bestBytes []byte
	for lo <= hi {
		mid := lo + (hi-lo)/2
		data, err := encodeJPEGSegment(img, y, mid, 82)
		if err != nil {
			return 0, nil, err
		}
		if len(data) <= weComImageMaxBytes {
			bestHeight = mid
			bestBytes = data
			lo = mid + 1
			continue
		}
		hi = mid - 1
	}
	if bestHeight > 0 && len(bestBytes) >= weComImageMinBytes {
		return bestHeight, bestBytes, nil
	}
	for _, quality := range []int{74, 66, 58, 50, 42, 35} {
		data, err := encodeJPEGSegment(img, y, 1, quality)
		if err != nil {
			return 0, nil, err
		}
		if len(data) >= weComImageMinBytes && len(data) <= weComImageMaxBytes {
			return 1, data, nil
		}
	}
	return 0, nil, errors.New("WeCom image segment cannot be compressed under upload limit")
}

func encodeJPEGSegment(img image.Image, y int, height int, quality int) ([]byte, error) {
	bounds := img.Bounds()
	if height <= 0 || y < 0 || y+height > bounds.Dy() {
		return nil, errors.New("invalid image segment")
	}
	dst := image.NewRGBA(image.Rect(0, 0, bounds.Dx(), height))
	draw.Draw(dst, dst.Bounds(), image.White, image.Point{}, draw.Src)
	draw.Draw(dst, dst.Bounds(), img, image.Point{X: bounds.Min.X, Y: bounds.Min.Y + y}, draw.Over)
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, dst, &jpeg.Options{Quality: quality}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func ensureImageFilename(filename string, contentType string, index int) string {
	stem := safeFilenamePart(strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename)))
	ext := ".png"
	if contentType == "image/jpeg" {
		ext = ".jpg"
	}
	if index > 0 {
		return fmt.Sprintf("%s-%02d%s", stem, index, ext)
	}
	return stem + ext
}
