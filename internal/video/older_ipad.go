package video

// Older-iPad support (fork: immich-kiosk-older-ipads).
//
// iPhones record HEVC, and Immich passes HEVC through when it is an accepted codec.
// Old iPads (A7/A8) have no HEVC decoder, so in older-iPad mode downloaded clips are
// transcoded to 720p H.264 Main, muted, with faststart. Needs ffmpeg on PATH; without
// it the original clip is kept and an error is logged.
//
// With boomerang, the clip plays forward then backward: the reversed copy (minus its
// first frame, so the turn does not stutter) is concatenated after the original. reverse
// buffers every frame, so it runs after the 720p scale to keep memory down (~125 MB for
// a 3 s Live Photo clip).

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

func transcodeForOlderIPad(src string, boomerang bool) (string, error) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return "", fmt.Errorf("ffmpeg not found: %w", err)
	}

	dst := strings.TrimSuffix(src, ".ipad.mp4") + ".ipad.mp4"
	tmp := dst + ".part"

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	vf := "scale='if(gt(iw,ih),-2,720)':'if(gt(iw,ih),720,-2)':out_range=tv,format=yuv420p,fps=30"
	if boomerang {
		vf += ",split[fwd][rev];[rev]reverse,trim=start_frame=1,setpts=PTS-STARTPTS[back];[fwd][back]concat=n=2:v=1:a=0"
	}

	cmd := exec.CommandContext(ctx, "nice", "-n", "10", "ffmpeg", "-nostdin", "-loglevel", "error", "-y",
		"-i", src, "-map", "0:v:0", "-an",
		"-vf", vf,
		"-c:v", "libx264", "-preset", "veryfast", "-crf", "24", "-profile:v", "main", "-level", "3.1",
		"-pix_fmt", "yuv420p", "-color_range", "tv", "-threads", "4", "-movflags", "+faststart",
		"-f", "mp4", tmp)

	if out, err := cmd.CombinedOutput(); err != nil {
		_ = os.Remove(tmp)
		return "", fmt.Errorf("ffmpeg: %w: %s", err, strings.TrimSpace(string(out)))
	}

	if err := os.Rename(tmp, dst); err != nil {
		_ = os.Remove(tmp)
		return "", err
	}

	return dst, nil
}
