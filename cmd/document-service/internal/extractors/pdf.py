import logging
from typing import Tuple, Optional
import io

try:
    import pdfplumber
    HAS_PDFPLUMBER = True
except ImportError:
    HAS_PDFPLUMBER = False

try:
    from PyPDF2 import PdfReader
    HAS_PYPDF2 = True
except ImportError:
    HAS_PYPDF2 = False

logger = logging.getLogger(__name__)

def extract_pdf(file_obj: io.BytesIO) -> Tuple[str, Optional[int]]:
    """Extract text from a PDF file.
    
    Returns:
        Tuple of (extracted_text, number_of_pages)
    """
    text_parts = []
    pages = 0
    
    # Prefer pdfplumber for better table/formatted text extraction
    if HAS_PDFPLUMBER:
        try:
            with pdfplumber.open(file_obj) as pdf:
                pages = len(pdf.pages)
                for i, page in enumerate(pdf.pages):
                    page_text = page.extract_text()
                    if page_text:
                        text_parts.append(f"--- Page {i + 1} ---\n{page_text}")
            return "\n\n".join(text_parts), pages
        except Exception as e:
            logger.warning(f"pdfplumber failed, falling back to PyPDF2: {e}")
            file_obj.seek(0)
    
    # Fallback to PyPDF2
    if HAS_PYPDF2:
        try:
            reader = PdfReader(file_obj)
            pages = len(reader.pages)
            for i, page in enumerate(reader.pages):
                page_text = page.extract_text()
                if page_text:
                    text_parts.append(f"--- Page {i + 1} ---\n{page_text}")
            return "\n\n".join(text_parts), pages
        except Exception as e:
            logger.error(f"PyPDF2 extraction failed: {e}")
            raise Exception(f"PDF extraction failed: {e}")
    
    raise Exception("No PDF extraction library available. Install pdfplumber or PyPDF2.")