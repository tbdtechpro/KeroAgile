package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/tbdtechpro/KeroAgile/internal/artifacts"
	"github.com/tbdtechpro/KeroAgile/internal/config"
	"github.com/tbdtechpro/KeroAgile/internal/domain"
)

// newKeeper builds the file keeper from the [artifacts] configuration. The
// holding area defaults to "artifacts" beside the database, so
// KEROAGILE_DATA_DIR moves both.
func newKeeper() (*artifacts.Keeper, error) {
	a, err := cfg.Artifacts.Resolved()
	if err != nil {
		return nil, err
	}
	dir := a.Dir
	if dir == "" {
		dir = filepath.Join(filepath.Dir(config.DBPath()), "artifacts")
	}
	home, _ := os.UserHomeDir()
	dir = domain.ExpandHome(dir, home)
	// A remote that cannot be set up (no credentials yet, say) must not
	// stop files being kept here: run without it and report why.
	remote, remoteErr := newRemote(a)
	return artifacts.New(svc, st, artifacts.Settings{
		RemoteErr:        remoteErr,
		Dir:              dir,
		LocalMax:         a.LocalMax,
		MinFree:          a.MinFree,
		KeepLocalUnder:   a.KeepLocalUnder,
		MaxFile:          a.MaxFile,
		SweepMaxFile:     a.SweepMaxFile,
		KeepUnreferenced: a.KeepUnreferenced,
		DefaultPolicy:    domain.ArtifactPolicy(a.DefaultPolicy),
		SweepSkip:        a.SweepSkip,
		Remote:           remote,
	}), nil
}

// newRemote builds the remote store the configuration names, or nil.
func newRemote(a config.Artifacts) (artifacts.Remote, error) {
	switch a.Remote {
	case "s3":
		c := cfg.Artifacts.S3
		if c.Endpoint == "" || c.Bucket == "" {
			return nil, fmt.Errorf("[artifacts.s3] needs endpoint and bucket")
		}
		creds, err := artifacts.LoadCredentials(c.Profile)
		if err != nil {
			return nil, err
		}
		return artifacts.NewS3(artifacts.S3Config{Endpoint: c.Endpoint, Region: c.Region, Bucket: c.Bucket, PathStyle: c.PathStyle}, creds), nil
	case "folder":
		p := cfg.Artifacts.Folder.Path
		if p == "" {
			return nil, fmt.Errorf("[artifacts.folder] needs path")
		}
		home, _ := os.UserHomeDir()
		return artifacts.NewFolder(domain.ExpandHome(p, home)), nil
	}
	return nil, nil
}
