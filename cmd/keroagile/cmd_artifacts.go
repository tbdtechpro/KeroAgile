package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/spf13/cobra"
	"github.com/tbdtechpro/KeroAgile/internal/artifacts"
	"github.com/tbdtechpro/KeroAgile/internal/domain"
)

var artifactsCmd = &cobra.Command{
	Use:     "artifacts",
	Aliases: []string{"files"},
	Short:   "Files kept against tickets: scripts, logs, screenshots, recordings",
}

func keeperOrErr() (*artifacts.Keeper, error) {
	k, err := newKeeper()
	if err != nil {
		return nil, fmt.Errorf("[artifacts] configuration: %w", err)
	}
	return k, nil
}

func artifactID(s string) (int64, error) {
	id, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%w file ID %q: give the number from artifacts list", domain.ErrInvalid, s)
	}
	return id, nil
}

func printArtifacts(list []*domain.Artifact) {
	if jsonFlag {
		printJSON(list)
		return
	}
	if len(list) == 0 {
		fmt.Println("  no files")
		return
	}
	for _, a := range list {
		on := "unfiled"
		if a.TaskID != nil {
			on = *a.TaskID
		}
		fmt.Printf("  %-5d %-8s %-9s %-7s %-11s %s\n", a.ID, on, artifacts.HumanSize(a.Size), a.Kind, a.Where, a.Name)
	}
}

var artifactsAttachCmd = &cobra.Command{
	Use:   "attach <task-id> <path>",
	Short: "Keep a file against a ticket",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		k, err := keeperOrErr()
		if err != nil {
			return err
		}
		kind, _ := cmd.Flags().GetString("kind")
		note, _ := cmd.Flags().GetString("note")
		author := cfg.DefaultAssignee
		if _, err := svc.GetUser(author); err != nil {
			author = ""
		}
		a, err := k.Attach(args[0], args[1], domain.ArtifactKind(kind), note, author)
		if err != nil {
			return err
		}
		if jsonFlag {
			printJSON(a)
		} else {
			fmt.Printf("kept %s on %s as file %d (%s)\n", a.Name, *a.TaskID, a.ID, artifacts.HumanSize(a.Size))
		}
		return nil
	},
}

var artifactsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List a ticket's files, or a project's",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		task, _ := cmd.Flags().GetString("task")
		project, _ := cmd.Flags().GetString("project")
		unfiled, _ := cmd.Flags().GetBool("unfiled")
		if task == "" && project == "" {
			if p, err := projectForDir(""); err == nil {
				project = p.ID
			}
		}
		list, err := svc.ListArtifacts(domain.ArtifactQuery{TaskID: task, ProjectID: project, Unfiled: unfiled})
		if err != nil {
			return err
		}
		printArtifacts(list)
		return nil
	},
}

var artifactsFetchCmd = &cobra.Command{
	Use:   "fetch <file-id>",
	Short: "Get a file onto this machine and print where it is",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := artifactID(args[0])
		if err != nil {
			return err
		}
		k, err := keeperOrErr()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		path, err := k.Fetch(ctx, id)
		if err != nil {
			return err
		}
		if to, _ := cmd.Flags().GetString("to"); to != "" {
			if err := copyFile(path, to); err != nil {
				return err
			}
			path, _ = filepath.Abs(to)
		}
		if jsonFlag {
			printJSON(map[string]any{"id": id, "path": path})
		} else {
			fmt.Println(path)
		}
		return nil
	},
}

func copyFile(from, to string) error {
	if err := os.MkdirAll(filepath.Dir(to), 0o755); err != nil {
		return err
	}
	in, err := os.Open(from)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(to, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

var artifactsFileCmd = &cobra.Command{
	Use:   "file <file-id> <task-id>",
	Short: "Put a file (usually an unfiled one) on a ticket",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := artifactID(args[0])
		if err != nil {
			return err
		}
		a, err := svc.FileArtifact(id, args[1])
		if err != nil {
			return err
		}
		if jsonFlag {
			printJSON(a)
		} else {
			fmt.Printf("file %d is on %s\n", a.ID, *a.TaskID)
		}
		return nil
	},
}

var artifactsDiscardCmd = &cobra.Command{
	Use:   "discard <file-id>",
	Short: "Discard a file; the sweep will not keep it again",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := artifactID(args[0])
		if err != nil {
			return err
		}
		if err := svc.DiscardArtifact(id); err != nil {
			return err
		}
		if jsonFlag {
			printJSON(map[string]any{"discarded": id})
		} else {
			fmt.Printf("discarded file %d\n", id)
		}
		return nil
	},
}

var artifactsStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show the holding area, the remote store, and what is waiting",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		k, err := keeperOrErr()
		if err != nil {
			return err
		}
		s, err := k.Status()
		if err != nil {
			return err
		}
		if jsonFlag {
			printJSON(s)
			return nil
		}
		fmt.Printf("holding area  %s\n", s.Dir)
		fmt.Printf("  in use      %s of %s (local_max)\n", artifacts.HumanSize(s.LocalBytes), artifacts.HumanSize(s.LocalMax))
		if s.DiskFree > 0 {
			fmt.Printf("  disk free   %s (min_free %s)\n", artifacts.HumanSize(s.DiskFree), artifacts.HumanSize(s.MinFree))
		}
		fmt.Printf("default policy %s\n", s.DefaultPolicy)
		fmt.Printf("remote        %s\n", s.Remote)
		if s.Remote != "none" {
			fmt.Printf("  waiting     %d files, %s\n", s.WaitingFiles, artifacts.HumanSize(s.WaitingBytes))
			if s.LastSync != "" {
				fmt.Printf("  last sync   %s\n", s.LastSync)
			}
			if s.LastSyncError != "" {
				fmt.Printf("  last error  %s\n", s.LastSyncError)
			}
		}
		if s.LastHookError != "" {
			fmt.Printf("last hook error  %s\n", s.LastHookError)
		}
		for _, sk := range s.SkippedSweeps {
			fmt.Printf("not swept     %s  (%s, %d times, last %s)\n", sk.Dir, sk.Reason, sk.Count, sk.LastAt.Local().Format("2006-01-02 15:04"))
		}
		return nil
	},
}

var artifactsSweepCmd = &cobra.Command{
	Use:   "sweep --dir <folder>",
	Short: "Keep a folder's new or changed files as unfiled files of a project",
	Long: `Keeps the files in a folder that are not kept already, as unfiled files of a
project (by default, the project of the working directory). The Stop hook runs
this for each session's scratch folder; run it by hand for any other folder.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, _ := cmd.Flags().GetString("dir")
		if dir == "" {
			return fmt.Errorf("%w sweep: --dir is required", domain.ErrInvalid)
		}
		session, _ := cmd.Flags().GetString("session")
		projectID, _ := cmd.Flags().GetString("project")
		var p *domain.Project
		var err error
		if projectID != "" {
			p, err = svc.GetProject(projectID)
		} else {
			p, err = projectForDir("")
		}
		if err != nil {
			return err
		}
		k, err := keeperOrErr()
		if err != nil {
			return err
		}
		res, err := k.Sweep(context.Background(), artifacts.SweepInput{Dir: dir, SessionID: session, Project: p})
		if err != nil {
			return err
		}
		if jsonFlag {
			printJSON(res)
		} else {
			fmt.Printf("kept %d, updated %d, skipped %d %v\n", res.Kept, res.Updated, res.Skipped, res.Reasons)
		}
		return nil
	},
}

func init() {
	artifactsSweepCmd.Flags().String("dir", "", "folder to sweep")
	artifactsSweepCmd.Flags().String("session", "", "session ID to record")
	artifactsSweepCmd.Flags().String("project", "", "project (default: the project of the working directory)")
	artifactsAttachCmd.Flags().String("kind", "", "script|log|image|audio|video|document|other (default: from the file name)")
	artifactsAttachCmd.Flags().String("note", "", "a caption")
	artifactsListCmd.Flags().String("task", "", "a ticket's files")
	artifactsListCmd.Flags().String("project", "", "a project's files (default: the project of the working directory)")
	artifactsListCmd.Flags().Bool("unfiled", false, "only files on no ticket")
	artifactsFetchCmd.Flags().String("to", "", "also copy the file here (must not exist)")
	artifactsCmd.AddCommand(artifactsAttachCmd, artifactsListCmd, artifactsFetchCmd, artifactsFileCmd,
		artifactsDiscardCmd, artifactsStatusCmd, artifactsSweepCmd)
	rootCmd.AddCommand(artifactsCmd)
}
