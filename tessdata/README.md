# Tesseract MRZ recognition data

`mrz.traineddata` is the integer/fast Tesseract MRZ language from
https://github.com/DoubangoTelecom/tesseractMRZ.

Source: https://github.com/DoubangoTelecom/tesseractMRZ/blob/0f039b6d18a4a0e7adb0b4841a8732e200b8d968/tessdata_fast/mrz.traineddata

SHA-256: `ece2a54f125a73792f9cec74e6f8e62f6e5e00c7f19153ba865af689be5b5f01`

The upstream BSD 3-Clause license is retained in LICENSE. The file is about
1.4 MB and runs locally through the existing Tesseract library. It is not an
orientation classifier and adds no extra OCR passes or runtime downloads.

When deploying a compiled binary, place this tessdata directory beside it, or
set MRZ_TESSDATA_PREFIX to its location. MRZ_TESSERACT_LANG defaults to mrz;
set it to eng to use installed English trained data instead.
