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

    print("DEBUG: Loaded image:", img.shape)

    try:
        result = ocr.ocr(img, cls=True)

        # Debug print
        print("\nRAW OCR RESULT:", json.dumps(result, indent=2, default=str))

        if not result or not result[0]:
            return jsonify({"text": ""}), 200

        # Extract ALL text from the blocks
        extracted = []
        for block in result[0]:
            if isinstance(block, (list, tuple)) and len(block) >= 2:
                text_block = block[1]
                if isinstance(text_block, (list, tuple)) and len(text_block) >= 1:
                    extracted.append(str(text_block[0]))

        final_text = "\n".join(extracted)
        return jsonify({"text": final_text}), 200

    except Exception as e:
        print("OCR ERROR:", e)
        return jsonify({"error": str(e)}), 500


if __name__ == "__main__":
    app.run(host="0.0.0.0", port=8866)
