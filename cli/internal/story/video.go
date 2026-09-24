package story

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"sb/internal/ojson"
	"sb/internal/render"
)

// FadeOutSeconds matches audio-playback.js FADE_OUT_IN_SECONDS.
const FadeOutSeconds = 0.25

// VideoJob is a prepared ffmpeg run (exporters/ffmpeg.js convertToVideo, without watermark).
type VideoJob struct {
	Dir     string   // temp folder with frames and video.ffconcat
	Concat  string   // ffconcat contents
	Args    []string // ffmpeg arguments
	Output  string
	Missing []string
}

func jsNum(f float64) string { return ojson.FormatNumber(f) }

// PrepareVideo flattens every board into a temp folder and builds the ffmpeg arguments.
func PrepareVideo(s *Scene, out string) (*VideoJob, error) {
	UpdateTiming(s)
	boards := s.Boards()
	if len(boards) == 0 {
		return nil, fmt.Errorf("the scene has no boards")
	}
	dir, err := os.MkdirTemp("", "sb-video-")
	if err != nil {
		return nil, err
	}
	job := &VideoJob{Dir: dir, Output: out}
	w, h := s.ImageSize()
	for _, b := range boards {
		img, missing := Flatten(s, b, w, h)
		job.Missing = append(job.Missing, missing...)
		if err := render.SavePNG(filepath.Join(dir, URL(b)), img); err != nil {
			os.RemoveAll(dir)
			return nil, err
		}
	}

	const streamOffset = 1 // video only (no watermark stream)
	var audioArgs, filters []string
	for _, b := range boards {
		a := b.Obj("audio")
		if a == nil {
			continue
		}
		audioArgs = append(audioArgs, "-i", s.ImagePath(a.Str("filename")))
		fade := fmt.Sprintf("areverse, afade=d=%s:curve=exp, areverse", jsNum(FadeOutSeconds))
		filter := fade
		if t := b.NumOr("time", 0); t > 0 {
			filter = fmt.Sprintf("%s,adelay=%s|%s", fade, jsNum(t), jsNum(t))
		}
		n := len(filters) + streamOffset
		filters = append(filters, fmt.Sprintf("[%d]%s[s%d]", n, filter, n))
	}
	audioComplex := ""
	if len(filters) > 0 {
		mix := ";"
		for i := range filters {
			mix += fmt.Sprintf("[s%d]", i+streamOffset)
		}
		mix += fmt.Sprintf("amix=%d[mix]", len(filters))
		audioComplex = strings.Join(filters, ";") + mix
	}

	// the last board is listed twice "because ffmpeg"
	lines := []string{"ffconcat version 1.0"}
	for _, b := range append(boards, boards[len(boards)-1]) {
		lines = append(lines, "", "file "+filepath.ToSlash(filepath.Join(dir, URL(b))), "duration "+jsNum(Duration(s, b)/1000))
	}
	job.Concat = strings.Join(lines, "\n")
	concatPath := filepath.Join(dir, "video.ffconcat")
	if err := os.WriteFile(concatPath, []byte(job.Concat), 0o644); err != nil {
		os.RemoveAll(dir)
		return nil, err
	}

	complex := []string{"[0]scale=-2:900[frame]", "[frame]null[vid]"}
	if audioComplex != "" {
		complex = append(complex, audioComplex)
	}
	args := []string{"-safe", "0", "-i", concatPath}
	args = append(args, audioArgs...)
	args = append(args, "-filter_complex", strings.Join(complex, ";"), "-map", "[vid]:v",
		"-r", jsNum(s.Fps()), "-vcodec", "libx264", "-acodec", "aac", "-pix_fmt", "yuv420p",
		"-tune", "stillimage", "-preset", "veryslow")
	if len(audioArgs) > 0 {
		args = append(args, "-map", "[mix]:a")
	}
	args = append(args, "-movflags", "+faststart", "-n", "-stats", out)
	job.Args = args
	return job, nil
}

// FfmpegPath finds ffmpeg on PATH (SB_FFMPEG overrides).
func FfmpegPath() (string, error) {
	if p := os.Getenv("SB_FFMPEG"); p != "" {
		return p, nil
	}
	p, err := exec.LookPath("ffmpeg")
	if err != nil {
		return "", fmt.Errorf("ffmpeg not found on PATH (install it, e.g. `brew install ffmpeg`, or set SB_FFMPEG)")
	}
	return p, nil
}

// Run executes ffmpeg and removes the temp folder; stderr goes to log.
func (j *VideoJob) Run(log *os.File) error {
	defer os.RemoveAll(j.Dir)
	bin, err := FfmpegPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(j.Output), 0o755); err != nil {
		return err
	}
	cmd := exec.Command(bin, j.Args...)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if log != nil {
		cmd.Stderr = log
	}
	if err := cmd.Run(); err != nil {
		msg := stderr.String()
		if len(msg) > 2000 {
			msg = msg[len(msg)-2000:]
		}
		return fmt.Errorf("could not use ffmpeg: %v\n%s", err, msg)
	}
	return nil
}
