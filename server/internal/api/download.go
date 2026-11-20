package api

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

var (
	apkNamePattern    = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*\.apk$`)
	apkVersionPattern = regexp.MustCompile(`^CoupleLink-Android-(1\.0-r(\d+))\.apk$`)
)

// serveDownload streams a packaged client from the configured download directory.
func (s *Server) serveDownload(w http.ResponseWriter, r *http.Request) error {
	name := r.PathValue("name")
	if s.cfg.DownloadDir == "" || !apkNamePattern.MatchString(name) {
		return fail(404, "not_found")
	}
	file, err := os.Open(filepath.Join(s.cfg.DownloadDir, name))
	if err != nil {
		return fail(404, "not_found")
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || info.IsDir() {
		return fail(404, "not_found")
	}
	w.Header().Set("Content-Type", "application/vnd.android.package-archive")
	w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
	http.ServeContent(w, r, name, info.ModTime(), file)
	return nil
}

// updateManifest reports the newest APK available in the download directory so a
// client can offer "check for updates" without any store.
func (s *Server) updateManifest(w http.ResponseWriter, r *http.Request) error {
	if s.cfg.DownloadDir == "" {
		return fail(503, "update_unavailable")
	}
	name, version, size, ok := s.latestAPK()
	if !ok {
		return fail(503, "update_unavailable")
	}
	hash, _ := fileSHA256(filepath.Join(s.cfg.DownloadDir, name))
	write(w, 200, map[string]any{
		"version": version,
		"file":    name,
		"path":    "/download/" + strings.TrimSpace(name),
		"size":    size,
		"sha256":  hash,
	})
	return nil
}

// latestAPK reports the newest packaged client in the download directory.
func (s *Server) latestAPK() (name, version string, size int64, ok bool) {
	if s.cfg.DownloadDir == "" {
		return "", "", 0, false
	}
	entries, err := os.ReadDir(s.cfg.DownloadDir)
	if err != nil {
		return "", "", 0, false
	}
	revision := -1
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		match := apkVersionPattern.FindStringSubmatch(entry.Name())
		if match == nil {
			continue
		}
		rev, convErr := strconv.Atoi(match[2])
		if convErr != nil {
			continue
		}
		if rev > revision {
			name, revision = entry.Name(), rev
		}
	}
	if name == "" {
		return "", "", 0, false
	}
	info, err := os.Stat(filepath.Join(s.cfg.DownloadDir, name))
	if err != nil {
		return "", "", 0, false
	}
	return name, "1.0-r" + strconv.Itoa(revision), info.Size(), true
}

// humanSize renders a byte count for the admin page.
func humanSize(n int64) string {
	if n >= 1<<20 {
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	}
	if n >= 1<<10 {
		return fmt.Sprintf("%.0f KB", float64(n)/(1<<10))
	}
	return fmt.Sprintf("%d B", n)
}

// fileSHA256 hashes a packaged client so downloads can be verified.
func fileSHA256(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	digest := sha256.New()
	if _, err = io.Copy(digest, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(digest.Sum(nil)), nil
}
