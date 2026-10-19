import { legacyDebug } from "./legacy";

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

    const play = (): void => {
        if (!video.isConnected || !video.paused) return;
        const p = video.play();
        if (p && typeof p.catch === "function") {
            p.catch((err: Error) =>
                legacyDebug(`boomerang play(): ${err.name} ${err.message}`),
            );
        }
    };

    // Nothing is loaded before play() (preload="none", and iOS ignores preload anyway), so the
    // video is transparent and the photo shows. The video stays visible throughout: iOS 12
    // rejects play() on a hidden (opacity:0) muted video with NotAllowedError.
    setTimeout(() => {
        video.muted = true;
        play();
        // some iOS builds ignore play() before data arrives; try again once it can play
        video.addEventListener("canplay", play, { once: true });
    }, BOOMERANG_HOLD_MS);
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
