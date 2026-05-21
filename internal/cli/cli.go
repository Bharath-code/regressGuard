package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"

	"github.com/Bharath-code/regressguard/internal/checkrun"
	"github.com/Bharath-code/regressguard/internal/config"
	"github.com/Bharath-code/regressguard/internal/configrun"
	"github.com/Bharath-code/regressguard/internal/doctorrun"
	"github.com/Bharath-code/regressguard/internal/explainrun"
	"github.com/Bharath-code/regressguard/internal/failures"
	"github.com/Bharath-code/regressguard/internal/hookrun"
	"github.com/Bharath-code/regressguard/internal/initrun"
	"github.com/Bharath-code/regressguard/internal/mcprun"
	"github.com/Bharath-code/regressguard/internal/snapshotrun"
	"github.com/Bharath-code/regressguard/internal/statusrun"
	"github.com/Bharath-code/regressguard/internal/ui"
	"github.com/Bharath-code/regressguard/internal/upgraderun"
	"github.com/spf13/cobra"
)

type BuildInfo struct {
	Version string
	Commit  string
	Date    string
}

func Execute(build BuildInfo) error {
	root := NewRootCommand(build)
	if err := root.Execute(); err != nil {
		if !isSilentError(err) {
			stderr := root.ErrOrStderr()
			if issue, ok := err.(failures.Actionable); ok {
				// Colored actionable error.
				_, _ = fmt.Fprintln(stderr, ui.Paint(stderr, ui.ColorFail, issue.Title))
				_, _ = fmt.Fprintln(stderr)
				_, _ = fmt.Fprintln(stderr, "Likely cause:")
				_, _ = fmt.Fprintln(stderr, "  "+issue.Cause)
				_, _ = fmt.Fprintln(stderr)
				_, _ = fmt.Fprintln(stderr, "Run:")
				_, _ = fmt.Fprintln(stderr, "  "+ui.Paint(stderr, ui.ColorInfo, issue.Next))
				_, _ = fmt.Fprintln(stderr)
				_, _ = fmt.Fprintln(stderr, "Need more context:")
				_, _ = fmt.Fprintln(stderr, "  "+ui.Paint(stderr, ui.ColorInfo, issue.MoreContext))
			} else {
				_, _ = fmt.Fprintln(stderr, err)
			}
		}
		return err
	}
	return nil
}

func NewRootCommand(build BuildInfo) *cobra.Command {
	root := &cobra.Command{
		Use:           "rg",
		Short:         "Before you commit, know what broke.",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			// E12-T3: if in a configured project, show status + help.
			if config.Exists(".") {
				_, _ = statusrun.Run(statusrun.Options{
					ProjectRoot: ".",
					Stdout:      cmd.OutOrStdout(),
					Stderr:      cmd.ErrOrStderr(),
				})
				_, _ = fmt.Fprintln(cmd.OutOrStdout())
			}
			return cmd.Help()
		},
	}

	root.SetHelpTemplate(rootHelpTemplate())
	root.AddCommand(
		newInitCommand(),
		newSnapshotCommand(),
		newCheckCommand(),
		newExplainCommand(),
		newStatusCommand(),
		newHookCommand(),
		newConfigCommand(),
		newDoctorCommand(),
		newVersionCommand(build),
		newUpgradeCommand(build),
		newMCPCommand(build),
	)
	configureCompletionHelp(root)

	return root
}

func configureCompletionHelp(root *cobra.Command) {
	root.InitDefaultCompletionCmd()
	for _, cmd := range root.Commands() {
		if cmd.Name() != "completion" {
			continue
		}
		cmd.SetHelpTemplate(groupHelpTemplate("rg completion zsh"))
		for _, child := range cmd.Commands() {
			child.SetHelpTemplate(commandHelpTemplate("rg completion " + child.Name()))
		}
		return
	}
}

func newInitCommand() *cobra.Command {
	cmd := stubCommand("init", "Configure RegressGuard for this project", "rg init")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		serverURL, _ := cmd.Flags().GetString("server-url")
		testCommand, _ := cmd.Flags().GetString("test-command")
		yes, _ := cmd.Flags().GetBool("yes")
		jsonMode, _ := cmd.Flags().GetBool("json")
		forceInteractive, _ := cmd.Flags().GetBool("interactive")

		_, err := initrun.Run(initrun.Options{
			StartDir:         ".",
			ServerURL:        serverURL,
			TestCommand:      testCommand,
			Yes:              yes,
			JSON:             jsonMode,
			Interactive:      ui.IsTerminal(cmd.InOrStdin()) && ui.IsTerminal(cmd.OutOrStdout()),
			ForceInteractive: forceInteractive,
			Stdout:           cmd.OutOrStdout(),
			Stderr:           cmd.ErrOrStderr(),
			Stdin:            cmd.InOrStdin(),
		})
		if err != nil {
			if issue, ok := err.(failures.Actionable); ok && jsonMode {
				return writeActionable(cmd, issue)
			}
			return err
		}
		return nil
	}
	cmd.Flags().String("server-url", "", "dev server URL, for example http://localhost:3000")
	cmd.Flags().String("test-command", "", "override detected test command")
	cmd.Flags().Bool("yes", false, "overwrite existing config without prompting")
	cmd.Flags().Bool("json", false, "write machine-readable JSON to stdout")
	cmd.Flags().Bool("interactive", false, "force guided prompts")
	_ = cmd.Flags().MarkHidden("interactive")
	return cmd
}

func newSnapshotCommand() *cobra.Command {
	cmd := stubCommand("snapshot", "Record the current passing state", "rg snapshot")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		jsonMode, _ := cmd.Flags().GetBool("json")
		verbose, _ := cmd.Flags().GetBool("verbose")

		_, err := snapshotrun.Run(snapshotrun.Options{
			ProjectRoot: ".",
			JSON:        jsonMode,
			Verbose:     verbose,
			Stdout:      cmd.OutOrStdout(),
			Stderr:      cmd.ErrOrStderr(),
		})
		if err != nil {
			if issue, ok := err.(failures.Actionable); ok {
				if jsonMode {
					return writeActionable(cmd, issue)
				}
				return issue
			}
			return err
		}
		return nil
	}
	cmd.Flags().Bool("json", false, "write machine-readable JSON to stdout")
	cmd.Flags().Bool("verbose", false, "write diagnostics to stderr")
	return cmd
}

func newCheckCommand() *cobra.Command {
	cmd := stubCommand("check", "Compare current state against the snapshot", "rg check")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		jsonMode, _ := cmd.Flags().GetBool("json")
		verbose, _ := cmd.Flags().GetBool("verbose")
		since, _ := cmd.Flags().GetString("since")
		autoServer, _ := cmd.Flags().GetBool("auto-server")

		result, err := checkrun.Run(checkrun.Options{
			ProjectRoot: ".",
			JSON:        jsonMode,
			Verbose:     verbose,
			Since:       since,
			AutoServer:  autoServer,
			Stdout:      cmd.OutOrStdout(),
			Stderr:      cmd.ErrOrStderr(),
		})
		if err != nil {
			if issue, ok := err.(failures.Actionable); ok {
				if jsonMode {
					return writeActionable(cmd, issue)
				}
				return issue
			}
			return err
		}

		// Exit code 1 on critical regression (PRD AC E4-T4, Feature 3 AC4).
		if result.Status == "critical" {
			os.Exit(1)
		}
		return nil
	}
	cmd.Flags().Bool("json", false, "write machine-readable JSON to stdout")
	cmd.Flags().Bool("verbose", false, "write route and request diagnostics to stderr")
	cmd.Flags().String("since", "", "scope to routes changed since git ref (HEAD~1, main)")
	cmd.Flags().Bool("auto-server", false, "spawn dev server from config, run check, kill on exit")
	return cmd
}

func newExplainCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "explain <route>",
		Short: "Show before/after diff for a specific route",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			jsonMode, _ := cmd.Flags().GetBool("json")

			_, err := explainrun.Run(explainrun.Options{
				ProjectRoot: ".",
				Route:       args[0],
				JSON:        jsonMode,
				Stdout:      cmd.OutOrStdout(),
				Stderr:      cmd.ErrOrStderr(),
			})
			if err != nil {
				if issue, ok := err.(failures.Actionable); ok {
					if jsonMode {
						return writeActionable(cmd, issue)
					}
					return issue
				}
				return err
			}
			return nil
		},
	}
	cmd.SetHelpTemplate(commandHelpTemplate("rg explain \"GET /api/users\""))
	cmd.Flags().Bool("json", false, "write machine-readable JSON to stdout")
	return cmd
}

func newStatusCommand() *cobra.Command {
	cmd := stubCommand("status", "Quick health check — snapshot age, routes, hook status", "rg status")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		jsonMode, _ := cmd.Flags().GetBool("json")

		_, err := statusrun.Run(statusrun.Options{
			ProjectRoot: ".",
			JSON:        jsonMode,
			Stdout:      cmd.OutOrStdout(),
			Stderr:      cmd.ErrOrStderr(),
		})
		return err
	}
	cmd.Flags().Bool("json", false, "write machine-readable JSON to stdout")
	return cmd
}

func newHookCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "hook",
		Short: "Install or remove git hooks",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	cmd.SetHelpTemplate(groupHelpTemplate("rg hook install"))
	cmd.AddCommand(newHookInstallCommand())
	cmd.AddCommand(newHookUninstallCommand())
	return cmd
}

func newHookInstallCommand() *cobra.Command {
	cmd := stubCommand("install", "Install the pre-commit hook", "rg hook install")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		_, err := hookrun.Install(hookrun.InstallOptions{
			Stdout: cmd.OutOrStdout(),
			Stderr: cmd.ErrOrStderr(),
		})
		return err
	}
	return cmd
}

func newHookUninstallCommand() *cobra.Command {
	cmd := stubCommand("uninstall", "Remove the RegressGuard hook block", "rg hook uninstall")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return hookrun.Uninstall(hookrun.UninstallOptions{
			Stdout: cmd.OutOrStdout(),
			Stderr: cmd.ErrOrStderr(),
		})
	}
	return cmd
}

func newConfigCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "View or edit project config",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	cmd.SetHelpTemplate(groupHelpTemplate("rg config get serverUrl"))
	cmd.AddCommand(newConfigGetCommand())
	cmd.AddCommand(newConfigSetCommand())
	return cmd
}

func newConfigGetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get <key>",
		Short: "Read a config value",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			err := configrun.Get(args[0], ".", cmd.OutOrStdout())
			if err != nil {
				if issue, ok := err.(failures.Actionable); ok {
					return issue
				}
				return err
			}
			return nil
		},
	}
	cmd.SetHelpTemplate(commandHelpTemplate("rg config get serverUrl"))
	return cmd
}

func newConfigSetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set <key> <value>",
		Short: "Write a config value",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			err := configrun.Set(args[0], args[1], ".", cmd.OutOrStdout())
			if err != nil {
				if issue, ok := err.(failures.Actionable); ok {
					return issue
				}
				return err
			}
			return nil
		},
	}
	cmd.SetHelpTemplate(commandHelpTemplate("rg config set serverUrl http://localhost:3000"))
	return cmd
}

func newDoctorCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Diagnose setup issues",
		RunE: func(cmd *cobra.Command, args []string) error {
			ok := doctorrun.Run(doctorrun.Options{
				ProjectRoot: ".",
				Stdout:      cmd.OutOrStdout(),
				Stderr:      cmd.ErrOrStderr(),
			})
			if !ok {
				return fmt.Errorf("")
			}
			return nil
		},
	}
	cmd.SetHelpTemplate(commandHelpTemplate("rg doctor"))
	return cmd
}

func newVersionCommand(build BuildInfo) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print version and build metadata",
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			_, err := fmt.Fprintf(out, "rg %s\ncommit: %s\nbuild date: %s\nos/arch: %s/%s\n",
				build.Version, build.Commit, build.Date, runtime.GOOS, runtime.GOARCH)
			return err
		},
	}
	cmd.SetHelpTemplate(commandHelpTemplate("rg version"))
	return cmd
}

func newUpgradeCommand(build BuildInfo) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "upgrade",
		Short: "Update rg to the latest version",
		RunE: func(cmd *cobra.Command, args []string) error {
			checkOnly, _ := cmd.Flags().GetBool("check")

			_, err := upgraderun.Run(upgraderun.Options{
				CurrentVersion: build.Version,
				CheckOnly:      checkOnly,
				Stdout:         cmd.OutOrStdout(),
				Stderr:         cmd.ErrOrStderr(),
			})
			if err != nil {
				if issue, ok := err.(failures.Actionable); ok {
					return issue
				}
				return err
			}
			return nil
		},
	}
	cmd.SetHelpTemplate(commandHelpTemplate("rg upgrade"))
	cmd.Flags().Bool("check", false, "check for updates without installing")
	return cmd
}

func newMCPCommand(build BuildInfo) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mcp",
		Short: "MCP server for AI coding agents",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	cmd.SetHelpTemplate(groupHelpTemplate("rg mcp serve"))
	cmd.AddCommand(newMCPServeCommand(build))
	return cmd
}

func newMCPServeCommand(build BuildInfo) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start MCP server on stdio (for AI agents)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return mcprun.Serve(mcprun.Options{
				Version:     build.Version,
				ProjectRoot: ".",
			})
		},
	}
	cmd.SetHelpTemplate(commandHelpTemplate("rg mcp serve"))
	return cmd
}

func stubCommand(use, short, example string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			_, err := fmt.Fprintf(out, "%s is not implemented yet.\n\nNext:\n  %s --help\n", use, example)
			return err
		},
	}
	cmd.SetHelpTemplate(commandHelpTemplate(example))
	return cmd
}

func jsonAwareStub(name, nextCommand string) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		jsonMode, _ := cmd.Flags().GetBool("json")
		verbose, _ := cmd.Flags().GetBool("verbose")

		if verbose {
			if _, err := fmt.Fprintf(cmd.ErrOrStderr(), "INFO %s engine not implemented yet.\n", name); err != nil {
				return err
			}
		}

		if jsonMode {
			payload := map[string]any{
				"status": "not_implemented",
				"summary": map[string]int{
					"critical": 0,
					"warnings": 0,
					"passed":   0,
				},
				"results": []any{},
				"next":    nextCommand,
			}
			encoder := json.NewEncoder(cmd.OutOrStdout())
			encoder.SetIndent("", "  ")
			return encoder.Encode(payload)
		}

		_, err := fmt.Fprintf(cmd.OutOrStdout(), "%s is not implemented yet.\n\nNext:\n  %s\n", name, nextCommand)
		return err
	}
}

type silentError struct{}

func (silentError) Error() string { return "" }

func isSilentError(err error) bool {
	_, ok := err.(silentError)
	return ok
}

func writeActionable(cmd *cobra.Command, issue failures.Actionable) error {
	jsonMode, _ := cmd.Flags().GetBool("json")
	if jsonMode {
		payload := map[string]any{
			"status": "error",
			"error":  issue,
		}
		encoder := json.NewEncoder(cmd.OutOrStdout())
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(payload); err != nil {
			return err
		}
		return silentError{}
	}
	return issue
}

func rootHelpTemplate() string {
	return strings.TrimLeft(`
RegressGuard
Before you commit, know what broke.

Commands:
  init       Configure RegressGuard for this project
  snapshot   Record the current passing state
  check      Compare current state against the snapshot
  status     Quick health check (sub-second)
  hook       Install or remove git hooks
  config     View or edit project config
  doctor     Diagnose setup issues

Start:
  rg init
`, "\n")
}

func commandHelpTemplate(example string) string {
	return strings.TrimLeft(fmt.Sprintf(`
Usage:
  {{.UseLine}}

Purpose:
  {{.Short}}

Examples:
  %s

Exit codes:
  0  pass or warnings only
  1  critical regression
  2  usage, config, or runtime error

Agent guidance:
  Prefer --help over hardcoded command knowledge.
{{if .HasAvailableFlags}}
Flags:
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}
`, example), "\n")
}

func groupHelpTemplate(example string) string {
	return strings.TrimLeft(fmt.Sprintf(`
Usage:
  {{.UseLine}}

Purpose:
  {{.Short}}

Commands:
{{range .Commands}}{{if (or .IsAvailableCommand (eq .Name "help"))}}  {{rpad .Name .NamePadding }} {{.Short}}
{{end}}{{end}}
Examples:
  %s

Exit codes:
  0  pass or warnings only
  1  critical regression
  2  usage, config, or runtime error
`, example), "\n")
}

func WriteHelp(w io.Writer, cmd *cobra.Command) error {
	cmd.SetOut(w)
	return cmd.Help()
}
