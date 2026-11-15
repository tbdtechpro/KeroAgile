package main

import (
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
	remote, err := newRemote(a)
	if err != nil {
		return nil, err
	}
	return artifacts.New(svc, st, artifacts.Settings{
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
	return nil, nil
}
