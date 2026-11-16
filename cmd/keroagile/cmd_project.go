package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tbdtechpro/KeroAgile/internal/domain"
	"github.com/tbdtechpro/KeroAgile/internal/git"
)

var projectCmd = &cobra.Command{Use: "project", Short: "Manage projects"}

var projectAddCmd = &cobra.Command{
	Use:   "add <id> <name>",
	Short: "Create a project",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		repo, _ := cmd.Flags().GetString("repo")
		if err := svc.CreateProject(args[0], args[1], repo); err != nil {
			return err
		}
		p, _ := svc.GetProject(args[0])
		if jsonFlag {
			printJSON(p)
		} else {
			fmt.Printf("created project %s (%s)\n", p.ID, p.Name)
		}
		return nil
	},
}

var projectListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all projects",
	RunE: func(cmd *cobra.Command, args []string) error {
		projects, err := svc.ListProjects()
		if err != nil {
			return err
		}
		if jsonFlag {
			printJSON(projects)
			return nil
		}
		for _, p := range projects {
			sprint := ""
			if p.SprintMode {
				sprint = " [sprints]"
			}
			where := p.RepoDir
			if p.RepoRemote != "" {
				where = strings.TrimSpace(where + "  " + p.RepoRemote)
			}
			fmt.Printf("  %s  %s%s  %s\n", p.ID, p.Name, sprint, where)
		}
		return nil
	},
}

var projectSprintCmd = &cobra.Command{
	Use:   "set-sprint <project-id> on|off",
	Short: "Enable or disable sprint mode for a project",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		enabled := args[1] == "on"
		if err := svc.SetSprintMode(args[0], enabled); err != nil {
			return err
		}
		if jsonFlag {
			p, err := svc.GetProject(args[0])
			if err != nil {
				return err
			}
			printJSON(p)
		} else {
			fmt.Printf("sprint mode %s for %s\n", args[1], args[0])
		}
		return nil
	},
}

var projectSetCmd = &cobra.Command{
	Use:   "set <project-id>",
	Short: "Set a project's directory, remote or file policy",
	Long: `Set where a project's code lives and what happens to files kept for its tickets.

  --dir              the local checkout; sessions working inside it belong to the project
  --remote           the forge repository (github.com/owner/repo or a clone URL);
                     when --dir is given and the project has no remote, it is read
                     from the checkout's origin
  --artifact-policy  remote (keep and upload), local (keep here), off (do not capture),
                     or default to follow [artifacts] default_policy

Give an empty value to clear --dir or --remote.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var p domain.ProjectPatch
		if cmd.Flags().Changed("dir") {
			dir, _ := cmd.Flags().GetString("dir")
			if dir != "" {
				if abs, err := filepath.Abs(dir); err == nil {
					dir = abs
				}
			}
			p.RepoDir = domain.To(dir)
			if !cmd.Flags().Changed("remote") && dir != "" {
				if cur, err := svc.GetProject(args[0]); err == nil && cur.RepoRemote == "" {
					if url, err := git.RemoteURL(dir); err == nil && url != "" {
						p.RepoRemote = domain.To(url)
					}
				}
			}
		}
		if cmd.Flags().Changed("remote") {
			remote, _ := cmd.Flags().GetString("remote")
			p.RepoRemote = domain.To(remote)
		}
		if cmd.Flags().Changed("artifact-policy") {
			policy, _ := cmd.Flags().GetString("artifact-policy")
			if policy == "default" {
				policy = ""
			}
			p.ArtifactPolicy = domain.To(domain.ArtifactPolicy(policy))
		}
		proj, err := svc.PatchProject(args[0], p)
		if err != nil {
			return err
		}
		if jsonFlag {
			printJSON(proj)
		} else {
			fmt.Printf("%s  dir %q  remote %q  policy %q\n", proj.ID, proj.RepoDir, proj.RepoRemote, proj.ArtifactPolicy)
		}
		return nil
	},
}

var projectWhichCmd = &cobra.Command{
	Use:   "which",
	Short: "Show which project a directory belongs to",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, _ := cmd.Flags().GetString("dir")
		p, err := projectForDir(dir)
		if err != nil {
			return err
		}
		if jsonFlag {
			printJSON(p)
		} else {
			fmt.Printf("%s  %s\n", p.ID, p.Name)
		}
		return nil
	},
}

// projectForDir finds the project dir belongs to (the working directory when
// dir is empty), by directory first and then by the checkout's origin.
func projectForDir(dir string) (*domain.Project, error) {
	if dir == "" {
		wd, err := os.Getwd()
		if err != nil {
			return nil, err
		}
		dir = wd
	}
	if abs, err := filepath.Abs(dir); err == nil {
		dir = abs
	}
	remote, _ := git.RemoteURL(dir)
	home, _ := os.UserHomeDir()
	return svc.ProjectForDir(dir, remote, home)
}

func init() {
	projectAddCmd.Flags().String("repo", "", "the project's local directory, or its git remote")
	projectSetCmd.Flags().String("dir", "", "local checkout directory")
	projectSetCmd.Flags().String("remote", "", "forge repository, e.g. github.com/owner/repo")
	projectSetCmd.Flags().String("artifact-policy", "", "remote | local | off | default")
	projectWhichCmd.Flags().String("dir", "", "directory to look up (default: the working directory)")
	projectCmd.AddCommand(projectAddCmd, projectListCmd, projectSprintCmd, projectSetCmd, projectWhichCmd)
}
