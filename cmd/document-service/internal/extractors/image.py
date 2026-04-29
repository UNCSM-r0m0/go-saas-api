import logging
import io

try:
    from PIL import Image
    import pytesseract
    HAS_OCR = True
except ImportError:
    HAS_OCR = False

logger = logging.getLogger(__name__)

def extract_image(file_obj: io.BytesIO) -> str:
    """Extract text from an image using OCR (Tesseract).
    
    Supports Spanish and English text.
    """
    if not HAS_OCR:
        raise Exception("pytesseract or Pillow not installed")
    
    try:
        image = Image.open(file_obj)
        
        # Convert to RGB if necessary (for formats like PNG with transparency)
        if image.mode in ('RGBA', 'P'):
            image = image.convert('RGB')
        
        # Try Spanish first, fallback to English
        text = pytesseract.image_to_string(image, lang='spa+eng')
        
        if not text.strip():
            logger.warning("OCR returned empty text")
            return "[No text detected in image]"
        
        return text.strip()
    except Exception as e:
        logger.error(f"Image OCR failed: {e}")
        raise Exception(f"Image OCR failed: {e}")