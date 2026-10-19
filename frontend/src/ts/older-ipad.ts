/**
 * Older-iPad support (fork: immich-kiosk-older-ipads).
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
function scheduleOlderIPadReload(minutes: number): void {
    if (!minutes || minutes <= 0) return;

    const tryReload = (): void => {
        fetch(`/health?older-ipad-reload=${Date.now()}`, { cache: "no-store" })
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

/**
 * Shows a line of text on screen when `older_ipad_debug=true` is in the URL — old iPads
 * have no reachable console.
 * @param msg - What to show
 */
function olderIPadDebug(msg: string): void {
    if (!/[?&]older_ipad_debug=(true|1)\b/.test(window.location.search)) return;
    let box = document.getElementById("older-ipad-debug");
    if (!box) {
        box = document.createElement("div");
        box.id = "older-ipad-debug";
        box.style.cssText =
            "position:fixed;left:8px;top:8px;z-index:99999;max-width:60%;font:12px/1.3 monospace;" +
            "color:#fff;background:rgba(0,0,0,.7);padding:6px 8px;border-radius:4px;pointer-events:none";
        document.body.appendChild(box);
    }
    const line = document.createElement("div");
    line.textContent = `${new Date().toLocaleTimeString()} ${msg}`;
    box.appendChild(line);
    while (box.childNodes.length > 8 && box.firstChild) {
        box.removeChild(box.firstChild);
    }
}

/**
 * Makes videos inserted by htmx actually autoplay on old iOS. htmx parses fragments in a
 * separate document (a <template>) and moves the nodes into the page; iOS 12 WebKit
 * then refuses play() on such a video with NotAllowedError even when it is muted
 * (tested on an iPad Air 1, iOS 12.5.8: adopted videos fail, videos created in the page
 * play). So each inserted video is swapped for a fresh one made in this document, then
 * muted from script and started.
 * Must run before Kiosk's own hx-on::load handlers look the video up, so it is
 * registered as a capture listener on the document.
 * @param root - The element htmx just inserted
 */
function startInsertedVideos(root: Element): void {
    const inserted: HTMLVideoElement[] =
        root instanceof HTMLVideoElement
            ? [root]
            : Array.prototype.slice.call(root.querySelectorAll("video"));

    const videos = inserted.map((old) => {
        const fresh = document.createElement("video");
        for (let i = 0; i < old.attributes.length; i++) {
            const a = old.attributes[i];
            fresh.setAttribute(a.name, a.value);
        }
        while (old.firstChild) fresh.appendChild(old.firstChild);
        if (old.parentNode) old.parentNode.replaceChild(fresh, old);
        return fresh;
    });

    videos.forEach((video) => {
        if (video.hasAttribute("muted")) {
            video.muted = true;
            video.defaultMuted = true;
        }
        video.setAttribute("playsinline", "");
        video.setAttribute("webkit-playsinline", "");
        (video as HTMLVideoElement & { playsInline: boolean }).playsInline =
            true;

        if (!video.hasAttribute("autoplay")) return;

        const tag = (video.getAttribute("data-group") || "video").slice(-8);
        const attempt = (): void => {
            const p = video.play();
            if (p && typeof p.catch === "function") {
                p.then(() => olderIPadDebug(`playing ${tag}`)).catch(
                    (err: Error) =>
                        olderIPadDebug(
                            `play() ${tag}: ${err.name} ${err.message}`,
                        ),
                );
            }
        };
        attempt();
        // some iOS builds ignore play() before data arrives; try again once it can play
        video.addEventListener("canplay", attempt, { once: true });
        video.addEventListener("error", () => {
            const code = video.error ? video.error.code : "?";
            olderIPadDebug(`video error ${tag}: code ${code}`);
        });
    });
}

export { olderIPadDebug, releaseMedia, scheduleOlderIPadReload, startInsertedVideos };
