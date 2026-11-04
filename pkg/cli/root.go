package cli

import (
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/obot-platform/cmd"
	"github.com/obot-platform/obot/apiclient"
	"github.com/obot-platform/obot/logger"
	"github.com/obot-platform/obot/pkg/cli/internal"
	"github.com/obot-platform/obot/pkg/cli/internal/localconfig"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

type Uni struct {
	Debug  bool `usage:"Enable debug logging"`
	Client *apiclient.Client
}

func (a *Uni) PersistentPre(*cobra.Command, []string) error {
	if os.Getenv("NO_COLOR") != "" || !term.IsTerminal(int(os.Stdout.Fd())) {
		color.NoColor = true
	}

	// Runs for every subcommand, after all package initialization, so this wins
	// over any handler a dependency installed from its init.
	logger.Setup(logger.Text)

	if a.Debug {
		logger.SetDebug()
	}

	if a.Client.Token == "" {
		a.Client = a.Client.WithTokenFetcher(internal.Token)
	}

	return nil
}

func New() *cobra.Command {
	root := &Uni{
		Client: newClient(),
	}
	return cmd.Command(root,
		&Server{},
		&Login{root: root},
		&Logout{root: root},
		&MCP{root: root},
		&Setup{root: root},
		&Skills{root: root},
		&Tunnel{},
		&Version{},
		&Daemon{},
	)
}

func (a *Uni) Run(cmd *cobra.Command, _ []string) error {
	return cmd.Help()
}

func newClient() *apiclient.Client {
	baseURL := os.Getenv("UNI_BASE_URL")
	if baseURL != "" {
		if appURL, err := internal.AppURLForAPIBaseURL(baseURL); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: invalid UNI_BASE_URL: %v\n", err)
			baseURL = ""
		} else {
			baseURL = localconfig.APIBaseURL(appURL)
		}
	}
	if baseURL == "" {
		if cfg, err := localconfig.Load(); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to load uni config: %v\n", err)
		} else if cfg.DefaultURL != "" {
			baseURL = localconfig.APIBaseURL(cfg.DefaultURL)
		}
	}
	if baseURL == "" {
		baseURL = "http://localhost:8080/api"
	}

	return &apiclient.Client{
		BaseURL: baseURL,
		Token:   os.Getenv(internal.TokenEnvVar),
	}
}
