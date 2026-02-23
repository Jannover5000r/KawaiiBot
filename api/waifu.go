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
	waifuBaseURL = "https://api.waifu.im/images"
)

// WaifuClient represents the Waifu.im API client
type WaifuClient struct {
	httpClient *http.Client
	userAgent  string
}

type NSFWMode int

const (
	NSFWModeSFW NSFWMode = iota
	NSFWModeNSFW
	NSFWModeAll
)

type WaifuResponse struct {
	Items           []WaifuImage `json:"items"`
	PageNumber      int          `json:"pageNumber"`
	TotalPages      int          `json:"totalPages"`
	TotalCount      int          `json:"totalCount"`
	MaxPageSize     int          `json:"maxPageSize"`
	DefaultPageSize int          `json:"defaultPageSize"`
	HasPreviousPage bool         `json:"hasPreviousPage"`
	HasNextPage     bool         `json:"hasNextPage"`
}
type Artist struct {
	ID           int     `json:"id"`
	Name         string  `json:"name"`
	Patreon      *string `json:"patreon"`
	Pixiv        *string `json:"pixiv"`
	Twitter      *string `json:"twitter"`
	DeviantArt   *string `json:"deviantArt"`
	ReviewStatus string  `json:"reviewStatus"`
	CreatorID    *int    `json:"creatorId"`
	ImageCount   int     `json:"imageCount"`
}
type WaifuImage struct {
	ID             int64    `json:"id"`
	PerceptualHash string   `json:"perceptualHash"`
	Extension      string   `json:"extension"`
	DominantColor  string   `json:"dominantColor"`
	Source         string   `json:"source"`
	Artists        []Artist `json:"artists"`
	UploaderID     *int64   `json:"uploaderId"`
	UploadedAt     string   `json:"uploadedAt"`
	IsNSFW         bool     `json:"isNsfw"`
	IsAnimated     bool     `json:"isAnimated"`
	Width          int      `json:"width"`
	Height         int      `json:"height"`
	ByteSize       int      `json:"byteSize"`
	URL            string   `json:"url"`
	Tags           []Tag    `json:"tags"`
	ReviewStatus   string   `json:"reviewStatus"`
	Favorites      int      `json:"favorites"`
	LikedAt        *string  `json:"likedAt"`
	AddedToAlbumAt *string  `json:"addedToAlbumAt"`
	Albums         []string `json:"albums"`
}

type Tag struct {
	TagID           int    `json:"id"` // was tag_id
	Name            string `json:"name"`
	Slug            string `json:"slug"` // new
	Description     string `json:"description"`
	ReviewStatusTag string `json:"reviewStatus"` // was review_status (verify if present)
	CreatorID       *int64 `json:"creatorId"`    // nullable
	ImageCount      int    `json:"imageCount"`   // new
	IsNSFW          bool   `json:"is_nsfw"`      // verify if this still exists in tags
}

func (m NSFWMode) String() string {
	switch m {
	case NSFWModeSFW:
		return "False"
	case NSFWModeNSFW:
		return "True"
	case NSFWModeAll:
		return "All"
	default:
		return "False"
	}
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
func (c *WaifuClient) GetWaifuImages(mode NSFWMode, count int) ([]WaifuImage, error) {
	if count < 1 {
		count = 1
	}
	if count > 10 {
		count = 10
	}

	params := fmt.Sprintf("?IsNsfw=%s&pageSize=%d", mode.String(), count)

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

	return result.Items, nil
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

