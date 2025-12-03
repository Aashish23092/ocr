import requests
import sys

def reproduce():
    url = "http://localhost:8866/ocr"
    # Create a dummy image or use an existing one if available. 
    # Since I don't have a guaranteed image, I'll try to use a simple one or check if there are test files.
    # For now, I'll create a simple black image using PIL and send it.
    
    try:
        from PIL import Image
        import io
        
        img = Image.new('RGB', (100, 30), color = (73, 109, 137))
        img_byte_arr = io.BytesIO()
        img.save(img_byte_arr, format='PNG')
        img_byte_arr = img_byte_arr.getvalue()
        
        files = {'image': ('test.png', img_byte_arr, 'image/png')}
        
        print(f"Sending request to {url}...")
        response = requests.post(url, files=files)
        
        print(f"Status Code: {response.status_code}")
        print(f"Response Body: {response.text}")
        
    except Exception as e:
        print(f"Error: {e}")

if __name__ == "__main__":
    reproduce()
