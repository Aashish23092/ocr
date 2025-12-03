from paddleocr import PaddleOCR

# Initialize PaddleOCR to trigger model download
ocr = PaddleOCR(
    use_angle_cls=True,
    lang='en',
    ocr_version='PP-OCRv3',
    show_log=True
)
print("Models downloaded successfully.")
