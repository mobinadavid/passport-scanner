package main

/*
#cgo pkg-config: lept
#include <stdlib.h>
#include <leptonica/allheaders.h>

// The same blackhat/threshold/close operation is run with wide and tall
// kernels. Leptonica is already a dependency of Tesseract.
static BOXA *mrzBands(PIX *gray, int vertical) {
    int w = pixGetWidth(gray), h = pixGetHeight(gray);
    int length = vertical ? h : w, across = vertical ? w : h;
    int black = L_MAX(9, (int)(length * .025)) | 1;
    int close = L_MAX(11, (int)(length * .035)) | 1;
    int merge = L_MAX(9, (int)(across * .05)) | 1;
    PIX *hat = NULL, *inv = NULL, *binary = NULL, *joined = NULL, *band = NULL;
    BOXA *boxes = NULL;
    hat = pixTophat(gray, vertical ? 3 : black, vertical ? black : 3, L_TOPHAT_BLACK);
    if (!hat) goto cleanup;
    inv = pixInvert(NULL, hat);
    if (!inv) goto cleanup;
    if (pixOtsuAdaptiveThreshold(inv, w, h, 0, 0, 0.0, NULL, &binary)) goto cleanup;
    joined = pixCloseBrick(NULL, binary, vertical ? 3 : close, vertical ? close : 3);
    if (!joined) goto cleanup;
    band = pixDilateBrick(NULL, joined, vertical ? merge : 3, vertical ? 3 : merge);
    if (!band) goto cleanup;
    boxes = pixConnCompBB(band, 8);
cleanup:
    pixDestroy(&hat); pixDestroy(&inv); pixDestroy(&binary);
    pixDestroy(&joined); pixDestroy(&band);
    return boxes;
}
*/
import "C"

import (
	"errors"
	"fmt"
	"image"
	"math"
	"unsafe"
)

var errMRZNotFound = errors.New("no clear MRZ block near a page edge; upload a close-up of the passport data page")

type mrzLocation struct {
	Bounds image.Rectangle
	Edge   string
	Rotate int // clockwise degrees, chosen from the original image edge
}

// edgeRotation uses the detected band's axis: only top/bottom are meaningful
// for horizontal text, and left/right for vertical text. Central bands are
// ambiguous and rejected instead of arbitrarily turning them upside down.
func edgeRotation(box image.Rectangle, width, height int, vertical bool) (string, int, bool) {
	before, after, size := box.Min.Y, height-box.Max.Y, height
	first, last, firstAngle, lastAngle := "top", "bottom", 180, 0
	if vertical {
		before, after, size = box.Min.X, width-box.Max.X, width
		first, last, firstAngle, lastAngle = "left", "right", 270, 90
	}
	if min(before, after) > size*3/10 || abs(before-after) < size/10 {
		return "", 0, false
	}
	if before < after {
		return first, firstAngle, true
	}
	return last, lastAngle, true
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// prepareMRZ detects on a <=600px preview, then crops/rotates original pixels.
// There is no OSD, learned model, or OCR inside this function.
func prepareMRZ(data []byte) ([]byte, mrzLocation, error) {
	var location mrzLocation
	if len(data) == 0 {
		return nil, location, fmt.Errorf("empty image")
	}
	encoded := C.CBytes(data)
	defer C.free(encoded)
	src := C.pixReadMem((*C.l_uint8)(encoded), C.size_t(len(data)))
	if src == nil {
		return nil, location, fmt.Errorf("could not decode image")
	}
	defer C.pixDestroy(&src)
	width, height := int(C.pixGetWidth(src)), int(C.pixGetHeight(src))
	scale := math.Min(1, 600/float64(max(width, height)))
	preview := C.pixScaleToSize(src, C.l_int32(math.Round(float64(width)*scale)), C.l_int32(math.Round(float64(height)*scale)))
	if preview == nil {
		return nil, location, fmt.Errorf("could not resize image")
	}
	defer C.pixDestroy(&preview)
	gray := C.pixConvertTo8(preview, 0)
	if gray == nil {
		return nil, location, fmt.Errorf("could not convert image to grayscale")
	}
	defer C.pixDestroy(&gray)
	w, h := int(C.pixGetWidth(gray)), int(C.pixGetHeight(gray))
	best := -1.0
	for _, vertical := range []bool{false, true} {
		axis := C.int(0)
		if vertical {
			axis = 1
		}
		boxes := C.mrzBands(gray, axis)
		if boxes == nil {
			return nil, location, fmt.Errorf("MRZ image processing failed")
		}
		for i := C.l_int32(0); i < C.boxaGetCount(boxes); i++ {
			var x, y, bw, bh C.l_int32
			if C.boxaGetBoxGeometry(boxes, i, &x, &y, &bw, &bh) != 0 {
				continue
			}
			length, thickness, longSide := int(bw), int(bh), w
			if vertical {
				length, thickness, longSide = int(bh), int(bw), h
			}
			ratio := float64(length) / float64(max(1, thickness))
			if float64(length) < float64(longSide)*.45 || ratio < 5 || ratio > 20 {
				continue
			}
			box := image.Rect(int(x), int(y), int(x+bw), int(y+bh))
			edge, angle, ok := edgeRotation(box, w, h, vertical)
			if !ok {
				continue
			}
			score := float64(length) / float64(longSide)
			if score <= best {
				continue
			}
			best = score
			// A margin protects the outer glyphs and provides whitespace to OCR.
			pad := max(3, int(float64(thickness)*.15))
			sx, sy := float64(width)/float64(w), float64(height)/float64(h)
			location = mrzLocation{Bounds: image.Rect(
				max(0, int(math.Floor(float64(int(x)-pad)*sx))),
				max(0, int(math.Floor(float64(int(y)-pad)*sy))),
				min(width, int(math.Ceil(float64(int(x+bw)+pad)*sx))),
				min(height, int(math.Ceil(float64(int(y+bh)+pad)*sy)))), Edge: edge, Rotate: angle}
		}
		C.boxaDestroy(&boxes)
	}
	if best < 0 {
		return nil, location, errMRZNotFound
	}
	box := C.boxCreate(C.l_int32(location.Bounds.Min.X), C.l_int32(location.Bounds.Min.Y), C.l_int32(location.Bounds.Dx()), C.l_int32(location.Bounds.Dy()))
	if box == nil {
		return nil, location, fmt.Errorf("could not create MRZ crop")
	}
	defer C.boxDestroy(&box)
	crop := C.pixClipRectangle(src, box, nil)
	if crop == nil {
		return nil, location, fmt.Errorf("could not crop MRZ")
	}
	defer C.pixDestroy(&crop)
	rotated := C.pixRotateOrth(crop, C.l_int32(location.Rotate/90))
	if rotated == nil {
		return nil, location, fmt.Errorf("could not rotate MRZ")
	}
	defer C.pixDestroy(&rotated)
	var out *C.l_uint8
	var size C.size_t
	if C.pixWriteMemPng(&out, &size, rotated, 0) != 0 {
		if out != nil {
			C.lept_free(unsafe.Pointer(out))
		}
		return nil, location, fmt.Errorf("could not encode MRZ crop")
	}
	defer C.lept_free(unsafe.Pointer(out))
	if uint64(size) > 1<<31-1 {
		return nil, location, fmt.Errorf("MRZ crop is too large")
	}
	return C.GoBytes(unsafe.Pointer(out), C.int(size)), location, nil
}
