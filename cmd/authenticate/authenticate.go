package authenticate

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jsilland/sutro/config"
	"github.com/spf13/cobra"
	"golang.org/x/oauth2"
)

type authenticationFlags struct {
	clientID         string
	clientSecret     string
	authorizationURL string
	tokenURL         string
	scopes           []string
}

func Command(ctx context.Context, sink config.ConfigurationSink) *cobra.Command {
	flags := authenticationFlags{}

	command := &cobra.Command{
		Use:   "authenticate",
		Short: "Authentication support",
		RunE: func(cmd *cobra.Command, args []string) error {
			return authenticate(ctx, sink, flags)
		},
	}

	command.PersistentFlags().StringVar(&flags.clientID, "client_id", "", "The OAuth client ID")
	command.MarkPersistentFlagRequired("client_id")
	command.PersistentFlags().StringVar(&flags.clientSecret, "client_secret", "", "The OAuth client secret")
	command.MarkPersistentFlagRequired("client_secret")
	command.PersistentFlags().StringVar(&flags.authorizationURL, "authorization_url", "", "The authorization URL")
	command.MarkPersistentFlagRequired("authorization_url")
	command.PersistentFlags().StringVar(&flags.tokenURL, "token_url", "", "The token URL")
	command.MarkPersistentFlagRequired("token_url")
	command.PersistentFlags().StringSliceVar(&flags.scopes, "scopes", []string{}, "The scopes to request")

	return command
}

func authenticate(ctx context.Context, sink config.ConfigurationSink, flags authenticationFlags) error {
	oAuthCodeChannel := make(chan string)
	redirectService, err := NewOAuthRedirectService(oAuthCodeChannel)
	if err != nil {
		return err
	}
	defer redirectService.Shutdown(ctx)

	oAuthConfig := oauth2.Config{
		ClientID:     flags.clientID,
		ClientSecret: flags.clientSecret,
		Endpoint: oauth2.Endpoint{
			AuthURL:  flags.authorizationURL,
			TokenURL: flags.tokenURL,
		},
		RedirectURL: redirectService.RedirectURL().String(),
		Scopes:      flags.scopes,
	}

	url := oAuthConfig.AuthCodeURL(
		redirectService.State(),
		oauth2.AccessTypeOffline,
		oauth2.SetAuthURLParam("scope", "activity:read_all,profile:read_all,read_all"),
		oauth2.SetAuthURLParam("scope", strings.Join(flags.scopes, ",")),
	)

	fmt.Printf("Sutro needs to obtain your consent to access your data, which requires going to the following URL: %s\n", url)
	openInBrowser, err := promptBoolean("Do you want to open it your default browser?")

	if openInBrowser {
		err = openBrowser(url)
		if err != nil {
			return err
		}
	} else {
		fmt.Println("Alright. Please open the URL yourself and come back here after, we'll hang tight…")
	}

	code := <-oAuthCodeChannel

	if code == "" {
		return errors.New("Failed to obtain code from authenticate service")
	}

	token, err := oAuthConfig.Exchange(
		ctx,
		code,
		oauth2.SetAuthURLParam("client_id", oAuthConfig.ClientID),
		oauth2.SetAuthURLParam("client_secret", oAuthConfig.ClientSecret),
	)
	if err != nil {
		return err
	}

	fmt.Println("The authentication was successful, saving the config")

	return sink.Save(ctx, config.NewConfiguration(oAuthConfig, *token))
}

func openBrowser(url string) error {
	var err error

	switch runtime.GOOS {
	case "linux":
		err = exec.Command("xdg-open", url).Start()
	case "windows":
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		err = exec.Command("open", url).Start()
	default:
		fmt.Printf("Unable to open a browser - please open the URL yourself and follow the prompt: %s\n", url)
	}
	return err
}

func getFreeTCPPort() (int, error) {
	addr, err := net.ResolveTCPAddr("tcp", "localhost:0")
	if err != nil {
		return 0, err
	}

	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

type oAuthHTTPHandler struct {
	codeChannel chan string
	state       string
}

func (handler *oAuthHTTPHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	code := request.URL.Query().Get("code")
	state := request.URL.Query().Get("state")

	if handler.state != state {
		writer.WriteHeader(http.StatusBadRequest)
		writer.Header().Add("Content-Type", "text/plain; charset=utf-8")
		defer writer.Write([]byte("The returned state does not match the one set for this redirect service."))
		close(handler.codeChannel)
	}

	writer.WriteHeader(http.StatusOK)
	writer.Header().Add("Content-Type", "text/plain; charset=utf-8")
	defer writer.Write([]byte("Code successfully received, you can close this tab and go back to your terminal"))

	handler.codeChannel <- code
}

// OAuthRedirectService is a service that implements the second leg of
// a three-legged OAuth flow by running an ephemeral HTTP server and
// crafting a unique redirect URL to be passed to the authorization
// request.
type OAuthRedirectService interface {
	Shutdown(context.Context) error
	RedirectURL() *url.URL
	State() string
}

type oAuthRedirectService struct {
	server      *http.Server
	state       string
	redirectURL *url.URL
}

func (oars *oAuthRedirectService) RedirectURL() *url.URL {
	return oars.redirectURL
}

func (oars *oAuthRedirectService) State() string {
	return oars.state
}

func (oars *oAuthRedirectService) Shutdown(ctx context.Context) error {
	return oars.server.Shutdown(ctx)
}

func NewOAuthRedirectService(codeChannel chan string) (OAuthRedirectService, error) {
	port, err := getFreeTCPPort()
	if err != nil {
		return nil, err
	}

	redirectURL := &url.URL{
		Scheme: "http",
		Host:   fmt.Sprintf("localhost:%d", port),
		Path:   "/exchange",
	}

	state, err := uuid.NewUUID()
	if err != nil {
		return nil, err
	}

	router := http.NewServeMux()
	router.Handle(redirectURL.Path, &oAuthHTTPHandler{
		codeChannel: codeChannel,
		state:       state.String(),
	})

	server := &http.Server{
		Addr:           fmt.Sprintf(":%d", port),
		Handler:        router,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	go server.ListenAndServe()

	return &oAuthRedirectService{
		server:      server,
		redirectURL: redirectURL,
		state:       state.String(),
	}, nil
}

func promptBoolean(prompt string) (bool, error) {
	fmt.Printf("%s (yes/no):", prompt)
	return promptBooleanOnce(3)
}

func promptBooleanOnce(remainingAttempts int) (bool, error) {
	if remainingAttempts == 0 {
		return false, errors.New("Failed to obtain result from prompt")
	}

	var result string

	_, err := fmt.Scan(&result)
	if err != nil {
		return false, err
	}

	result = strings.TrimSpace(result)
	result = strings.ToLower(result)

	switch result {
	case "yes":
		return true, nil
	case "no":
		return false, nil
	default:
		fmt.Print("Please enter 'yes' or 'no': ")
		remainingAttempts--
		return promptBooleanOnce(remainingAttempts)
	}
}
