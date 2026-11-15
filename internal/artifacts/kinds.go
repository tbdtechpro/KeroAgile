package artifacts

import (
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/tbdtechpro/KeroAgile/internal/domain"
)

var kindByExt = map[string]domain.ArtifactKind{}

func init() {
	add := func(k domain.ArtifactKind, exts ...string) {
		for _, e := range exts {
			kindByExt[e] = k
		}
	}
	add(domain.KindScript, ".sh", ".bash", ".zsh", ".py", ".go", ".js", ".mjs", ".ts", ".rb", ".pl", ".ps1", ".bat", ".cmd",
		".sql", ".lua", ".tape", ".exp", ".awk", ".sed", ".mk")
	add(domain.KindLog, ".log", ".out", ".err", ".trace", ".jsonl", ".ndjson")
	add(domain.KindImage, ".png", ".jpg", ".jpeg", ".gif", ".webp", ".svg", ".bmp", ".tif", ".tiff", ".heic", ".avif")
	add(domain.KindAudio, ".wav", ".mp3", ".flac", ".ogg", ".oga", ".m4a", ".aac", ".opus", ".aiff", ".aif", ".mid", ".midi")
	add(domain.KindVideo, ".mp4", ".webm", ".mov", ".mkv", ".avi", ".m4v", ".ogv", ".cast")
	add(domain.KindDocument, ".pdf", ".md", ".txt", ".html", ".htm", ".csv", ".tsv", ".json", ".yaml", ".yml", ".toml",
		".xml", ".doc", ".docx", ".odt", ".rtf", ".xlsx", ".pptx")
}

// kindFor guesses what sort of file name is from its extension.
func kindFor(name string) domain.ArtifactKind {
	if k, ok := kindByExt[strings.ToLower(filepath.Ext(name))]; ok {
		return k
	}
	return domain.KindOther
}

// mediaTypeFor is the file's MIME type, from its extension or, failing that,
// its first bytes.
func mediaTypeFor(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext { // ones the system tables often lack or get wrong
	case ".md":
		return "text/markdown; charset=utf-8"
	case ".log", ".out", ".err", ".sh", ".py", ".go", ".ts", ".tape", ".jsonl":
		return "text/plain; charset=utf-8"
	case ".cast":
		return "application/x-asciicast"
	}
	if t := mime.TypeByExtension(ext); t != "" {
		return t
	}
	f, err := os.Open(path)
	if err != nil {
		return "application/octet-stream"
	}
	defer f.Close()
	buf := make([]byte, 512)
	n, _ := f.Read(buf)
	return http.DetectContentType(buf[:n])
}
