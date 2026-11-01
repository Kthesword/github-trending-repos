// Package cmd implements the command-line interface: flag definitions,
// input validation, and orchestration of the API client and formatter.
package cmd

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"runtime/debug"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/kthesword/github-trending-repos/api"
	"github.com/kthesword/github-trending-repos/formatter"
	"github.com/kthesword/github-trending-repos/models"
)

const (
	appName = "trending-repos"

	defaultDuration = string(models.PeriodWeek)
	defaultLimit    = 10
	defaultTimeout  = api.DefaultTimeout

	// envToken optionally supplies a GitHub token to raise rate limits.
	envToken = "GITHUB_TOKEN"
	// envAPIURL optionally overrides the API root (e.g. GitHub Enterprise).
	// The name matches the variable GitHub Actions sets automatically.
	envAPIURL = "GITHUB_API_URL"
)

// version is injected at build time:
//
//	go build -ldflags "-X github.com/kthesword/github-trending-repos/cmd.version=v1.0.0"
var version = ""

// options holds the parsed command-line flags.
type options struct {
	duration string
	limit    int
	timeout  time.Duration
	noColor  bool
}

// Execute runs the root command and returns the process exit code.
func Execute() int {
	// Cancel in-flight HTTP requests cleanly when the user presses Ctrl+C.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	root := newRootCmd()
	// color.Output / color.Error translate ANSI sequences on legacy Windows
	// consoles and pass through unchanged everywhere else.
	root.SetOut(color.Output)
	root.SetErr(color.Error)

	if err := root.ExecuteContext(ctx); err != nil {
		return printError(color.Error, err)
	}
	return exitOK
}

// newRootCmd builds the root command. It is a constructor (rather than a
// package-level variable) so tests get a fresh, isolated instance.
func newRootCmd() *cobra.Command {
	opts := &options{}

	cmd := &cobra.Command{
		Use:   appName,
		Short: "Discover trending GitHub repositories from your terminal",
		Long: `trending-repos queries the GitHub Search API for public repositories
created within a recent time window and lists them ranked by star count.

No authentication is required. Unauthenticated clients may perform up to
10 searches per minute; set the GITHUB_TOKEN environment variable to a
personal access token to raise that limit.`,
		Example: `  # Top 10 repositories created in the past week (the defaults)
  trending-repos

  # Top 20 repositories created in the past month
  trending-repos --duration month --limit 20

  # Today's hottest projects, without colors (handy for piping)
  trending-repos -d day --no-color`,
		Version:       resolveVersion(),
		Args:          noPositionalArgs,
		SilenceUsage:  true, // runtime errors shouldn't dump the usage text
		SilenceErrors: true, // errors are printed by Execute in a friendly form
		RunE: func(cmd *cobra.Command, _ []string) error {
			return run(cmd.Context(), cmd.OutOrStdout(), opts)
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&opts.duration, "duration", "d", defaultDuration,
		fmt.Sprintf("time range to search (%s)", strings.Join(models.Periods(), ", ")))
	flags.IntVarP(&opts.limit, "limit", "l", defaultLimit,
		fmt.Sprintf("maximum number of repositories to display (1-%d)", api.MaxSearchResults))
	flags.DurationVar(&opts.timeout, "timeout", defaultTimeout,
		"timeout for each HTTP request to GitHub")
	flags.BoolVar(&opts.noColor, "no-color", false,
		"disable colorized output (also honors the NO_COLOR environment variable)")

	// Treat malformed flags (e.g. --limit abc, --unknown) as usage errors.
	cmd.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return &usageError{err: err}
	})

	return cmd
}

// run validates the options, fetches trending repositories, and renders them.
func run(ctx context.Context, out io.Writer, opts *options) error {
	if opts.noColor {
		color.NoColor = true // also affects error output printed later
	}

	period, err := models.ParsePeriod(opts.duration)
	if err != nil {
		return &usageError{err: err}
	}
	if opts.limit < 1 || opts.limit > api.MaxSearchResults {
		return usageErrorf("invalid limit %d: must be between 1 and %d", opts.limit, api.MaxSearchResults)
	}
	if opts.timeout <= 0 {
		return usageErrorf("invalid timeout %s: must be greater than zero", opts.timeout)
	}

	since := period.Since(time.Now())

	client := api.NewClient(
		api.WithBaseURL(os.Getenv(envAPIURL)),
		api.WithToken(os.Getenv(envToken)),
		api.WithUserAgent(appName+"/"+resolveVersion()),
		api.WithHTTPClient(&http.Client{Timeout: opts.timeout}),
	)

	result, err := client.SearchTrending(ctx, since, opts.limit)
	if err != nil {
		return err
	}

	table := formatter.NewTable(out, formatter.WithColor(!color.NoColor))
	return table.Render(formatter.Meta{
		Period:     period.Description(),
		Since:      since,
		TotalCount: result.TotalCount,
		Incomplete: result.Incomplete,
	}, result.Repositories)
}

// resolveVersion prefers the ldflags-injected version, then the module
// version recorded by `go install`, and finally falls back to "dev".
func resolveVersion() string {
	if version != "" {
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok {
		if v := info.Main.Version; v != "" && v != "(devel)" {
			return v
		}
	}
	return "dev"
}
