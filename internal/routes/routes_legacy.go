package routes

// Legacy-device support (fork: immich-kiosk-legacy).
//
// Old iPads (iOS 12, 1 GB of RAM) are killed by iOS when photos arrive as base64 data
// URIs inside htmx fragments: the multi-megabyte strings and their decoded bitmaps
// outlive the frame that showed them. In legacy mode the encoded image is held here for
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
	// legacyImageTTL must outlive Kiosk's own asset cache (duration + 1 min, default 5 min),
	// or a prefetched frame could point at an image that is already gone.
	legacyImageTTL      = 30 * time.Minute
	legacyImageMaxBytes = 200 << 20
)

type legacyImage struct {
	data    []byte
	mime    string
	expires time.Time
}

var legacyImages = struct {
	sync.Mutex
	byToken map[string]legacyImage
	order   []string // insertion order, oldest first
	size    int
}{byToken: map[string]legacyImage{}}

// storeLegacyImage keeps the encoded image for legacyImageTTL and returns its URL.
func storeLegacyImage(data []byte, mime string) (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	token := hex.EncodeToString(b)
	now := time.Now()

	legacyImages.Lock()
	defer legacyImages.Unlock()

	// drop expired entries, then the oldest ones while over budget
	for len(legacyImages.order) > 0 {
		oldest := legacyImages.order[0]
		img, ok := legacyImages.byToken[oldest]
		if ok && now.Before(img.expires) && legacyImages.size+len(data) <= legacyImageMaxBytes {
			break
		}
		if ok {
			legacyImages.size -= len(img.data)
			delete(legacyImages.byToken, oldest)
		}
		legacyImages.order = legacyImages.order[1:]
	}

	legacyImages.byToken[token] = legacyImage{data: data, mime: mime, expires: now.Add(legacyImageTTL)}
	legacyImages.order = append(legacyImages.order, token)
	legacyImages.size += len(data)

	return "/legacy/image/" + token, nil
}

// LegacyImage serves an image stored by storeLegacyImage. Tokens are 128-bit random,
// so the URL itself is the capability, as a data URI in the page would have been.
func LegacyImage(c *echo.Context) error {
	legacyImages.Lock()
	img, ok := legacyImages.byToken[c.Param("token")]
	legacyImages.Unlock()

	if !ok || time.Now().After(img.expires) {
		return c.NoContent(http.StatusNotFound)
	}

	// no-store: the frame holds the only reference, so WebKit can free it with the frame
	c.Response().Header().Set("Cache-Control", "no-store")
	return c.Blob(http.StatusOK, img.mime, img.data)
}

// imageToSrc returns what goes into an <img src>: a short URL in legacy mode, otherwise
// the usual base64 data URI.
func imageToSrc(img image.Image, mimeType string, cfg config.Config, requestID, deviceID string, action string, isPrefetch bool) (string, error) {
	if !cfg.Legacy {
		return imageToBase64(img, mimeType, cfg.Kiosk.DebugVerbose, requestID, deviceID, action, isPrefetch)
	}

	startTime := time.Now()
	data, mime, err := utils.EncodeImage(img, mimeType)
	if err != nil {
		return "", err
	}
	src, err := storeLegacyImage(data, mime)
	if err != nil {
		return "", err
	}
	logImageProcessing(cfg.Kiosk.DebugVerbose, requestID, deviceID, isPrefetch, action+" (legacy URL)", startTime)
	return src, nil
}
