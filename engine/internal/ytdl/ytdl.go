// Package ytdl mengunduh video dari tautan YouTube, dengan lebih dari satu
// mesin: bila satu gagal, mesin berikutnya dicoba.
//
// Urutan percobaan (diuji 14 Oktober 2026, dari laptop DAN dari VPS):
//
//  1. yt-dlp, client bawaan              — jalan di keduanya.
//  2. yt-dlp, client android_vr          — jalan di keduanya; jalur lain di
//     sisi YouTube, menolong saat client bawaan diblokir/diminta PO token.
//  3. Cobalt yang DI-HOST SENDIRI        — kode yang sama sekali lain
//     (youtube.js), jadi tidak ikut rusak bila yt-dlp rusak. Hanya dipakai
//     bila CLIPPER_COBALT_URL diisi.
//
// Yang SENGAJA tidak dipakai:
//   - instance Cobalt publik, Invidious/Piped, situs "savefrom": tautan dan
//     videonya lewat server orang lain yang tidak kita kenal.
//   - lux (iawia002/lux): rilis terakhir 2024 dan panic saat diuji ke YouTube.
//
// Keamanan: tautan dari pengguna TIDAK pernah diteruskan apa adanya. Hanya ID
// video 11 karakter yang diambil, lalu dibentuk ulang menjadi
// https://www.youtube.com/watch?v=<id>. Dengan begitu tidak ada argumen yang
// bisa diselundupkan ke yt-dlp ("--exec ..."), dan yt-dlp tidak bisa disuruh
// membuka alamat internal (SSRF) lewat extractor generiknya.
package ytdl

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

var idRe = regexp.MustCompile(`^[A-Za-z0-9_-]{11}$`)

// VideoID mengambil ID video dari tautan YouTube yang dikenal. Selain itu ditolak.
func VideoID(raw string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") {
		return "", errors.New("not a YouTube link")
	}
	host := strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.")
	var id string
	switch host {
	case "youtu.be":
		id = strings.Trim(u.Path, "/")
	case "youtube.com", "m.youtube.com", "music.youtube.com":
		if u.Path == "/watch" {
			id = u.Query().Get("v")
		} else {
			for _, pre := range []string{"/shorts/", "/live/", "/embed/"} {
				if rest, ok := strings.CutPrefix(u.Path, pre); ok {
					id = strings.Trim(rest, "/")
				}
			}
		}
	default:
		return "", errors.New("not a YouTube link")
	}
	if !idRe.MatchString(id) {
		return "", errors.New("the link does not contain a YouTube video ID")
	}
	return id, nil
}

// Options = letak biner dan batas unduhan.
type Options struct {
	Dir      string // folder hasil; berkasnya <Dir>/<id>.mp4
	YtDlp    string // biner yt-dlp; kosong = lewati
	FFmpeg   string // untuk menggabung video+audio
	Cobalt   string // alamat Cobalt self-hosted; kosong = lewati
	MaxBytes int64  // batas besar berkas
	Log      func(string)
	// Verify memeriksa berkas hasil (mis. ffprobe). Gagal = berkas dibuang dan
	// mesin berikutnya dicoba; video rusak tidak pernah sampai ke pengguna.
	Verify func(path string) error
}

// FromEnv mengisi alamat Cobalt dari CLIPPER_COBALT_URL.
func (o *Options) FromEnv() { o.Cobalt = os.Getenv("CLIPPER_COBALT_URL") }

// Info = hasil cek: video bisa diambil, beserta yang diketahui tentangnya.
type Info struct {
	Title    string  `json:"title"`
	Duration float64 `json:"duration"` // detik; 0 = tidak diketahui (Cobalt)
	Engine   string  `json:"engine"`   // mesin yang menjawab
}

type attempt struct {
	name     string
	check    func(ctx context.Context, watch string) (Info, error)
	download func(ctx context.Context, watch, out string) error
}

// attempts menyusun rantai mesin; urutannya sama untuk cek dan unduh.
func attempts(o Options) []attempt {
	var tries []attempt
	if o.YtDlp != "" {
		for _, client := range []string{"", "android_vr"} {
			tries = append(tries, attempt{"yt-dlp " + or(client, "default"),
				func(ctx context.Context, w string) (Info, error) { return ytdlpCheck(ctx, o, client, w) },
				func(ctx context.Context, w, out string) error { return ytdlp(ctx, o, client, w, out) }})
		}
	}
	if o.Cobalt != "" {
		tries = append(tries, attempt{"cobalt",
			func(ctx context.Context, w string) (Info, error) {
				_, name, err := cobaltResolve(ctx, o, w)
				return Info{Title: strings.TrimSuffix(name, filepath.Ext(name))}, err
			},
			func(ctx context.Context, w, out string) error { return cobalt(ctx, o, w, out) }})
	}
	return tries
}

var errNoEngine = errors.New("no YouTube downloader is set up: install yt-dlp on the Requirements page")

// Check menanyakan ke tiap mesin, berurutan, apakah video bisa diambil — tanpa
// mengunduh videonya. Galat dari SEMUA mesin dikembalikan bila tak satu pun bisa.
func Check(ctx context.Context, raw string, o Options) (Info, error) {
	id, err := VideoID(raw)
	if err != nil {
		return Info{}, err
	}
	if o.Log == nil {
		o.Log = func(string) {}
	}
	tries := attempts(o)
	if len(tries) == 0 {
		return Info{}, errNoEngine
	}
	watch := "https://www.youtube.com/watch?v=" + id
	var errs []error
	for _, t := range tries {
		if ctx.Err() != nil {
			return Info{}, ctx.Err()
		}
		info, err := t.check(ctx, watch)
		if err == nil {
			info.Engine = t.name
			o.Log("Checked with " + t.name + ": " + info.Title)
			return info, nil
		}
		o.Log(t.name + " check failed: " + err.Error())
		errs = append(errs, fmt.Errorf("%s: %w", t.name, err))
	}
	return Info{}, errors.Join(errs...)
}

// Download mencoba tiap mesin berurutan dan mengembalikan path mp4 pertama yang
// berhasil. Semua galat dilaporkan bila semuanya gagal.
func Download(ctx context.Context, raw string, o Options) (string, error) {
	id, err := VideoID(raw)
	if err != nil {
		return "", err
	}
	if o.Log == nil {
		o.Log = func(string) {}
	}
	if o.MaxBytes <= 0 {
		o.MaxBytes = 8 << 30
	}
	if err := os.MkdirAll(o.Dir, 0o755); err != nil {
		return "", err
	}
	watch := "https://www.youtube.com/watch?v=" + id
	final := filepath.Join(o.Dir, id+".mp4")

	tries := attempts(o)
	if len(tries) == 0 {
		return "", errNoEngine
	}

	var errs []error
	for _, t := range tries {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		tmp, err := os.MkdirTemp(o.Dir, ".dl-")
		if err != nil {
			return "", err
		}
		out := filepath.Join(tmp, "video.mp4")
		o.Log("Downloading with " + t.name + "…")
		err = t.download(ctx, watch, out)
		if err == nil {
			err = nonEmpty(out)
		}
		if err == nil && o.Verify != nil {
			err = o.Verify(out)
		}
		if err == nil {
			err = os.Rename(out, final)
		}
		os.RemoveAll(tmp)
		if err == nil {
			o.Log("Downloaded with " + t.name + ".")
			return final, nil
		}
		o.Log(t.name + " failed: " + err.Error())
		errs = append(errs, fmt.Errorf("%s: %w", t.name, err))
	}
	return "", errors.Join(errs...)
}

// H.264 ≤1080p bila ada: paling aman untuk ffmpeg di tahap berikutnya. Cek
// memakai format yang SAMA, jadi "bisa" di cek berarti format ini memang ada.
const ytFormat = "bv*[height<=1080][vcodec^=avc1]+ba[ext=m4a]/bv*[height<=1080]+ba/b[height<=1080]/b"

// ytdlpArgs = argumen bersama cek & unduh; watch selalu yang terakhir, di belakang "--".
func ytdlpArgs(o Options, client, watch string, extra ...string) []string {
	args := append([]string{
		"--ignore-config", // config pengguna bisa berisi --exec
		"--no-playlist", "--no-warnings", "-f", ytFormat,
	}, extra...)
	// Tantangan "n" YouTube butuh runtime JS; yt-dlp hanya mencari deno sendiri.
	if _, err := exec.LookPath("node"); err == nil {
		args = append(args, "--js-runtimes", "node")
	}
	if client != "" {
		args = append(args, "--extractor-args", "youtube:player_client="+client)
	}
	return append(args, "--", watch)
}

func runYtdlp(ctx context.Context, o Options, args []string) (string, error) {
	cmd := exec.CommandContext(ctx, o.YtDlp, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("%v: %s", err, lastLine(stderr.String()))
	}
	return stdout.String(), nil
}

// ytdlpCheck: --print membuat yt-dlp hanya menyimulasikan; tidak ada video turun.
func ytdlpCheck(ctx context.Context, o Options, client, watch string) (Info, error) {
	out, err := runYtdlp(ctx, o, ytdlpArgs(o, client, watch,
		"--print", "%(title)s", "--print", "%(duration)s", "--print", "%(is_live)s"))
	if err != nil {
		return Info{}, err
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) < 3 {
		return Info{}, errors.New("unexpected reply from yt-dlp")
	}
	if strings.TrimSpace(lines[2]) == "True" {
		return Info{}, errors.New("this is a live stream: wait until it has ended")
	}
	var dur float64
	fmt.Sscan(lines[1], &dur)
	return Info{Title: strings.TrimSpace(lines[0]), Duration: dur}, nil
}

func ytdlp(ctx context.Context, o Options, client, watch, out string) error {
	extra := []string{"--no-progress", "--max-filesize", fmt.Sprint(o.MaxBytes),
		"--merge-output-format", "mp4", "-o", out}
	// Nama polos ("ffmpeg") dibaca yt-dlp sebagai path relatif: ffmpeg "tidak
	// ada", penggabungan dilewati diam-diam, dan video.mp4 tidak pernah terbentuk.
	if p, err := exec.LookPath(o.FFmpeg); o.FFmpeg != "" && err == nil {
		extra = append(extra, "--ffmpeg-location", p)
	}
	_, err := runYtdlp(ctx, o, ytdlpArgs(o, client, watch, extra...))
	return err
}

// cobaltResolve meminta Cobalt menyiapkan video; balasannya alamat unduh +
// nama berkas. Tanpa mengunduh apa pun — itulah cek untuk Cobalt.
func cobaltResolve(ctx context.Context, o Options, watch string) (string, string, error) {
	base, err := url.Parse(o.Cobalt)
	if err != nil || (base.Scheme != "https" && base.Scheme != "http") {
		return "", "", errors.New("CLIPPER_COBALT_URL is not an http(s) address")
	}
	body, _ := json.Marshal(map[string]string{
		"url": watch, "videoQuality": "1080", "youtubeVideoCodec": "h264",
	})
	req, err := http.NewRequestWithContext(ctx, "POST", base.String(), bytes.NewReader(body))
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	if k := os.Getenv("CLIPPER_COBALT_KEY"); k != "" {
		req.Header.Set("Authorization", "Api-Key "+k)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", "", err
	}
	defer res.Body.Close()
	var r struct {
		Status   string `json:"status"`
		URL      string `json:"url"`
		Filename string `json:"filename"`
		Error    struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&r); err != nil {
		return "", "", fmt.Errorf("unreadable reply (HTTP %d)", res.StatusCode)
	}
	if r.Status != "tunnel" && r.Status != "redirect" {
		return "", "", fmt.Errorf("cobalt said %s %s", r.Status, r.Error.Code)
	}
	return r.URL, r.Filename, nil
}

func cobalt(ctx context.Context, o Options, watch, out string) error {
	link, _, err := cobaltResolve(ctx, o, watch)
	if err != nil {
		return err
	}
	get, err := http.NewRequestWithContext(ctx, "GET", link, nil)
	if err != nil {
		return err
	}
	vid, err := http.DefaultClient.Do(get)
	if err != nil {
		return err
	}
	defer vid.Body.Close()
	if vid.StatusCode != 200 {
		return fmt.Errorf("video request returned HTTP %d", vid.StatusCode)
	}
	f, err := os.Create(out)
	if err != nil {
		return err
	}
	n, err := io.Copy(f, io.LimitReader(vid.Body, o.MaxBytes+1))
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil && n > o.MaxBytes {
		err = errors.New("video is larger than the size limit")
	}
	return err
}

func nonEmpty(p string) error {
	st, err := os.Stat(p)
	if err != nil {
		return errors.New("finished without producing a file")
	}
	if st.Size() == 0 {
		return errors.New("produced an empty file")
	}
	return nil
}

func lastLine(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	return strings.TrimSpace(lines[len(lines)-1])
}

func or(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
