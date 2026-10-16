/**
 * Legacy-device support (fork: immich-kiosk-legacy).
 *
 * Old iPads (iOS 12, 1 GB of RAM) get killed by iOS when memory grows. Two things
 * keep it flat: media in a removed frame is released explicitly (WebKit keeps decoded
 * images and video buffers of detached nodes alive until a GC that may never come in
 * time), and the whole page is reloaded on a timer.
 */

const BLANK_GIF =
    "data:image/gif;base64,R0lGODlhAQABAAAAACH5BAEKAAEALAAAAAABAAEAAAICTAEAOw==";
const RELOAD_RETRY_MS = 60_000;

/**
 * Empties every image and video inside an element before it is removed.
 * @param el - The frame about to leave the DOM
 */
function releaseMedia(el: Element): void {
    el.querySelectorAll("img").forEach((img) => {
        img.removeAttribute("srcset");
        img.src = BLANK_GIF;
    });
    el.querySelectorAll("video").forEach((video) => {
        try {
            video.pause();
        } catch (_) {
            // already stopped
        }
        video.querySelectorAll("source").forEach((s) => {
            s.remove();
        });
        video.removeAttribute("src");
        try {
            video.load(); // releases the decoder and its buffers
        } catch (_) {
            // nothing loaded
        }
    });
}

/**
 * Reloads the page every `minutes`, but only once the server answers, so an outage
 * leaves the last photo on screen instead of an error page.
 * @param minutes - Minutes between reloads; 0 or less disables it
 */
function scheduleLegacyReload(minutes: number): void {
    if (!minutes || minutes <= 0) return;

    const tryReload = (): void => {
        fetch(`/health?legacy-reload=${Date.now()}`, { cache: "no-store" })
            .then((res) => {
                if (res.ok) {
                    window.location.reload();
                } else {
                    setTimeout(tryReload, RELOAD_RETRY_MS);
                }
            })
            .catch(() => setTimeout(tryReload, RELOAD_RETRY_MS));
    };

    setTimeout(tryReload, minutes * 60_000);
}

export { releaseMedia, scheduleLegacyReload };
