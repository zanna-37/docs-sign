package pdfproc

import (
	"bytes"
	"image"
	"image/color"
	"testing"

	"github.com/go-pdf/fpdf"
)

// makeSamplePDF builds a simple multi-page PDF for tests.
func makeSamplePDF(t *testing.T, pages int) []byte {
	t.Helper()
	doc := fpdf.New("P", "pt", "A4", "")
	for i := 0; i < pages; i++ {
		doc.AddPage()
		doc.SetFont("Arial", "B", 24)
		doc.SetXY(72, 72)
		doc.Cell(300, 30, "Sample page")
	}
	var buf bytes.Buffer
	if err := doc.Output(&buf); err != nil {
		t.Fatalf("sample pdf: %v", err)
	}
	return buf.Bytes()
}

func makeSignature() image.Image {
	img := image.NewRGBA(image.Rect(0, 0, 240, 90))
	for y := 0; y < 90; y++ {
		for x := 0; x < 240; x++ {
			// A semi-transparent blue blob so we exercise alpha compositing.
			img.Set(x, y, color.RGBA{R: 20, G: 40, B: 200, A: 180})
		}
	}
	return img
}

func TestSignFlatten(t *testing.T) {
	r, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()

	pdf := makeSamplePDF(t, 2)

	pc, err := r.PageCount(pdf)
	if err != nil {
		t.Fatal(err)
	}
	if pc != 2 {
		t.Fatalf("page count = %d, want 2", pc)
	}

	placements := []Placement{
		{Page: 0, X: 60, Y: 60, W: 180, H: 70, RotationDeg: 12, Image: makeSignature()},
		{Page: 1, X: 200, Y: 400, W: 150, H: 60, RotationDeg: -8, Image: makeSignature()},
	}
	out, n, err := r.Sign(pdf, placements, SignOptions{DPI: 100})
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("signed page count = %d, want 2", n)
	}
	if !bytes.HasPrefix(out, []byte("%PDF")) {
		t.Fatalf("output is not a PDF (prefix %q)", out[:min(8, len(out))])
	}
	// The flattened output must itself be a valid 2-page PDF.
	pc2, err := r.PageCount(out)
	if err != nil {
		t.Fatalf("reload flattened: %v", err)
	}
	if pc2 != 2 {
		t.Fatalf("flattened page count = %d, want 2", pc2)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
