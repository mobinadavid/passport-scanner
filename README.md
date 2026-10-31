# Raw Tesseract passport OCR

The server passes the uploaded image bytes directly to Tesseract once, then
parses the recognized MRZ and validates its ICAO check digits. No cropping,
image preprocessing, or OCR retries are performed.

Requires Go 1.22+, a C++ compiler, and Tesseract/Leptonica development libraries
with English trained data installed. On Debian/Ubuntu:

```sh
sudo apt-get install g++ libtesseract-dev libleptonica-dev tesseract-ocr-eng
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
unrecognized or structurally invalid MRZ, and 502 for OCR errors. Uploads are limited to 20 MiB including
multipart overhead.

Dates use `YYYY-MM-DD`. MRZ years contain two digits: birth years that would
fall after the current year are interpreted as 1900s; expiry years use 2000s.
Checksum validity does not establish document authenticity or date validity.

Optional environment settings:

- `MRZ_TESSDATA_PREFIX`: custom Tesseract trained-data directory.
- `MRZ_TESSERACT_LANG`: installed recognition language (Tesseract defaults to `eng`).

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
