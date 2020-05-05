package config

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/user"
	"path"
	"strings"

	"golang.org/x/oauth2"
)

type ConfigurationSource interface {
	Get() (Configuration, error)
}

type ConfigurationSink interface {
	Save(context.Context, Configuration) error
}

type ConfigurationBridge interface {
	ConfigurationSource
	ConfigurationSink
}

func NewDotFileConfiguration(filename string) (ConfigurationBridge, error) {
	if !strings.HasPrefix(filename, ".") {
		filename = fmt.Sprintf(".%s", filename)
	}

	u, err := user.Current()

	if err != nil {
		return nil, err
	}

	return &fileConfiguration{path.Join(u.HomeDir, filename)}, nil
}

type fileConfiguration struct {
	path string
}

func (fcs *fileConfiguration) Get() (Configuration, error) {
	fileInfo, err := os.Stat(fcs.path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if fileInfo.IsDir() {
		return nil, errors.New(fmt.Sprintf("Unable to read configuration file at %s", fcs.path))
	}

	file, err := os.Open(fcs.path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	bytes, err := ioutil.ReadAll(file)
	if err != nil {
		return nil, err
	}

	var config configuration
	err = json.Unmarshal(bytes, &config)
	if err != nil {
		return nil, err
	}
	return &config, nil
}

func (fcs *fileConfiguration) Save(ctx context.Context, c Configuration) error {
	token, err := c.TokenSource(ctx).Token()
	if err != nil {
		return err
	}
	oAuthConfig := c.OAuthConfiguration()

	persistentConfiguration := configuration{
		ClientID:     oAuthConfig.ClientID,
		ClientSecret: oAuthConfig.ClientSecret,
		Endpoints: endpoints{
			AuthURL:  oAuthConfig.Endpoint.AuthURL,
			TokenURL: oAuthConfig.Endpoint.TokenURL,
		},
		Token: *token,
	}

	file, err := os.OpenFile(fcs.path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}

	bytes, err := json.MarshalIndent(persistentConfiguration, "", "  ")
	if err != nil {
		return err
	}

	_, err = file.Write(bytes)
	return err
}

func NewConfiguration(oAuthConfiguration oauth2.Config, token oauth2.Token) Configuration {
	return &configuration{
		ClientID:     oAuthConfiguration.ClientID,
		ClientSecret: oAuthConfiguration.ClientSecret,
		Endpoints: endpoints{
			AuthURL:  oAuthConfiguration.Endpoint.AuthURL,
			TokenURL: oAuthConfiguration.Endpoint.TokenURL,
		},
		Token: token,
	}
}

type Configuration interface {
	OAuthConfiguration() *oauth2.Config
	TokenSource(context.Context) oauth2.TokenSource
}

type configuration struct {
	ClientID     string       `json:"client_id"`
	ClientSecret string       `json:"client_secret"`
	Endpoints    endpoints    `json:"endpoints"`
	Token        oauth2.Token `json:"token"`
}

type endpoints struct {
	AuthURL  string `json:"auth_url"`
	TokenURL string `json:"token_url"`
}

func (c *configuration) OAuthConfiguration() *oauth2.Config {
	return &oauth2.Config{
		ClientID:     c.ClientID,
		ClientSecret: c.ClientSecret,
		Endpoint: oauth2.Endpoint{
			AuthURL:  c.Endpoints.AuthURL,
			TokenURL: c.Endpoints.TokenURL,
		},
	}
}

func (c *configuration) TokenSource(ctx context.Context) oauth2.TokenSource {
	return c.OAuthConfiguration().TokenSource(ctx, &c.Token)
}
