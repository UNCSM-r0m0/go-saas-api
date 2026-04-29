# Document Gateway Routes Specification

## Purpose

Expose document-service upload and retrieval through the API Gateway.

## Requirements

### Requirement: File Upload

The system MUST accept file uploads and proxy them to document-service.

#### Scenario: Successful upload

- GIVEN an authenticated request to `POST /api/v1/files/upload` with multipart file data
- WHEN the file is within size limits
- THEN it is proxied to `document-service:8000`
- AND returns `{document_id, url, size}`

#### Scenario: Upload too large

- GIVEN an authenticated upload request with file > `MaxUploadSize`
- WHEN the gateway processes it
- THEN it returns `413 Payload Too Large`
- AND the request is NOT forwarded

### Requirement: Document Retrieval

The system MUST proxy document download requests.

#### Scenario: Retrieve document

- GIVEN an authenticated request to `GET /api/v1/documents/{id}`
- WHEN the document exists
- THEN it is proxied to `document-service:8000`
- AND the file is returned

#### Scenario: Document not found

- GIVEN an authenticated request to `GET /api/v1/documents/{id}`
- WHEN the document does not exist
- THEN it returns `404 Not Found`
