// Package api: Used to call the waifu.im api for waifu pictures
package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	waifuBaseURL = "https://api.waifu.im/search?"
)

// WaifuClient represents the Waifu.im API client
type WaifuClient struct {
	httpClient *http.Client
	userAgent  string
}

// WaifuImage represents an image from the Waifu.im API
type WaifuImage struct {
	Signature     string `json:"signature"`
	Extension     string `json:"extension"`
	ImageID       int    `json:"image_id"`
	Favorites     int    `json:"favorites"`
	DominantColor string `json:"dominant_color"`
	Source        string `json:"source"`
	Artist        interface{} `json:"artist"` // Can be null or string
	UploadedAt    string `json:"uploaded_at"`
	LikedAt       interface{} `json:"liked_at"` // Can be null or string
	IsNSFW        bool   `json:"is_nsfw"`
	Width         int    `json:"width"`
	Height        int    `json:"height"`
	ByteSize      int    `json:"byte_size"`
	URL           string `json:"url"`
	PreviewURL    string `json:"preview_url"`
	Tags          []struct {
		TagID       int    `json:"tag_id"`
		Name        string `json:"name"`
		Description string `json:"description"`
		IsNSFW      bool   `json:"is_nsfw"`
	} `json:"tags"`
}

// WaifuResponse represents the API response from waifu.im
type WaifuResponse struct {
	Images []WaifuImage `json:"images"`
}

// NewWaifuClient creates a new Waifu.im API client
func NewWaifuClient(userAgent string) *WaifuClient {
	return &WaifuClient{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		userAgent: userAgent,
	}
}

// GetWaifuImages fetches waifu images from the API
func (c *WaifuClient) GetWaifuImages(count int, isNSFW bool, isGIF bool) ([]WaifuImage, error) {
	// Build query parameters
	params := fmt.Sprintf("is_nsfw=%t", isNSFW)
	
	// Only add limit if count > 1 (API requirement)
	if count > 1 {
		params += fmt.Sprintf("&limit=%d", count)
	}
	
	// Add GIF filter if requested
	if isGIF {
		params += "&gif=true"
	}

	req, err := http.NewRequest(http.MethodGet, waifuBaseURL+params, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", c.userAgent)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	var result WaifuResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return result.Images, nil
}

// DownloadWaifuImage downloads a waifu image from the provided URL
func (c *WaifuClient) DownloadWaifuImage(imageURL string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, imageURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", c.userAgent)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to download image: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to download image: status %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read image data: %w", err)
	}

	return data, nil
}