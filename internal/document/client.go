package document

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"time"
)

// Client extracts text from documents via the document-service.
type Client struct {
	baseURL string
	client  *http.Client
}

// NewClient creates a new document service client.
func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		client:  &http.Client{Timeout: 30 * time.Second},
	}
}

// ExtractResponse is the response from the document service.
type ExtractResponse struct {
	Text   string `json:"text"`
	Pages  *int   `json:"pages,omitempty"`
	Sheets *int   `json:"sheets,omitempty"`
	Error  string `json:"error,omitempty"`
}

func (c *Client) extract(ctx context.Context, endpoint string, fileName string, contentType string, reader io.Reader) (*ExtractResponse, error) {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	
	part, err := writer.CreateFormFile("file", fileName)
	if err != nil {
		return nil, fmt.Errorf("create form file: %w", err)
	}
	
	if _, err := io.Copy(part, reader); err != nil {
		return nil, fmt.Errorf("copy file: %w", err)
	}
	
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("close writer: %w", err)
	}
	
	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+endpoint, &buf)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	
	req.Header.Set("Content-Type", writer.FormDataContentType())
	
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		var errResp struct {
			Detail string `json:"detail"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&errResp); err == nil && errResp.Detail != "" {
			return nil, fmt.Errorf("document service error (%d): %s", resp.StatusCode, errResp.Detail)
		}
		return nil, fmt.Errorf("document service returned %d", resp.StatusCode)
	}
	
	var result ExtractResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	
	return &result, nil
}

// ExtractPDF extracts text from a PDF file.
func (c *Client) ExtractPDF(ctx context.Context, fileName string, reader io.Reader) (*ExtractResponse, error) {
	return c.extract(ctx, "/extract/pdf", fileName, "application/pdf", reader)
}

// ExtractDOCX extracts text from a DOCX file.
func (c *Client) ExtractDOCX(ctx context.Context, fileName string, reader io.Reader) (*ExtractResponse, error) {
	return c.extract(ctx, "/extract/docx", fileName, "application/vnd.openxmlformats-officedocument.wordprocessingml.document", reader)
}

// ExtractXLSX extracts text from an XLSX file.
func (c *Client) ExtractXLSX(ctx context.Context, fileName string, reader io.Reader) (*ExtractResponse, error) {
	return c.extract(ctx, "/extract/xlsx", fileName, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", reader)
}

// ExtractImage extracts text from an image using OCR.
func (c *Client) ExtractImage(ctx context.Context, fileName string, contentType string, reader io.Reader) (*ExtractResponse, error) {
	return c.extract(ctx, "/extract/image", fileName, contentType, reader)
}

// Health checks if the document service is healthy.
func (c *Client) Health(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/health", nil)
	if err != nil {
		return err
	}
	
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("document service health check failed: %d", resp.StatusCode)
	}
	
	return nil
}