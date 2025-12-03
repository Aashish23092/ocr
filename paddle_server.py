from flask import Flask, request, jsonify
from paddleocr import PaddleOCR
import numpy as np
import cv2
from PIL import Image
import io
import json

app = Flask(__name__)

ocr = PaddleOCR(
    use_angle_cls=True,
    lang='en',
    ocr_version='PP-OCRv3',
    show_log=True
)
# ocr = None

def load_image_safely(file_bytes):
    np_img = np.frombuffer(file_bytes, np.uint8)
    img = cv2.imdecode(np_img, cv2.IMREAD_COLOR)
    if img is not None and img.size > 0:
        return img

    try:
        pil = Image.open(io.BytesIO(file_bytes))
        pil = pil.convert("RGB")
        img = cv2.cvtColor(np.array(pil), cv2.COLOR_RGB2BGR)
        return img
    except:
        return None


@app.route("/ocr", methods=["POST"])
def ocr_route():
    if "image" not in request.files:
        return jsonify({"error": "image field missing"}), 400

    file_bytes = request.files["image"].read()
    if not file_bytes:
        return jsonify({"error": "empty file"}), 400

    img = load_image_safely(file_bytes)
    if img is None:
        return jsonify({"error": "failed to decode image"}), 400



    try:
        result = ocr.ocr(img, cls=True)

        # Determine if result is [block, block] or [[block, block]]
        blocks = []
        if isinstance(result[0], (list, tuple)) and len(result[0]) == 2 and \
           isinstance(result[0][0], (list, tuple)) and len(result[0][0]) == 4:
            blocks = result
        elif isinstance(result[0], (list, tuple)):
            blocks = result[0]
        
        if not blocks:
             return jsonify({"text": ""}), 200

        # Extract ALL text from the blocks
        extracted = []
        for block in blocks:
            if isinstance(block, (list, tuple)) and len(block) >= 2:
                text_block = block[1]
                if isinstance(text_block, (list, tuple)) and len(text_block) >= 1:
                    text = str(text_block[0])
                    extracted.append(text)

        final_text = "\n".join(extracted)
        return jsonify({"text": final_text}), 200

    except Exception as e:
        print("OCR ERROR:", e)
        return jsonify({"error": str(e)}), 500


if __name__ == "__main__":
    app.run(host="0.0.0.0", port=8866)
