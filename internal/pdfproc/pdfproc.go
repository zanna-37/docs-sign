// Package pdfproc renders PDF pages with PDFium (compiled to WebAssembly and run via
// wazero, so there is no cgo dependency), composites signature images onto the rendered
// bitmaps, and reassembles an image-only ("flattened") PDF. Because every page becomes a
// raster image, no original page object — including a signature image — survives in the
// output, satisfying the "impossible to recover the raw signature" requirement.
//
// All work happens in memory; plaintext PDFs and signatures never touch the disk here.
package pdfproc

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"math"
	"time"

	"github.com/go-pdf/fpdf"
	pdfium "github.com/klippa-app/go-pdfium"
	"github.com/klippa-app/go-pdfium/requests"
	"github.com/klippa-app/go-pdfium/webassembly"
	"golang.org/x/image/draw"
	"golang.org/x/image/math/f64"
)

const (
	// DefaultDPI is the rasterization resolution used when none is specified.
	DefaultDPI = 150
	// MaxDPI bounds the requested resolution to keep memory/time in check.
	MaxDPI = 400

	instanceTimeout = 2 * time.Minute
)

// Renderer owns a pool of PDFium WebAssembly instances.
type Renderer struct {
	pool pdfium.Pool
}

// New initializes the PDFium WebAssembly runtime (the wasm module is embedded in the
// binary by the go-pdfium package).
func New() (*Renderer, error) {
	pool, err := webassembly.Init(webassembly.Config{MinIdle: 1, MaxIdle: 2, MaxTotal: 4})
	if err != nil {
		return nil, fmt.Errorf("pdfproc: init pdfium: %w", err)
	}
	return &Renderer{pool: pool}, nil
}

// Close tears down the PDFium runtime.
func (r *Renderer) Close() error { return r.pool.Close() }

// Placement positions one signature image on a page. Coordinates use a top-left origin
// measured in PDF points (1/72 inch); RotationDeg is clockwise about the placement center.
type Placement struct {
	Page        int
	X, Y        float64 // top-left corner, points
	W, H        float64 // size, points
	RotationDeg float64
	Image       image.Image
}

// SignOptions tunes the signing pipeline.
type SignOptions struct {
	DPI int
}

func (r *Renderer) instance() (pdfium.Pdfium, error) {
	return r.pool.GetInstance(instanceTimeout)
}

// PageCount returns the number of pages in a PDF.
func (r *Renderer) PageCount(pdf []byte) (int, error) {
	inst, err := r.instance()
	if err != nil {
		return 0, err
	}
	defer inst.Close()
	doc, err := inst.FPDF_LoadMemDocument(&requests.FPDF_LoadMemDocument{Data: &pdf})
	if err != nil {
		return 0, fmt.Errorf("pdfproc: load pdf: %w", err)
	}
	defer inst.FPDF_CloseDocument(&requests.FPDF_CloseDocument{Document: doc.Document})
	pc, err := inst.FPDF_GetPageCount(&requests.FPDF_GetPageCount{Document: doc.Document})
	if err != nil {
		return 0, err
	}
	return pc.PageCount, nil
}

// Sign renders every page of pdf, composites the placements, and returns a flattened,
// image-only PDF together with its page count.
func (r *Renderer) Sign(pdf []byte, placements []Placement, opts SignOptions) ([]byte, int, error) {
	dpi := opts.DPI
	if dpi <= 0 {
		dpi = DefaultDPI
	}
	if dpi > MaxDPI {
		dpi = MaxDPI
	}

	inst, err := r.instance()
	if err != nil {
		return nil, 0, err
	}
	defer inst.Close()

	doc, err := inst.FPDF_LoadMemDocument(&requests.FPDF_LoadMemDocument{Data: &pdf})
	if err != nil {
		return nil, 0, fmt.Errorf("pdfproc: load pdf: %w", err)
	}
	defer inst.FPDF_CloseDocument(&requests.FPDF_CloseDocument{Document: doc.Document})

	pcResp, err := inst.FPDF_GetPageCount(&requests.FPDF_GetPageCount{Document: doc.Document})
	if err != nil {
		return nil, 0, err
	}
	pageCount := pcResp.PageCount

	byPage := make(map[int][]Placement)
	for _, p := range placements {
		if p.Page < 0 || p.Page >= pageCount {
			return nil, 0, fmt.Errorf("pdfproc: placement page %d out of range (0..%d)", p.Page, pageCount-1)
		}
		byPage[p.Page] = append(byPage[p.Page], p)
	}

	out := fpdf.New("P", "pt", "", "")
	out.SetAutoPageBreak(false, 0)
	out.SetMargins(0, 0, 0)

	for i := 0; i < pageCount; i++ {
		page := requests.Page{ByIndex: &requests.PageByIndex{Document: doc.Document, Index: i}}
		sz, err := inst.GetPageSize(&requests.GetPageSize{Page: page})
		if err != nil {
			return nil, 0, fmt.Errorf("pdfproc: page %d size: %w", i, err)
		}
		wPt, hPt := sz.Width, sz.Height

		rp, err := inst.RenderPageInDPI(&requests.RenderPageInDPI{
			DPI:        dpi,
			Page:       page,
			RenderForm: true,
			Document:   &doc.Document,
		})
		if err != nil {
			return nil, 0, fmt.Errorf("pdfproc: render page %d: %w", i, err)
		}

		img := rp.Result.Image
		scaleX := float64(rp.Result.Width) / wPt
		scaleY := float64(rp.Result.Height) / hPt
		for _, pl := range byPage[i] {
			compositePlacement(img, pl, scaleX, scaleY)
		}

		var buf bytes.Buffer
		encErr := png.Encode(&buf, img)
		rp.Cleanup() // release wasm-side resources for this page
		if encErr != nil {
			return nil, 0, fmt.Errorf("pdfproc: encode page %d: %w", i, encErr)
		}

		name := fmt.Sprintf("page-%d", i)
		out.RegisterImageOptionsReader(name, fpdf.ImageOptions{ImageType: "PNG"}, &buf)
		out.AddPageFormat("P", fpdf.SizeType{Wd: wPt, Ht: hPt})
		out.ImageOptions(name, 0, 0, wPt, hPt, false, fpdf.ImageOptions{ImageType: "PNG"}, 0, "")
		if out.Err() {
			return nil, 0, fmt.Errorf("pdfproc: assemble page %d: %w", i, out.Error())
		}
	}

	var result bytes.Buffer
	if err := out.Output(&result); err != nil {
		return nil, 0, fmt.Errorf("pdfproc: output: %w", err)
	}
	return result.Bytes(), pageCount, nil
}

// compositePlacement draws pl.Image onto dst, applying scale (points->pixels), the
// requested rotation about the placement center, and alpha compositing. The affine matrix
// maps source-image pixels directly to destination pixels in a single high-quality pass.
func compositePlacement(dst *image.RGBA, pl Placement, scaleX, scaleY float64) {
	b := pl.Image.Bounds()
	sw, sh := float64(b.Dx()), float64(b.Dy())
	if sw == 0 || sh == 0 || pl.W <= 0 || pl.H <= 0 {
		return
	}

	sx := (pl.W * scaleX) / sw // src-pixel -> dst-pixel scale in X
	sy := (pl.H * scaleY) / sh
	cx := (pl.X + pl.W/2) * scaleX // placement center in dst pixels
	cy := (pl.Y + pl.H/2) * scaleY
	theta := pl.RotationDeg * math.Pi / 180
	cos, sin := math.Cos(theta), math.Sin(theta)

	// dst = R(theta) * S(sx,sy) * (src - srcCenter) + center
	m := f64.Aff3{
		cos * sx, -sin * sy, cx - cos*sx*(sw/2) + sin*sy*(sh/2),
		sin * sx, cos * sy, cy - sin*sx*(sw/2) - cos*sy*(sh/2),
	}
	draw.CatmullRom.Transform(dst, m, pl.Image, b, draw.Over, nil)
}
