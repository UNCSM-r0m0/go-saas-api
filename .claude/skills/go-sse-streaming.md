# Server-Sent Events (SSE) Streaming Skill

## Overview
Implementing real-time streaming with Server-Sent Events in Go for AI chat applications.

## Why SSE?

- **Unidirectional**: Server → Client (perfect for AI streaming)
- **HTTP-based**: Works through firewalls and proxies
- **Automatic reconnection**: Browser handles reconnects
- **Simple protocol**: Text-based, easy to debug

## Basic SSE Handler

```go
func StreamHandler(c *gin.Context) {
    // Set SSE headers
    c.Header("Content-Type", "text/event-stream")
    c.Header("Cache-Control", "no-cache")
    c.Header("Connection", "keep-alive")
    c.Header("Access-Control-Allow-Origin", "*")
    
    // Create channel for data
    dataChan := make(chan string)
    
    // Start goroutine to generate data
    go func() {
        defer close(dataChan)
        for i := 0; i < 10; i++ {
            dataChan <- fmt.Sprintf("Message %d", i)
            time.Sleep(100 * time.Millisecond)
        }
    }()
    
    // Stream data
    c.Stream(func(w io.Writer) bool {
        msg, ok := <-dataChan
        if !ok {
            return false
        }
        
        // SSE format: "data: <message>\n\n"
        fmt.Fprintf(w, "data: %s\n\n", msg)
        return true
    })
}
```

## SSE Format Specification

```
data: {"message": "Hello"}

event: custom-event
data: {"type": "update"}

id: 123
data: {"chunk": "partial content"}

: This is a comment (ignored by client)

data: Multi-line
data: message content

retry: 5000
```

## Integration with Ollama

```go
func ChatStreamHandler(c *gin.Context) {
    var req ChatRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(400, gin.H{"error": err.Error()})
        return
    }
    
    // Set SSE headers
    c.Header("Content-Type", "text/event-stream")
    c.Header("Cache-Control", "no-cache")
    c.Header("Connection", "keep-alive")
    
    // Request to Ollama
    payload := map[string]interface{}{
        "model":    req.Model,
        "messages": req.Messages,
        "stream":   true,
    }
    
    jsonData, _ := json.Marshal(payload)
    httpReq, _ := http.NewRequest("POST", ollamaURL+"/api/chat", bytes.NewBuffer(jsonData))
    httpReq.Header.Set("Content-Type", "application/json")
    
    resp, err := http.DefaultClient.Do(httpReq)
    if err != nil {
        c.SSEvent("error", `{"error": "failed to connect"}`)
        return
    }
    defer resp.Body.Close()
    
    // Stream Ollama response to client
    reader := bufio.NewReader(resp.Body)
    messageID := uuid.New().String()
    
    c.Stream(func(w io.Writer) bool {
        line, err := reader.ReadString('\n')
        if err != nil {
            return false
        }
        
        var ollamaResp struct {
            Message struct {
                Content string `json:"content"`
            } `json:"message"`
            Done bool `json:"done"`
        }
        
        if err := json.Unmarshal([]byte(line), &ollamaResp); err != nil {
            return true
        }
        
        if ollamaResp.Done {
            return false
        }
        
        // Format as OpenAI-compatible SSE
        chunk := map[string]interface{}{
            "id":      messageID,
            "object":  "chat.completion.chunk",
            "created": time.Now().Unix(),
            "model":   req.Model,
            "choices": []map[string]interface{}{
                {
                    "index": 0,
                    "delta": map[string]string{
                        "content": ollamaResp.Message.Content,
                    },
                    "finish_reason": nil,
                },
            },
        }
        
        data, _ := json.Marshal(chunk)
        fmt.Fprintf(w, "data: %s\n\n", string(data))
        return true
    })
}
```

## Client-Side JavaScript

```javascript
const eventSource = new EventSource('/api/v1/chat/stream', {
    method: 'POST',
    headers: {
        'Content-Type': 'application/json',
        'Authorization': 'Bearer ' + token
    },
    body: JSON.stringify({
        model: 'gpt-4',
        messages: [{role: 'user', content: 'Hello'}]
    })
});

eventSource.onmessage = (event) => {
    const data = JSON.parse(event.data);
    console.log('Received:', data.choices[0].delta.content);
};

eventSource.onerror = (error) => {
    console.error('SSE Error:', error);
    eventSource.close();
};

eventSource.onopen = () => {
    console.log('Connection opened');
};
```

## React Hook for SSE

```typescript
import { useState, useEffect, useCallback } from 'react';

export function useSSE<T>(url: string) {
    const [data, setData] = useState<T[]>([]);
    const [isConnected, setIsConnected] = useState(false);
    const [error, setError] = useState<Error | null>(null);
    
    const connect = useCallback((body: object) => {
        const eventSource = new EventSource(url, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(body)
        });
        
        eventSource.onopen = () => setIsConnected(true);
        
        eventSource.onmessage = (event) => {
            const parsed = JSON.parse(event.data);
            setData(prev => [...prev, parsed]);
        };
        
        eventSource.onerror = (err) => {
            setError(err as Error);
            setIsConnected(false);
        };
        
        return () => eventSource.close();
    }, [url]);
    
    return { data, isConnected, error, connect };
}
```

## Error Handling

```go
func SSEErrorHandler(c *gin.Context, err error) {
    // Send error as SSE event
    errorData := map[string]string{
        "error": err.Error(),
    }
    data, _ := json.Marshal(errorData)
    
    c.SSEvent("error", string(data))
}

// Usage in handler
c.Stream(func(w io.Writer) bool {
    result, err := processChunk()
    if err != nil {
        fmt.Fprintf(w, "event: error\n")
        fmt.Fprintf(w, "data: %s\n\n", `{"error": "`+err.Error()+`"}`)
        return false
    }
    
    fmt.Fprintf(w, "data: %s\n\n", result)
    return true
})
```

## Connection Management

### Keep-Alive
```go
func KeepAliveMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        // Prevent timeouts
        c.Writer.Header().Set("X-Accel-Buffering", "no")
        
        // Send periodic pings
        go func() {
            ticker := time.NewTicker(30 * time.Second)
            defer ticker.Stop()
            
            for {
                select {
                case <-ticker.C:
                    c.SSEvent("ping", "")
                case <-c.Request.Context().Done():
                    return
                }
            }
        }()
        
        c.Next()
    }
}
```

## Testing SSE

```go
func TestSSEHandler(t *testing.T) {
    gin.SetMode(gin.TestMode)
    
    r := gin.New()
    r.GET("/stream", StreamHandler)
    
    req := httptest.NewRequest("GET", "/stream", nil)
    w := httptest.NewRecorder()
    
    r.ServeHTTP(w, req)
    
    // Check headers
    assert.Equal(t, "text/event-stream", w.Header().Get("Content-Type"))
    
    // Check body format
    body := w.Body.String()
    assert.Contains(t, body, "data:")
}
```

## Best Practices

1. **Always set proper headers**: Content-Type, Cache-Control, Connection
2. **Handle client disconnects**: Check Request.Context().Done()
3. **Buffer management**: Use bufio.Reader for efficient reading
4. **Error events**: Send errors as SSE events, not HTTP errors
5. **Reconnection**: Set retry timeout for clients
6. **Rate limiting**: Prevent abuse of streaming endpoints
7. **Timeouts**: Set reasonable timeouts for long-running streams

## Common Pitfalls

- ❌ Not setting proper headers
- ❌ Forgetting to flush buffer
- ❌ Not handling client disconnects
- ❌ Sending binary data (SSE is text-only)
- ❌ Ignoring backpressure from slow clients
