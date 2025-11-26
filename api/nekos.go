// Package api: Used to call the nekos.moe api for pictures
package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	baseURL = "https://nekos.moe/api/v1/"
)

// Client represents the Nekos.moe API client
type Client struct {
	httpClient *http.Client
	userAgent  string
}

// Image represents an image from the API
type Image struct {
	ID        string   `json:"id"`
	Tags      []string `json:"tags"`
	Artist    string   `json:"artist"`
	NSFW      bool     `json:"nsfw"`
	Likes     int      `json:"likes"`
	Favorites int      `json:"favorites"`
	CreatedAt string   `json:"createdAt"`
	Uploader  struct {
		ID       string `json:"id"`
		Username string `json:"username"`
	} `json:"uploader"`
	Approver *struct {
		ID       string `json:"id"`
		Username string `json:"username"`
	} `json:"approver"`
	OriginalHash string `json:"originalHash"`
}

// RandomImageResponse represents the API response
type RandomImageResponse struct {
	Images []Image `json:"images"`
}

// New creates a new API client
func New(userAgent string) *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		userAgent: userAgent,
	}
}

// GetRandomImages fetches random images from the API
func (c *Client) GetRandomImages(count int, rating string) ([]Image, error) {
	// Use the correct endpoint: /images/random
	endpoint := fmt.Sprintf("random/image?count=%d", count)

	// For Nekos.moe, use nsfw=true/false instead of rating parameter
	// If rating is empty, omit the nsfw parameter to get mixed results
	if rating != "" {
		if rating == "explicit" {
			endpoint += "&nsfw=true"
		} else {
			endpoint += "&nsfw=false" // Default to SFW for "safe" or any other value
		}
	}

	// Nekos.moe random endpoint doesn't support tags, so we ignore them for random images

	req, err := http.NewRequest(http.MethodGet, baseURL+endpoint, nil)
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

	var result RandomImageResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return result.Images, nil
}

func (c *Client) DownloadImage(imageURL string) ([]byte, error) {
	// The API returns just the ID, we need to construct the full URL
	// Format: https://nekos.moe/image/{ID}.jpg
	fullURL := "https://nekos.moe/image/" + imageURL + ".jpg"

	req, err := http.NewRequest(http.MethodGet, fullURL, nil)
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

// GetImageByID gets a specific image by its ID
func (c *Client) GetImageByID(id string) (*Image, error) {
	endpoint := fmt.Sprintf("images/%s", id)

	req, err := http.NewRequest(http.MethodGet, baseURL+endpoint, nil)
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

	var image Image
	if err := json.NewDecoder(resp.Body).Decode(&image); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &image, nil
}

// SearchImages searches for images based on tags
func (c *Client) SearchImages(tags []string, count int, rating string) ([]Image, error) {
	endpoint := "images/search?"

	// Add tags
	for i, tag := range tags {
		if i > 0 {
			endpoint += "&"
		}
		endpoint += fmt.Sprintf("tags=%s", tag)
	}

	// Add count and rating
	endpoint += fmt.Sprintf("&count=%d", count)
	if rating != "" {
		endpoint += fmt.Sprintf("&rating=%s", rating)
	}

	req, err := http.NewRequest(http.MethodGet, baseURL+endpoint, nil)
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

	var result RandomImageResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return result.Images, nil
}
