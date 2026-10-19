# immich-kiosk-legacy

A fork of [immich-kiosk](https://github.com/damongolding/immich-kiosk) (AGPL-3.0) that adds
a **legacy mode** for old devices — written for a 1st-gen iPad Air (A7, 1 GB RAM, iOS 12.5),
which iOS killed for memory about once a day running stock Kiosk.

Everything else is upstream Kiosk, unchanged. Legacy mode is **off by default** and turned on
per device: `http://<kiosk>/?legacy=true` (or `legacy: true` / `KIOSK_LEGACY=true`).

## What legacy mode changes

| | Stock Kiosk | Legacy mode |
|---|---|---|
| Images in each slide | inline base64 data URIs (~1–2 MB of text each, plus the blurred copy) | short URL `/legacy/image/<128-bit token>`, held server-side 30 min / 200 MB |
| Removing an old frame | `.remove()` | images set to a 1×1 GIF and videos unloaded first, so WebKit frees them |
| Long-running page | never reloads | full reload every `legacy_reload` minutes (default 60), only once `/health` answers |
| Video | Immich playback as-is (HEVC from iPhones) | transcoded to 720p H.264 Main, muted, faststart (ffmpeg) |
| Container base | distroless | Alpine + ffmpeg (same uid 65532) |

Options: `legacy` (bool, default false), `legacy_reload` (minutes, default 60, 0 = off).
Video posters are still inline base64 (one per video, not per slide).

`live_photo_boomerang` (bool, default false; env `KIOSK_LIVE_PHOTO_BOOMERANG`, any mode): Live Photo clips
play forward then backward. The reversed half is appended in the same server-side ffmpeg pass
(`internal/video/legacy.go`), so the only copy is the temp clip the video manager deletes 10 min after
its last view; nothing is written to Immich. A 3 s clip becomes ~6 s, ~0.7 s to make, ~385 MB peak RAM.
In boomerang mode the clip is one looping `<video>` (not upstream's top/bottom pair, whose fade at
each end showed the photo between loops). It is visible but loads nothing (`preload="none"`), so the photo
shows through; `play()` is called 1 s after it was inserted (`live-photo.ts`, `BOOMERANG_HOLD_MS`).
Do not hide it while starting it: iOS 12 rejects `play()` on an `opacity:0` muted video
(NotAllowedError), and iOS loads nothing before `play()`, so `canplay` cannot gate the start.
`live_photo_loop_delay` does not apply.

## Files

`internal/routes/routes_legacy.go` (+ test), `internal/video/legacy.go`, `frontend/src/ts/legacy.ts`;
small hooks in `config.go`, `routes_asset_helpers.go`, `routes_previous_asset.go`, `video.go`,
`utils.go` (`EncodeImage` split out of `ImageToBase64`), `views_home.templ`, `kiosk.ts`, `main.go`,
`Dockerfile`.

## Staying current with upstream

```sh
git fetch upstream            # fetches main + tags only (upstream has branches that clash by case on macOS)
git checkout legacy
git merge upstream/main       # or: git rebase upstream/main
go tool templ generate && go build ./... && go test ./internal/routes/ -run 'Legacy|ImageToSrc'
```

Conflicts, when they come, will be in the hook lines above — each is marked `legacy` or `fork`.
