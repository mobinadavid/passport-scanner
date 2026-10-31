# Raw Tesseract passport OCR

The server finds the MRZ-shaped text block, uses its nearest page edge to choose
a rotation, crops and rotates that region, then runs Tesseract once. The OCR text
is parsed into passport fields and ICAO checksums. Orientation uses geometry,
without OSD or an orientation classifier. Recognition uses the bundled MRZ
Tesseract data (about 1.4 MB; see `tessdata/README.md` for source and license).

Keep using the same `/mrz` upload endpoint. This runs in the Go service using
Leptonica, which is already required by Tesseract; no Python or OpenCV is needed.

Requires Go 1.22+, a C++ compiler, and Tesseract/Leptonica development libraries
installed. On Debian/Ubuntu:

```sh
sudo apt-get install g++ libtesseract-dev libleptonica-dev
go run .
```

Send a multipart upload to port **8082**:

```sh
curl http://localhost:8082/mrz \
  -F image=@passport.jpg \
  -F requestId=109394 \
  -F jobId=109395
```

`requestId` and `jobId` are optional positive integers. If omitted, the server
generates process-local IDs from a timestamp-seeded counter. There is no job
queue or persistence: OCR and parsing finish within the request.

The response contains `requestId`, `jobId`, `status`, `passportResult`,
`generalResult`, and `error`. On completion, `status` is `2`, `generalResult`
and `error` are `null`, and `passportResult` contains:

- `firstName`, `lastName`, `fullNameLatin`, `passportNumber`, `birthDate`,
  `expiryDate`, `gender`, and `nationality`.
- `mrzRawText`: the MRZ lines selected and normalized by the parser, joined
  with a newline (not all text recognized elsewhere in the image).
- `passportNumberChecksum`, `birthDateChecksum`, `expiryDateChecksum`,
  `personalNumberChecksum`, and `compositeChecksum`: each has numeric `expected`
  (printed digit), numeric `calculated`, and boolean `isValid`.
- `allChecksumsValid`: whether all available check digits match.

Failed checksums are returned with the parsed passport and `status: 2`; callers
must check `allChecksumsValid` before trusting the values. An unused personal
number with a filler check digit is represented as zero. A nonnumeric printed
check digit is `null`. TD1/TD2 documents have no separate personal number check,
so that checksum object is `null`.

Failures use the same envelope with `status: 3`, `passportResult: null`,
`generalResult: null`, and a string `error`. HTTP statuses are 400 for invalid
uploads or IDs, 405 for unsupported methods, 422 for empty OCR text or an
unlocated, ambiguous, unrecognized or structurally invalid MRZ, and 502 for OCR errors. Uploads are limited to 20 MiB including
multipart overhead.

Dates use `YYYY-MM-DD`. MRZ years contain two digits: birth years that would
fall after the current year are interpreted as 1900s; expiry years use 2000s.
Checksum validity does not establish document authenticity or date validity.

Optional environment settings:

- `MRZ_TESSDATA_PREFIX`: custom Tesseract trained-data directory. By default,
  the service finds `tessdata` beside the executable or in the working directory.
- `MRZ_TESSERACT_LANG`: recognition language, default `mrz`. Set `eng` to use
  separately installed English data instead.
- `MRZ_DEBUG=true`: print candidate line bounds and geometry.

When deploying a compiled binary, copy the `tessdata` directory beside it or
set `MRZ_TESSDATA_PREFIX`. No runtime download is performed.

Run checks with `go test ./...`.

OCR uses a single PSM 6 (text block) pass without a character whitelist. This
preserves misrecognized punctuation such as `$`, which the parser normalizes
to `S`, rather than losing a character and shifting the remaining fields.
Given names stop at double-filler padding, so trailing OCR artifacts do not
become additional names. Invalid dates, nationality, or gender produce a parse
error instead of a completed passport with shifted fields. The parser does not
infer a missing passport character from a checksum.

Each request prints the complete, unmodified OCR text to the terminal using
`fmt.Printf`, with its request/job IDs, before normalization or parsing.

To run the real-image regression against the supplied FATEMEH IRANI image:

```sh
MRZ_TEST_IMAGE_PATH=/path/to/passport.png go test -run TestRecognizePassportImage -v .
```

## MRZ location and orientation

The service downscales a preview to at most 600 pixels on the long side, converts
it to grayscale, and runs blackhat morphology, Otsu thresholding, closing, and
connected-component analysis with both wide and tall kernels. Individual lines
must have aspect ratio 8–100 and cover at least 30% of their image axis. It pairs
lines with similar lengths, strong overlap, and close spacing, then selects the
longest pair near an edge. Keeping lines separate reduces merging with nearby
page text, which previously caused valid MRZ blocks to be rejected.
The candidate is a geometric guess; the parser and checksums validate the OCR.

For horizontal bands, it compares top and bottom margins; for vertical bands,
it compares left and right margins. The correction is:

- Bottom: no rotation.
- Top: 180 degrees.
- Left: 90 degrees counterclockwise.
- Right: 90 degrees clockwise.

Only the detection preview is downscaled. OCR receives the original-resolution
crop. After the quarter-turn rotation, a confident pixel-based skew estimate
corrects small tilts up to seven degrees. Background normalization reduces
uneven illumination before encoding the grayscale crop as PNG. The terminal
prints the detected edge, rotation, crop bounds, deskew angle/confidence,
preparation time, and raw OCR text. Tesseract still runs exactly once.

This assumes a close-up of an approximately axis-aligned passport data page with
the MRZ along its bottom edge. It is not a general direction detector. Wide scene
photos, both booklet pages, severe perspective, arbitrary tilt, and other long
text bands can cause detection errors. Central candidates or candidates more
than 30% of the cross-axis size from an edge are rejected with HTTP 422; the
service does not guess an orientation or try additional OCR passes.

The parser tolerates collapsed trailing padding on recognizable passport name
lines. It does not reconstruct missing characters in the checksummed data line.

To check multiple local photographs, including names as well as checksums for
the supplied regression photos:

```sh
MRZ_TEST_PHOTO_PATHS=/path/to/photo1.jpg:/path/to/photo2.jpg go test -run TestPhotographedPassports -v .
```

Optionally set `MRZ_TEST_CROP_DIR` to an existing directory to save the prepared
crops for inspection. Passport photographs are not bundled with the project.

The original real-image regression can use either supplied screenshot as
`MRZ_TEST_IMAGE_PATH`. Detection tests also cover all four edges, blank/invalid
images, ambiguous central bands, and preservation of the cropped glyph pixels.
