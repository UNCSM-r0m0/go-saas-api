import logging
import io

try:
    from docx import Document
    HAS_DOCX = True
except ImportError:
    HAS_DOCX = False

logger = logging.getLogger(__name__)

def extract_docx(file_obj: io.BytesIO) -> str:
    """Extract text from a DOCX file.
    
    Extracts paragraphs and tables with structure preserved.
    """
    if not HAS_DOCX:
        raise Exception("python-docx not installed")
    
    try:
        doc = Document(file_obj)
        text_parts = []
        
        # Extract paragraphs
        for para in doc.paragraphs:
            if para.text.strip():
                text_parts.append(para.text)
        
        # Extract tables
        for i, table in enumerate(doc.tables):
            text_parts.append(f"\n--- Table {i + 1} ---")
            for row in table.rows:
                row_text = [cell.text.strip() for cell in row.cells if cell.text.strip()]
                if row_text:
                    text_parts.append(" | ".join(row_text))
        
        return "\n".join(text_parts)
    except Exception as e:
        logger.error(f"DOCX extraction failed: {e}")
        raise Exception(f"DOCX extraction failed: {e}")