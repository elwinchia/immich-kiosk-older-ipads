package routes

import (
	"image"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v5"

	"github.com/damongolding/immich-kiosk/internal/config"
)

func resetLegacyImages() {
	legacyImages.Lock()
	legacyImages.byToken = map[string]legacyImage{}
	legacyImages.order = nil
	legacyImages.size = 0
	legacyImages.Unlock()
}

func serveLegacy(t *testing.T, url string) *httptest.ResponseRecorder {
	t.Helper()
	e := echo.New()
	e.GET("/legacy/image/:token", LegacyImage)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, url, nil))
	return rec
}

func TestLegacyImageRoundTrip(t *testing.T) {
	resetLegacyImages()
	url, err := storeLegacyImage([]byte("jpeg-bytes"), "image/jpeg")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(url, "/legacy/image/") || len(url) != len("/legacy/image/")+32 {
		t.Fatalf("unexpected url %q", url)
	}

	rec := serveLegacy(t, url)
	if rec.Code != http.StatusOK || rec.Body.String() != "jpeg-bytes" {
		t.Fatalf("got %d %q", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("Content-Type") != "image/jpeg" || rec.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("headers %v", rec.Header())
	}

	if rec := serveLegacy(t, "/legacy/image/0123456789abcdef0123456789abcdef"); rec.Code != http.StatusNotFound {
		t.Fatalf("unknown token: got %d", rec.Code)
	}
}

func TestLegacyImageExpiresAndBudget(t *testing.T) {
	resetLegacyImages()
	url, _ := storeLegacyImage([]byte("old"), "image/jpeg")
	token := strings.TrimPrefix(url, "/legacy/image/")

	legacyImages.Lock()
	img := legacyImages.byToken[token]
	img.expires = time.Now().Add(-time.Second)
	legacyImages.byToken[token] = img
	legacyImages.Unlock()

	if rec := serveLegacy(t, url); rec.Code != http.StatusNotFound {
		t.Fatalf("expired: got %d", rec.Code)
	}

	// the next store prunes the expired entry
	_, _ = storeLegacyImage([]byte("new"), "image/jpeg")
	legacyImages.Lock()
	_, stillThere := legacyImages.byToken[token]
	n, size := len(legacyImages.byToken), legacyImages.size
	legacyImages.Unlock()
	if stillThere || n != 1 || size != 3 {
		t.Fatalf("after prune: stillThere=%v n=%d size=%d", stillThere, n, size)
	}

	// over budget: the oldest entries go first
	resetLegacyImages()
	big := make([]byte, legacyImageMaxBytes/2+1)
	first, _ := storeLegacyImage(big, "image/jpeg")
	_, _ = storeLegacyImage(big, "image/jpeg")
	if rec := serveLegacy(t, first); rec.Code != http.StatusNotFound {
		t.Fatalf("budget: oldest should be evicted, got %d", rec.Code)
	}
}

func TestImageToSrcModes(t *testing.T) {
	resetLegacyImages()
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))

	var cfg config.Config
	src, err := imageToSrc(img, "image/jpeg", cfg, "req", "dev", "test", false)
	if err != nil || !strings.HasPrefix(src, "data:image/jpeg;base64,") {
		t.Fatalf("default mode: %q %v", src, err)
	}

	cfg.Legacy = true
	src, err = imageToSrc(img, "image/jpeg", cfg, "req", "dev", "test", false)
	if err != nil || !strings.HasPrefix(src, "/legacy/image/") {
		t.Fatalf("legacy mode: %q %v", src, err)
	}
	if rec := serveLegacy(t, src); rec.Code != http.StatusOK || rec.Body.Len() == 0 {
		t.Fatalf("legacy image not served: %d", rec.Code)
	}
}
