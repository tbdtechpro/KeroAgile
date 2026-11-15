package config

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// ArtifactsConfig is the [artifacts] table: where files kept against tickets
// go. Sizes are strings like "1GiB"; durations like "14d". Nothing set means
// files are kept on this machine and nothing is uploaded.
type ArtifactsConfig struct {
	Dir              string   `toml:"dir,omitempty"`               // holding area; default: "artifacts" beside the database
	LocalMax         string   `toml:"local_max,omitempty"`         // default 1GiB
	MinFree          string   `toml:"min_free,omitempty"`          // default 2GiB
	KeepLocalUnder   string   `toml:"keep_local_under,omitempty"`  // default 256KiB
	MaxFile          string   `toml:"max_file,omitempty"`          // default 2GiB
	SweepMaxFile     string   `toml:"sweep_max_file,omitempty"`    // default 100MiB
	KeepUnreferenced string   `toml:"keep_unreferenced,omitempty"` // default 14d
	DefaultPolicy    string   `toml:"default_policy,omitempty"`    // remote | local | off; default local
	Remote           string   `toml:"remote,omitempty"`            // none | s3 | folder; default none
	SweepSkip        []string `toml:"sweep_skip,omitempty"`        // more names the sweep leaves alone

	S3     S3Config     `toml:"s3,omitempty"`
	Folder FolderConfig `toml:"folder,omitempty"`
}

// S3Config is [artifacts.s3]. The secret is not here: credentials come from
// the environment or a profile in the shared AWS credentials file.
type S3Config struct {
	Endpoint  string `toml:"endpoint,omitempty"`
	Region    string `toml:"region,omitempty"`
	Bucket    string `toml:"bucket,omitempty"`
	PathStyle bool   `toml:"path_style,omitempty"`
	Profile   string `toml:"profile,omitempty"`
}

// FolderConfig is [artifacts.folder]: a directory used as the remote.
type FolderConfig struct {
	Path string `toml:"path,omitempty"`
}

// Artifacts holds the [artifacts] settings with defaults applied and sizes
// in bytes.
type Artifacts struct {
	Dir              string
	LocalMax         int64
	MinFree          int64
	KeepLocalUnder   int64
	MaxFile          int64
	SweepMaxFile     int64
	KeepUnreferenced time.Duration
	DefaultPolicy    string
	Remote           string
	SweepSkip        []string
}

// Resolved applies the defaults and checks the values.
func (c ArtifactsConfig) Resolved() (Artifacts, error) {
	a := Artifacts{Dir: c.Dir, SweepSkip: c.SweepSkip}
	sizes := []struct {
		name string
		val  string
		def  string
		dst  *int64
	}{
		{"local_max", c.LocalMax, "1GiB", &a.LocalMax},
		{"min_free", c.MinFree, "2GiB", &a.MinFree},
		{"keep_local_under", c.KeepLocalUnder, "256KiB", &a.KeepLocalUnder},
		{"max_file", c.MaxFile, "2GiB", &a.MaxFile},
		{"sweep_max_file", c.SweepMaxFile, "100MiB", &a.SweepMaxFile},
	}
	for _, sz := range sizes {
		v := sz.val
		if v == "" {
			v = sz.def
		}
		n, err := ParseSize(v)
		if err != nil {
			return Artifacts{}, fmt.Errorf("[artifacts] %s: %w", sz.name, err)
		}
		*sz.dst = n
	}
	keep := c.KeepUnreferenced
	if keep == "" {
		keep = "14d"
	}
	d, err := ParseDays(keep)
	if err != nil {
		return Artifacts{}, fmt.Errorf("[artifacts] keep_unreferenced: %w", err)
	}
	a.KeepUnreferenced = d

	a.DefaultPolicy = strings.ToLower(strings.TrimSpace(c.DefaultPolicy))
	switch a.DefaultPolicy {
	case "":
		a.DefaultPolicy = "local"
	case "remote", "local", "off":
	default:
		return Artifacts{}, fmt.Errorf("[artifacts] default_policy %q: use remote, local or off", c.DefaultPolicy)
	}
	a.Remote = strings.ToLower(strings.TrimSpace(c.Remote))
	switch a.Remote {
	case "":
		a.Remote = "none"
	case "none", "s3", "folder":
	default:
		return Artifacts{}, fmt.Errorf("[artifacts] remote %q: use none, s3 or folder", c.Remote)
	}
	return a, nil
}

var sizeUnits = map[string]float64{
	"": 1, "b": 1,
	"kb": 1e3, "mb": 1e6, "gb": 1e9, "tb": 1e12,
	"kib": 1 << 10, "mib": 1 << 20, "gib": 1 << 30, "tib": 1 << 40,
	"k": 1 << 10, "m": 1 << 20, "g": 1 << 30, "t": 1 << 40,
}

// ParseSize reads "1GiB", "256KiB", "500 MB" or a plain number of bytes.
// KiB, MiB, GiB and TiB are powers of 1024; KB, MB, GB and TB of 1000.
func ParseSize(s string) (int64, error) {
	t := strings.TrimSpace(s)
	i := 0
	for i < len(t) && (t[i] >= '0' && t[i] <= '9' || t[i] == '.') {
		i++
	}
	if i == 0 {
		return 0, fmt.Errorf("size %q: start with a number, e.g. 1GiB", s)
	}
	n, err := strconv.ParseFloat(t[:i], 64)
	if err != nil {
		return 0, fmt.Errorf("size %q: %w", s, err)
	}
	unit, ok := sizeUnits[strings.ToLower(strings.TrimSpace(t[i:]))]
	if !ok {
		return 0, fmt.Errorf("size %q: unknown unit; use B, KiB, MiB, GiB or TiB", s)
	}
	return int64(n * unit), nil
}

// ParseDays reads "14d", or anything time.ParseDuration accepts ("36h").
func ParseDays(s string) (time.Duration, error) {
	t := strings.TrimSpace(s)
	if strings.HasSuffix(t, "d") {
		n, err := strconv.ParseFloat(strings.TrimSuffix(t, "d"), 64)
		if err == nil && n >= 0 {
			return time.Duration(n * float64(24*time.Hour)), nil
		}
	}
	d, err := time.ParseDuration(t)
	if err != nil {
		return 0, fmt.Errorf("duration %q: use days like 14d, or hours like 36h", s)
	}
	return d, nil
}
