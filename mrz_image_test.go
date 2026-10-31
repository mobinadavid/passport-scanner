package main

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"testing"
)

func rotatedPNG(t *testing.T, src image.Image, clockwise int) []byte {
	t.Helper()
	w, h := src.Bounds().Dx(), src.Bounds().Dy()
	dw, dh := w, h
	if clockwise == 90 || clockwise == 270 {
		dw, dh = h, w
	}
	dst := image.NewNRGBA(image.Rect(0, 0, dw, dh))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			dx, dy := x, y
			switch clockwise {
			case 90:
				dx, dy = h-1-y, x
			case 180:
				dx, dy = w-1-x, h-1-y
			case 270:
				dx, dy = y, w-1-x
			}
			dst.Set(dx, dy, src.At(x, y))
		}
	}
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, dst); err != nil {
		t.Fatal(err)
	}
	return encoded.Bytes()
}

func TestMRZLocationAtEveryEdge(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 600, 400))
	draw.Draw(src, src.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
	// Two rows of separated strokes represent the MRZ's dense text texture.
	for _, y := range []int{310, 340} {
		for i := 0; i < 44; i++ {
			draw.Draw(src, image.Rect(40+i*12, y, 46+i*12, y+12), image.NewUniform(color.Black), image.Point{}, draw.Src)
		}
	}
	for _, tc := range []struct {
		angle      int
		edge       string
		correction int
	}{{0, "bottom", 0}, {90, "left", 270}, {180, "top", 180}, {270, "right", 90}} {
		t.Run(tc.edge, func(t *testing.T) {
			crop, found, err := prepareMRZ(rotatedPNG(t, src, tc.angle))
			if err != nil {
				t.Fatal(err)
			}
			if found.Edge != tc.edge || found.Rotate != tc.correction {
				t.Fatalf("wrong orientation: %+v", found)
			}
			decoded, err := png.Decode(bytes.NewReader(crop))
			if err != nil {
				t.Fatal(err)
			}
			if decoded.Bounds().Dx() < 5*decoded.Bounds().Dy() {
				t.Fatalf("crop not a horizontal MRZ: %v", decoded.Bounds())
			}
			// All rotations must produce the same upright glyph coordinates.
			black := 0
			for y := decoded.Bounds().Min.Y; y < decoded.Bounds().Max.Y; y++ {
				for x := decoded.Bounds().Min.X; x < decoded.Bounds().Max.X; x++ {
					r, g, b, _ := decoded.At(x, y).RGBA()
					if r+g+b == 0 {
						black++
					}
				}
			}
			if black != 44*2*6*12 {
				t.Fatalf("MRZ crop lost pixels: %d", black)
			}
		})
	}
}

func TestMRZRejectsBlankAndInvalidImages(t *testing.T) {
	blank := image.NewNRGBA(image.Rect(0, 0, 600, 400))
	draw.Draw(blank, blank.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
	if _, _, err := prepareMRZ(rotatedPNG(t, blank, 0)); !errors.Is(err, errMRZNotFound) {
		t.Fatalf("blank image: %v", err)
	}
	for _, data := range [][]byte{nil, []byte("not an image")} {
		if _, _, err := prepareMRZ(data); err == nil {
			t.Fatal("accepted invalid image")
		}
	}
}

func TestMRZCentralBandIsAmbiguous(t *testing.T) {
	for _, vertical := range []bool{false, true} {
		box := image.Rect(50, 170, 550, 230)
		w, h := 600, 400
		if vertical {
			box = image.Rect(170, 50, 230, 550)
			w, h = 400, 600
		}
		if _, _, ok := edgeRotation(box, w, h, vertical); ok {
			t.Fatal("central MRZ should not imply a rotation")
		}
	}
}
