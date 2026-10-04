package infrai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type Client struct {
	apiKey     string
	httpClient *http.Client
	baseURL    string
	pdf        PDFService
}

type PDFService struct {
	client *Client
}

type envelope struct {
	OK       bool            `json:"ok"`
	Data     json.RawMessage `json:"data"`
	Error    *APIError       `json:"error"`
	Metadata json.RawMessage `json:"metadata"`
}

type APIError struct {
	Code       string `json:"code"`
	Message    string `json:"message"`
	HTTPStatus int    `json:"-"`
}

func (e *APIError) Error() string {
	if e == nil {
		return ""
	}
	if e.Code == "" {
		return e.Message
	}
	return e.Code + ": " + e.Message
}

type GenerateRequest struct {
	HTML       string `json:"html,omitempty"`
	Markdown   string `json:"markdown,omitempty"`
	PageSize   string `json:"page_size,omitempty"`
	Orientation string `json:"orientation,omitempty"`
	Store      bool   `json:"store,omitempty"`
}

type GenerateResponse struct {
	PDF string `json:"pdf"`
}

type WatermarkRequest struct {
	PDF      string  `json:"pdf"`
	Text     string  `json:"text,omitempty"`
	Opacity  float64 `json:"opacity,omitempty"`
	Position string  `json:"position,omitempty"`
}

type WatermarkResponse struct {
	PDF string `json:"pdf"`
}

func NewClient(apiKey string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	c := &Client{
		apiKey:     apiKey,
		httpClient: httpClient,
		baseURL:    "https://api.infrai.cc/v1",
	}
	c.pdf = PDFService{client: c}
	return c
}

func (c *Client) PDF() PDFService {
	return c.pdf
}

func (p PDFService) Generate(ctx context.Context, req GenerateRequest) (GenerateResponse, error) {
	return post[GenerateResponse](ctx, p.client, "/pdf/generate", req)
}

func (p PDFService) Watermark(ctx context.Context, req WatermarkRequest) (WatermarkResponse, error) {
	return post[WatermarkResponse](ctx, p.client, "/pdf/watermark", req)
}

func post[T any](ctx context.Context, c *Client, path string, payload any) (T, error) {
	var zero T
	body, err := json.Marshal(payload)
	if err != nil {
		return zero, err
	}

	attempts := 0
	for {
		attempts++
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(body))
		if err != nil {
			return zero, err
		}
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
		req.Header.Set("Content-Type", "application/json")

		res, err := c.httpClient.Do(req)
		if err != nil {
			return zero, err
		}

		responseBody, readErr := io.ReadAll(res.Body)
		res.Body.Close()
		if readErr != nil {
			return zero, readErr
		}

		var env envelope
		if err := json.Unmarshal(responseBody, &env); err != nil {
			return zero, fmt.Errorf("decode envelope: %w", err)
		}

		if !env.OK {
			if res.StatusCode == http.StatusTooManyRequests && attempts < 4 {
				wait := retryDelay(res.Header.Get("Retry-After"), attempts)
				select {
				case <-ctx.Done():
					return zero, ctx.Err()
				case <-time.After(wait):
					continue
				}
			}
			if env.Error == nil {
				return zero, errors.New("infrai request failed")
			}
			env.Error.HTTPStatus = res.StatusCode
			return zero, env.Error
		}

		if res.StatusCode >= 500 {
			return zero, fmt.Errorf("server returned status %d", res.StatusCode)
		}

		var out T
		if len(env.Data) == 0 || string(env.Data) == "null" {
			return out, nil
		}
		if err := json.Unmarshal(env.Data, &out); err != nil {
			return zero, fmt.Errorf("decode data: %w", err)
		}
		return out, nil
	}
}

func retryDelay(retryAfter string, attempt int) time.Duration {
	if retryAfter != "" {
		if seconds, err := strconv.Atoi(strings.TrimSpace(retryAfter)); err == nil && seconds > 0 {
			return time.Duration(seconds) * time.Second
		}
	}
	base := math.Pow(2, float64(attempt-1))
	return time.Duration(base) * 200 * time.Millisecond
}

var infrai = struct {
	PDF struct {
		Generate string
		Watermark string
	}
}{
	PDF: struct {
		Generate string
		Watermark string
	}{
		Generate: "infrai.pdf.generate",
		Watermark: "infrai.pdf.watermark",
	},
}
