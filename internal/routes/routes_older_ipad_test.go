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

func resetOlderIPadImages() {
	olderIPadImages.Lock()
	olderIPadImages.byToken = map[string]olderIPadImage{}
	olderIPadImages.order = nil
	olderIPadImages.size = 0
	olderIPadImages.Unlock()
}

func serveOlderIPad(t *testing.T, url string) *httptest.ResponseRecorder {
	t.Helper()
	e := echo.New()
	e.GET("/ipad/image/:token", OlderIPadImage)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, url, nil))
	return rec
}

func TestOlderIPadImageRoundTrip(t *testing.T) {
	resetOlderIPadImages()
	url, err := storeOlderIPadImage([]byte("jpeg-bytes"), "image/jpeg")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(url, "/ipad/image/") || len(url) != len("/ipad/image/")+32 {
		t.Fatalf("unexpected url %q", url)
	}

	rec := serveOlderIPad(t, url)
	if rec.Code != http.StatusOK || rec.Body.String() != "jpeg-bytes" {
		t.Fatalf("got %d %q", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("Content-Type") != "image/jpeg" || rec.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("headers %v", rec.Header())
	}

	if rec := serveOlderIPad(t, "/ipad/image/0123456789abcdef0123456789abcdef"); rec.Code != http.StatusNotFound {
		t.Fatalf("unknown token: got %d", rec.Code)
	}
}

func TestOlderIPadImageExpiresAndBudget(t *testing.T) {
	resetOlderIPadImages()
	url, _ := storeOlderIPadImage([]byte("old"), "image/jpeg")
	token := strings.TrimPrefix(url, "/ipad/image/")

	olderIPadImages.Lock()
	img := olderIPadImages.byToken[token]
	img.expires = time.Now().Add(-time.Second)
	olderIPadImages.byToken[token] = img
	olderIPadImages.Unlock()

	if rec := serveOlderIPad(t, url); rec.Code != http.StatusNotFound {
		t.Fatalf("expired: got %d", rec.Code)
	}

	// the next store prunes the expired entry
	_, _ = storeOlderIPadImage([]byte("new"), "image/jpeg")
	olderIPadImages.Lock()
	_, stillThere := olderIPadImages.byToken[token]
	n, size := len(olderIPadImages.byToken), olderIPadImages.size
	olderIPadImages.Unlock()
	if stillThere || n != 1 || size != 3 {
		t.Fatalf("after prune: stillThere=%v n=%d size=%d", stillThere, n, size)
	}

	// over budget: the oldest entries go first
	resetOlderIPadImages()
	big := make([]byte, olderIPadImageMaxBytes/2+1)
	first, _ := storeOlderIPadImage(big, "image/jpeg")
	_, _ = storeOlderIPadImage(big, "image/jpeg")
	if rec := serveOlderIPad(t, first); rec.Code != http.StatusNotFound {
		t.Fatalf("budget: oldest should be evicted, got %d", rec.Code)
	}
}

func TestImageToSrcModes(t *testing.T) {
	resetOlderIPadImages()
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))

	var cfg config.Config
	src, err := imageToSrc(img, "image/jpeg", cfg, "req", "dev", "test", false)
	if err != nil || !strings.HasPrefix(src, "data:image/jpeg;base64,") {
		t.Fatalf("default mode: %q %v", src, err)
	}

	cfg.OlderIPad = true
	src, err = imageToSrc(img, "image/jpeg", cfg, "req", "dev", "test", false)
	if err != nil || !strings.HasPrefix(src, "/ipad/image/") {
		t.Fatalf("older-iPad mode: %q %v", src, err)
	}
	if rec := serveOlderIPad(t, src); rec.Code != http.StatusOK || rec.Body.Len() == 0 {
		t.Fatalf("older-iPad image not served: %d", rec.Code)
	}
}
