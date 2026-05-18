from fastapi import FastAPI, File, UploadFile, HTTPException
from pydantic import BaseModel
from typing import Optional
import io
import logging
import uuid

from internal.extractors.pdf import extract_pdf
from internal.extractors.docx import extract_docx
from internal.extractors.xlsx import extract_xlsx
from internal.extractors.image import extract_image

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

# In-memory store for extracted documents (MVP)
document_store = {}

app = FastAPI(
    title="Document Processing Service",
    description="Extract text from PDF, DOCX, XLSX, and images (OCR)",
    version="1.0.0"
)

class ExtractResponse(BaseModel):
    text: str
    pages: Optional[int] = None
    sheets: Optional[int] = None
    error: Optional[str] = None

class UploadResponse(BaseModel):
    document_id: str
    text: str
    pages: Optional[int] = None

class DocumentResponse(BaseModel):
    document_id: str
    text: str
    pages: Optional[int] = None

class HealthResponse(BaseModel):
    status: str
    version: str

@app.get("/health", response_model=HealthResponse)
async def health():
    return HealthResponse(status="healthy", version="1.0.0")

@app.post("/files/upload", response_model=UploadResponse)
async def upload_file(file: UploadFile = File(...)):
    if not file.content_type:
        raise HTTPException(status_code=400, detail="Could not determine file type")

    try:
        content = await file.read()
        content_stream = io.BytesIO(content)

        text = ""
        pages = None
        sheets = None

        if "pdf" in file.content_type:
            text, pages = extract_pdf(content_stream)
        elif "wordprocessingml" in file.content_type or "msword" in file.content_type:
            text = extract_docx(content_stream)
        elif "spreadsheetml" in file.content_type or "ms-excel" in file.content_type:
            text, sheets = extract_xlsx(content_stream)
        elif file.content_type.startswith("image/"):
            text = extract_image(content_stream)
        else:
            raise HTTPException(status_code=400, detail=f"Unsupported file type: {file.content_type}")

        document_id = str(uuid.uuid4())
        document_store[document_id] = {
            "document_id": document_id,
            "text": text,
            "pages": pages,
            "sheets": sheets,
            "filename": file.filename,
            "content_type": file.content_type,
        }

        return UploadResponse(document_id=document_id, text=text, pages=pages)
    except HTTPException:
        raise
    except Exception as e:
        logger.error(f"File upload/processing failed: {e}")
        raise HTTPException(status_code=422, detail=f"Processing failed: {str(e)}")

@app.get("/documents/{document_id}", response_model=DocumentResponse)
async def get_document(document_id: str):
    doc = document_store.get(document_id)
    if not doc:
        raise HTTPException(status_code=404, detail="Document not found")
    return DocumentResponse(
        document_id=doc["document_id"],
        text=doc["text"],
        pages=doc.get("pages"),
    )

# Legacy extract endpoints (kept for backward compatibility)
@app.post("/extract/pdf", response_model=ExtractResponse)
async def extract_pdf_endpoint(file: UploadFile = File(...)):
    if not file.content_type or "pdf" not in file.content_type:
        raise HTTPException(status_code=400, detail="File must be a PDF")
    
    try:
        content = await file.read()
        text, pages = extract_pdf(io.BytesIO(content))
        return ExtractResponse(text=text, pages=pages)
    except Exception as e:
        logger.error(f"PDF extraction failed: {e}")
        raise HTTPException(status_code=422, detail=f"PDF extraction failed: {str(e)}")

@app.post("/extract/docx", response_model=ExtractResponse)
async def extract_docx_endpoint(file: UploadFile = File(...)):
    if not file.content_type or "wordprocessingml" not in file.content_type:
        raise HTTPException(status_code=400, detail="File must be a DOCX")
    
    try:
        content = await file.read()
        text = extract_docx(io.BytesIO(content))
        return ExtractResponse(text=text)
    except Exception as e:
        logger.error(f"DOCX extraction failed: {e}")
        raise HTTPException(status_code=422, detail=f"DOCX extraction failed: {str(e)}")

@app.post("/extract/xlsx", response_model=ExtractResponse)
async def extract_xlsx_endpoint(file: UploadFile = File(...)):
    if not file.content_type or "spreadsheetml" not in file.content_type:
        raise HTTPException(status_code=400, detail="File must be an XLSX")
    
    try:
        content = await file.read()
        text, sheets = extract_xlsx(io.BytesIO(content))
        return ExtractResponse(text=text, sheets=sheets)
    except Exception as e:
        logger.error(f"XLSX extraction failed: {e}")
        raise HTTPException(status_code=422, detail=f"XLSX extraction failed: {str(e)}")

@app.post("/extract/image", response_model=ExtractResponse)
async def extract_image_endpoint(file: UploadFile = File(...)):
    allowed_types = {"image/png", "image/jpeg", "image/jpg", "image/webp", "image/gif"}
    if not file.content_type or file.content_type not in allowed_types:
        raise HTTPException(status_code=400, detail="File must be an image (PNG, JPEG, WebP, GIF)")
    
    try:
        content = await file.read()
        text = extract_image(io.BytesIO(content))
        return ExtractResponse(text=text)
    except Exception as e:
        logger.error(f"Image OCR failed: {e}")
        raise HTTPException(status_code=422, detail=f"Image OCR failed: {str(e)}")

if __name__ == "__main__":
    import uvicorn
    uvicorn.run(app, host="0.0.0.0", port=8000)
