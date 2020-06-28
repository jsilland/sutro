package main

import (
	"context"
	"fmt"
	"net/http"
	"os"

	runtimeClient "github.com/go-openapi/runtime/client"
	"github.com/jsilland/sutro/client"
	"github.com/jsilland/sutro/cmd/authenticate"
	"github.com/jsilland/sutro/config"
	"github.com/spf13/cobra"
	"golang.org/x/oauth2"
)

//go:generate swagger generate client -f swagger.json -t . --template-dir=go-swagger-cli/templates --allow-template-override -C go-swagger-cli/config.yml

type globalFlags struct {
	verbose bool
}

func main() {
	flags := globalFlags{}

	ctx := context.Background()
	bridge, err := config.NewDotFileConfiguration("sutro")

	if err != nil {
		fmt.Errorf(err.Error())
		os.Exit(-1)
	}

	config, err := bridge.Get()

	if err != nil {
		fmt.Errorf(err.Error())
		os.Exit(-2)
	}

	command := &cobra.Command{}
	if config != nil {
		httpClient := oauth2.NewClient(ctx, config.TokenSource(ctx))
		transportConfig := client.DefaultTransportConfig()
		runtime := runtimeClient.NewWithClient(
			transportConfig.Host,
			transportConfig.BasePath,
			transportConfig.Schemes,
			httpClient,
		)
		apiClient := client.New(runtime, nil)

		command = client.NewCommand(apiClient)

		command.PersistentPreRun = func(cmd *cobra.Command, args []string) {
			if flags.verbose {
				httpClient.Transport = &verboseTransport{httpClient.Transport}
			}
		}
	}
	command.AddCommand(authenticate.Command(ctx, bridge))

	command.PersistentFlags().BoolVarP(&flags.verbose, "verbose", "v", false, "verbose output")

	command.Use = "sutro"
	command.PersistentPostRunE = func(cmd *cobra.Command, args []string) error {
		if cmd.Name() == "authenticate" {
			return nil
		}

		return bridge.Save(ctx, config)
	}

	_, err = command.ExecuteC()

	if err != nil {
		_ = fmt.Errorf(err.Error())
		os.Exit(-3)
	}
}

type verboseTransport struct {
	http.RoundTripper
}

func (vt *verboseTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	fmt.Fprintf(os.Stdout, "%s %s\n", request.Method, request.URL.String())
	for header, values := range request.Header {
		for _, value := range values {
			fmt.Fprintf(os.Stdout, "%s: %s\n", header, value)
		}
	}
	response, err := vt.RoundTripper.RoundTrip(request)
	return response, err
}
