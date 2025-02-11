// handler.go
package function

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	pigo "github.com/esimov/pigo/core"
	"github.com/esimov/stackblur-go"
	"github.com/fogleman/gg"
)

var dc *gg.Context

// FaceDetector struct contains Pigo face detector general settings.
type FaceDetector struct {
	cascadeFile  string
	minSize      int
	maxSize      int
	shiftFactor  float64
	scaleFactor  float64
	iouThreshold float64
}

// DetectionResult contains the coordinates of the detected faces and the base64 converted image.
type DetectionResult struct {
	Faces       []image.Rectangle `json:"faces"`
	ImageBase64 string            `json:"image_base64"`
}

// Handle a serverless request
func Handle(req []byte) (result struct {
	Body       []byte
	Header     http.Header
	StatusCode int
}, err error) {
	// Initialize default headers
	result.Header = make(http.Header)
	result.Header.Set("Content-Type", "application/json")

	// Validate request
	if len(req) == 0 {
		result.StatusCode = http.StatusBadRequest
		result.Body = []byte(`{"error": "Empty request received"}`)
		return result, fmt.Errorf("empty request")
	}

	// Process based on input mode
	var data []byte
	if val, exists := os.LookupEnv("input_mode"); exists && val == "url" {
		inputURL := strings.TrimSpace(string(req))
		res, err := http.Get(inputURL)
		if err != nil {
			result.StatusCode = http.StatusBadRequest
			result.Body = []byte(fmt.Sprintf(`{"error": "Failed to download image: %v"}`, err))
			return result, err
		}
		defer res.Body.Close()

		data, err = ioutil.ReadAll(res.Body)
		if err != nil {
			result.StatusCode = http.StatusInternalServerError
			result.Body = []byte(fmt.Sprintf(`{"error": "Failed to read image data: %v"}`, err))
			return result, err
		}
	} else {
		// Try base64 decode first
		var decodeError error
		data, decodeError = base64.StdEncoding.DecodeString(string(req))
		if decodeError != nil {
			data = req
		}
	}

	// Validate image content type
	contentType := http.DetectContentType(data)
	if contentType != "image/jpeg" && contentType != "image/png" {
		result.StatusCode = http.StatusBadRequest
		result.Body = []byte(`{"error": "Only JPEG or PNG images are supported"}`)
		return result, fmt.Errorf("invalid content type: %s", contentType)
	}

	// Create temporary file
	tmpfile, err := os.CreateTemp("/tmp", "image")
	if err != nil {
		result.StatusCode = http.StatusInternalServerError
		result.Body = []byte(fmt.Sprintf(`{"error": "Failed to create temporary file: %v"}`, err))
		return result, err
	}
	defer os.Remove(tmpfile.Name())

	if _, err := io.Copy(tmpfile, bytes.NewBuffer(data)); err != nil {
		result.StatusCode = http.StatusInternalServerError
		result.Body = []byte(fmt.Sprintf(`{"error": "Failed to write image data: %v"}`, err))
		return result, err
	}

	// Get output mode
	var output string
	query, err := url.ParseQuery(os.Getenv("Http_Query"))
	if err == nil {
		output = query.Get("output")
	}

	if val, exists := os.LookupEnv("output_mode"); exists {
		output = val
	}

	// Initialize face detector
	cascadeFile := "/home/app/data/facefinder"
	if _, err := os.Stat(cascadeFile); os.IsNotExist(err) {
		result.StatusCode = http.StatusInternalServerError
		result.Body = []byte(fmt.Sprintf(`{"error": "Cascade file not found: %v"}`, err))
		return result, err
	}

	fd := NewFaceDetector(cascadeFile, 20, 2000, 0.1, 1.1, 0.18)
	if err := tmpfile.Close(); err != nil {
		result.StatusCode = http.StatusInternalServerError
		result.Body = []byte(fmt.Sprintf(`{"error": "Failed to close temporary file: %v"}`, err))
		return result, err
	}

	// Detect faces
	faces, err := fd.DetectFaces(tmpfile.Name())
	if err != nil {
		result.StatusCode = http.StatusInternalServerError
		result.Body = []byte(fmt.Sprintf(`{"error": "Face detection failed: %v"}`, err))
		return result, err
	}

	// Process output
	if output == "image" || output == "json_image" {
		rects, image, err := fd.DrawFaces(data, faces)
		if err != nil {
			result.StatusCode = http.StatusInternalServerError
			result.Body = []byte(fmt.Sprintf(`{"error": "Failed to process image: %v"}`, err))
			return result, err
		}

		if output == "image" {
			result.Header.Set("Content-Type", "image/jpeg")
			result.StatusCode = http.StatusOK
			result.Body = image
			return result, nil
		}

		resp := DetectionResult{
			Faces:       rects,
			ImageBase64: base64.StdEncoding.EncodeToString(image),
		}

		j, err := json.Marshal(resp)
		if err != nil {
			result.StatusCode = http.StatusInternalServerError
			result.Body = []byte(fmt.Sprintf(`{"error": "Failed to encode response: %v"}`, err))
			return result, err
		}

		result.StatusCode = http.StatusOK
		result.Body = j
	}

	return result, nil
}

// NewFaceDetector initializes the constructor function.
func NewFaceDetector(cf string, minSize, maxSize int, shf, scf, iou float64) *FaceDetector {
	return &FaceDetector{
		cascadeFile:  cf,
		minSize:      minSize,
		maxSize:      maxSize,
		shiftFactor:  shf,
		scaleFactor:  scf,
		iouThreshold: iou,
	}
}

// DetectFaces runs the detection algorithm over the provided source image.
func (fd *FaceDetector) DetectFaces(source string) ([]pigo.Detection, error) {
	src, err := pigo.GetImage(source)
	if err != nil {
		return nil, fmt.Errorf("failed to load image: %v", err)
	}

	pixels := pigo.RgbToGrayscale(src)
	cols, rows := src.Bounds().Max.X, src.Bounds().Max.Y

	dc = gg.NewContext(cols, rows)
	dc.DrawImage(src, 0, 0)

	cParams := pigo.CascadeParams{
		MinSize:     fd.minSize,
		MaxSize:     fd.maxSize,
		ShiftFactor: fd.shiftFactor,
		ScaleFactor: fd.scaleFactor,
		ImageParams: pigo.ImageParams{
			Pixels: pixels,
			Rows:   rows,
			Cols:   cols,
			Dim:    cols,
		},
	}

	cascadeFile, err := ioutil.ReadFile(fd.cascadeFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read cascade file: %v", err)
	}

	pigo := pigo.NewPigo()
	classifier, err := pigo.Unpack(cascadeFile)
	if err != nil {
		return nil, fmt.Errorf("failed to unpack cascade file: %v", err)
	}

	faces := classifier.RunCascade(cParams, 0)
	faces = classifier.ClusterDetections(faces, fd.iouThreshold)

	return faces, nil
}

// DrawFaces blurs the detected faces in the image.
func (fd *FaceDetector) DrawFaces(srcImage []byte, faces []pigo.Detection) ([]image.Rectangle, []byte, error) {
	var qThresh float32 = 5.0
	var rects []image.Rectangle

	img, _, err := image.Decode(bytes.NewReader(srcImage))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to decode image: %v", err)
	}

	for _, face := range faces {
		if face.Q > qThresh {
			rect := image.Rect(
				face.Col-face.Scale/2,
				face.Row-face.Scale/2,
				face.Col+face.Scale/2,
				face.Row+face.Scale/2,
			)
			rects = append(rects, rect)

			subImg := img.(SubImager).SubImage(rect)
			dim := subImg.Bounds().Max.X - subImg.Bounds().Min.X
			sf := int(round(float64(dim) * 0.1))

			blur, err := stackblur.Process(subImg, uint32(sf))
			if err != nil {
				return nil, nil, fmt.Errorf("failed to blur face: %v", err)
			}

			x, y := face.Col-face.Scale/2, face.Row-face.Scale/2
			dc.DrawImage(blur, x, y)
		}
	}

	finalImg := dc.Image()
	filename := fmt.Sprintf("/tmp/%d.jpg", time.Now().UnixNano())

	output, err := os.OpenFile(filename, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create output file: %v", err)
	}
	defer os.Remove(filename)
	defer output.Close()

	if err := jpeg.Encode(output, finalImg, &jpeg.Options{Quality: 100}); err != nil {
		return nil, nil, fmt.Errorf("failed to encode output image: %v", err)
	}

	rf, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read output file: %v", err)
	}

	return rects, rf, nil
}

// SubImager is a wrapper implementing the SubImage method.
type SubImager interface {
	SubImage(r image.Rectangle) image.Image
}

// round returns the nearest integer.
func round(f float64) float64 {
	if f < 0 {
		return float64(int(f - 0.5))
	}
	return float64(int(f + 0.5))
}
