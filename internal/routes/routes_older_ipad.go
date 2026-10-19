package routes

// Older-iPad support (fork: immich-kiosk-older-ipads).
//
// Old iPads (iOS 12, 1 GB of RAM) are killed by iOS when photos arrive as base64 data
// URIs inside htmx fragments: the multi-megabyte strings and their decoded bitmaps
// outlive the frame that showed them. In older-iPad mode the encoded image is held here for
// a short while and the page gets a small URL instead, which WebKit can load and free
// like any other image.

import (
	"crypto/rand"
	"encoding/hex"
	"image"
	"net/http"
	"sync"
	"time"

	"github.com/labstack/echo/v5"

	"github.com/damongolding/immich-kiosk/internal/config"
	"github.com/damongolding/immich-kiosk/internal/utils"
)

const (
	// olderIPadImageTTL must outlive Kiosk's own asset cache (duration + 1 min, default 5 min),
	// or a prefetched frame could point at an image that is already gone.
	olderIPadImageTTL      = 30 * time.Minute
	olderIPadImageMaxBytes = 200 << 20
)

type olderIPadImage struct {
	data    []byte
	mime    string
	expires time.Time
}

var olderIPadImages = struct {
	sync.Mutex
	byToken map[string]olderIPadImage
	order   []string // insertion order, oldest first
	size    int
}{byToken: map[string]olderIPadImage{}}

// storeOlderIPadImage keeps the encoded image for olderIPadImageTTL and returns its URL.
func storeOlderIPadImage(data []byte, mime string) (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	token := hex.EncodeToString(b)
	now := time.Now()

	olderIPadImages.Lock()
	defer olderIPadImages.Unlock()

	// drop expired entries, then the oldest ones while over budget
	for len(olderIPadImages.order) > 0 {
		oldest := olderIPadImages.order[0]
		img, ok := olderIPadImages.byToken[oldest]
		if ok && now.Before(img.expires) && olderIPadImages.size+len(data) <= olderIPadImageMaxBytes {
			break
		}
		if ok {
			olderIPadImages.size -= len(img.data)
			delete(olderIPadImages.byToken, oldest)
		}
		olderIPadImages.order = olderIPadImages.order[1:]
	}

	olderIPadImages.byToken[token] = olderIPadImage{data: data, mime: mime, expires: now.Add(olderIPadImageTTL)}
	olderIPadImages.order = append(olderIPadImages.order, token)
	olderIPadImages.size += len(data)

	return "/ipad/image/" + token, nil
}

// OlderIPadImage serves an image stored by storeOlderIPadImage. Tokens are 128-bit random,
// so the URL itself is the capability, as a data URI in the page would have been.
func OlderIPadImage(c *echo.Context) error {
	olderIPadImages.Lock()
	img, ok := olderIPadImages.byToken[c.Param("token")]
	olderIPadImages.Unlock()

	if !ok || time.Now().After(img.expires) {
		return c.NoContent(http.StatusNotFound)
	}

	// no-store: the frame holds the only reference, so WebKit can free it with the frame
	c.Response().Header().Set("Cache-Control", "no-store")
	return c.Blob(http.StatusOK, img.mime, img.data)
}

// imageToSrc returns what goes into an <img src>: a short URL in older-iPad mode, otherwise
// the usual base64 data URI.
func imageToSrc(img image.Image, mimeType string, cfg config.Config, requestID, deviceID string, action string, isPrefetch bool) (string, error) {
	if !cfg.OlderIPad {
		return imageToBase64(img, mimeType, cfg.Kiosk.DebugVerbose, requestID, deviceID, action, isPrefetch)
	}

	startTime := time.Now()
	data, mime, err := utils.EncodeImage(img, mimeType)
	if err != nil {
		return "", err
	}
	src, err := storeOlderIPadImage(data, mime)
	if err != nil {
		return "", err
	}
	logImageProcessing(cfg.Kiosk.DebugVerbose, requestID, deviceID, isPrefetch, action+" (older-iPad URL)", startTime)
	return src, nil
}
