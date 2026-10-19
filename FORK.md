# Immich Kiosk for older iPads

An **unofficial** fork of [Immich Kiosk](https://github.com/damongolding/immich-kiosk) (AGPL-3.0) that
keeps it running on old iPads used as photo frames. It was written for a 1st-gen iPad Air (A7,
1 GB RAM, iOS 12.5.8), which iOS killed for memory about once a day running stock Kiosk and which
could not play iPhone videos or Live Photos at all. Not affiliated with Immich or Immich Kiosk.

Everything else is upstream Kiosk, unchanged, and the extras are **off by default**.

**Tested on one device only** (iPad Air, iOS 12.5.8, Safari and the home-screen web app). Other
iOS 12–15 iPads will probably benefit; reports welcome.

**Written with AI help.** The changes in this fork were written with Claude (Anthropic) and tested
on the device above. Upstream does not accept AI-written pull requests, so none of this is offered
upstream as code. Please report problems here, not to Immich Kiosk, unless they also happen on
stock Kiosk.

## Running it

Same as stock Kiosk (see its docs); only the image changes:

```yaml
services:
  immich-kiosk:
    image: ghcr.io/elwinchia/immich-kiosk-older-ipads:latest
    environment:
      KIOSK_IMMICH_URL: http://immich-server:2283
      KIOSK_IMMICH_API_KEY: <a read-only Immich API key>
      KIOSK_OLDER_IPAD: true            # or per device: http://<kiosk>/?older_ipad=true
      KIOSK_LIVE_PHOTOS: true
      KIOSK_LIVE_PHOTO_BOOMERANG: true  # optional
    ports:
      - 3000:3000
    restart: unless-stopped
```

On the iPad, open the URL in Safari and use Share → Add to Home Screen. Set Auto-Lock to Never
yourself (iOS 12 has no Wake Lock API). If anything misbehaves, add `?older_ipad_debug=true` to the
URL: a text overlay then reports what the page is doing.

## Options

| Option | Env | Default | What it does |
|---|---|---|---|
| `older_ipad` | `KIOSK_OLDER_IPAD` | `false` | turns on older-iPad mode (below) |
| `older_ipad_reload` | `KIOSK_OLDER_IPAD_RELOAD` | `60` | minutes between full page reloads in older-iPad mode, `0` = off |
| `older_ipad_debug` | (URL only) | `false` | on-screen debug overlay in older-iPad mode |
| `live_photo_boomerang` | `KIOSK_LIVE_PHOTO_BOOMERANG` | `false` | Live Photos play forward then backward, looping (any mode) |

## What older-iPad mode changes

| | Stock Kiosk | Older-iPad mode |
|---|---|---|
| Images in each slide | inline base64 data URIs (~1–2 MB of text each, plus the blurred copy) | short URL `/ipad/image/<128-bit token>`, held server-side 30 min / 200 MB |
| Removing an old frame | `.remove()` | images set to a 1×1 GIF and videos unloaded first, so WebKit frees them |
| Long-running page | never reloads | full reload every `older_ipad_reload` minutes, only once `/health` answers |
| Video | Immich playback as-is (HEVC from iPhones) | transcoded to 720p H.264 Main, muted, faststart (ffmpeg) |
| Inserted videos | played where htmx put them | recreated with `document.createElement` first: iOS 12 rejects `play()` (NotAllowedError) on a muted video parsed in another document and adopted, which is what htmx does |

Video posters are still inline base64 (one per video, not per slide).

## Changes in every mode

- **Container base:** Alpine + ffmpeg instead of distroless (same uid 65532).
- **Web app manifest:** `start_url` keeps the query string. Upstream sets it to the bare path, so
  "Add to Home Screen" dropped every URL option (`?older_ipad=true`, the album, …).

## Boomerang Live Photos

With `live_photo_boomerang`, the reversed clip is appended in the same server-side ffmpeg pass
(`internal/video/older_ipad.go`), so the only copy is the temp clip Kiosk's video manager deletes
10 min after its last view; nothing is written to Immich. A 3 s clip becomes ~6 s, ~0.7 s to make,
~385 MB peak RAM (ffmpeg's `reverse` buffers every frame).

The page shows the photo for 1 s, then loops the clip as one `<video loop>` (not upstream's
top/bottom pair, whose fade at each end showed the photo between loops). The video stays visible
but loads nothing (`preload="none"`), so the photo shows through, and `play()` is called after the
hold (`live-photo.ts`, `BOOMERANG_HOLD_MS`). Two iOS 12 rules force this order: it rejects `play()`
on an `opacity:0` muted video, and it loads nothing before `play()`, so `canplay` cannot gate the
start. `live_photo_loop_delay` does not apply in boomerang mode.

## Files

`internal/routes/routes_older_ipad.go` (+ test), `internal/video/older_ipad.go`,
`frontend/src/ts/older-ipad.ts`, `.github/workflows/older-ipads-release.yaml`; small hooks in
`config.go`, `routes_asset_helpers.go`, `routes_previous_asset.go`, `routes_manifest.go`,
`routes_video.go`, `video.go`, `utils.go` (`EncodeImage` split out of `ImageToBase64`),
`views_home.templ`, `layout.templ`, `live_photo.templ`, `kiosk.ts`, `live-photo.ts`, `main.go`,
`Dockerfile`. Upstream's release, Reddit and auto-close workflows are kept as files (so merges stay
clean) but disabled in this repo's Actions settings.

## Releases

Tags are `v<upstream version>-ipad.<n>`, e.g. `v0.45.1-ipad.1`; pushing one builds
`ghcr.io/elwinchia/immich-kiosk-older-ipads:<version>` and `:latest` (amd64, arm64).

## Staying current with upstream

```sh
git fetch upstream            # fetches main + tags only (upstream has branches that clash by case on macOS)
git checkout main
git merge upstream/main
go tool templ generate && go build ./... && go test ./internal/routes/ -run 'OlderIPad|ImageToSrc'
```

Conflicts, when they come, will be in the hook lines above — each is marked `older-iPad` or `fork`.
