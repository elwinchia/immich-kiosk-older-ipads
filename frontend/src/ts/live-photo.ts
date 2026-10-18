enum LivePhoto {
    ATTRIBUTE = "data-live-photo",
    TOP = "top",
    BOTTOM = "bottom",
    FADE_CLASS = "live-photo-fade",
    BOOMERANG_ATTRIBUTE = "data-live-photo-boomerang",
}

// boomerang (fork): how long the photo shows before its clip starts looping
const BOOMERANG_HOLD_MS = 1000;

function startBoomerang(video: HTMLVideoElement): void {
    video.setAttribute("data-boomerang-started", "");
    const insertedAt = Date.now();

    const reveal = (): void => {
        const wait = Math.max(0, BOOMERANG_HOLD_MS - (Date.now() - insertedAt));
        setTimeout(() => {
            if (!video.isConnected) return;
            video.muted = true;
            const p = video.play();
            if (p && typeof p.catch === "function") p.catch(() => undefined);
            video.classList.remove(LivePhoto.FADE_CLASS);
        }, wait);
    };

    // HAVE_FUTURE_DATA
    if (video.readyState >= 3) reveal();
    else video.addEventListener("canplay", reveal, { once: true });
}

export function livePhoto(delay: number) {
    // bubble phase: legacy mode has already swapped in its recreated videos, so look them up
    document.addEventListener("htmx:load", () => {
        const pending = document.querySelectorAll(
            `video[${LivePhoto.BOOMERANG_ATTRIBUTE}]:not([data-boomerang-started])`,
        );
        for (let i = 0; i < pending.length; i++) {
            startBoomerang(pending[i] as HTMLVideoElement);
        }
    });

    document.addEventListener(
        "ended",
        (event) => {
            const target = event.target;
            if (!(target instanceof HTMLVideoElement)) return;
            if (!target.hasAttribute(LivePhoto.ATTRIBUTE)) return;

            const role = target.dataset.livePhotoRole;
            const group = target.dataset.group;
            if (!role || !group) return;

            const isTop = role === LivePhoto.TOP;

            const otherRole = isTop ? LivePhoto.BOTTOM : LivePhoto.TOP;

            const selector = `video[${LivePhoto.ATTRIBUTE}][data-group="${group}"][data-live-photo-role="${otherRole}"]`;
            const otherVideo = document.querySelector(selector);

            if (!(otherVideo instanceof HTMLVideoElement)) return;

            otherVideo.currentTime = 0;

            target.classList.add(LivePhoto.FADE_CLASS);
            otherVideo.classList.add(LivePhoto.FADE_CLASS);

            setTimeout(() => {
                if (!otherVideo.isConnected) return;
                otherVideo.classList.remove(LivePhoto.FADE_CLASS);
                otherVideo.play();
            }, delay);
        },
        true,
    );
}
