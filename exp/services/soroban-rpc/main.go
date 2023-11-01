package main

import (
	"fmt"
	"go/types"
	"net/http"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/stellar/go/clients/horizonclient"
	"github.com/stellar/go/exp/services/soroban-rpc/internal"
	"github.com/stellar/go/exp/services/soroban-rpc/internal/methods"
	"github.com/stellar/go/support/config"
	supporthttp "github.com/stellar/go/support/http"
	supportlog "github.com/stellar/go/support/log"
)

func main() {
	var port int
	var horizonURL string
	var logLevel logrus.Level
	logger := supportlog.New()

	configOpts := config.ConfigOptions{
		{
			Name:        "port",
			Usage:       "Port to listen and serve on",
			OptType:     types.Int,
			ConfigKey:   &port,
			FlagDefault: 8000,
			Required:    true,
		},
		&config.ConfigOption{
			Name:        "horizon-url",
			ConfigKey:   &horizonURL,
			OptType:     types.String,
			Required:    true,
			FlagDefault: "",
			Usage:       "URL used to query Horizon",
		},
		&config.ConfigOption{
			Name:        "log-level",
			ConfigKey:   &logLevel,
			OptType:     types.String,
			FlagDefault: "info",
			CustomSetValue: func(co *config.ConfigOption) error {
				ll, err := logrus.ParseLevel(viper.GetString(co.Name))
				if err != nil {
					return fmt.Errorf("Could not parse log-level: %v", viper.GetString(co.Name))
				}
				*(co.ConfigKey.(*logrus.Level)) = ll
				return nil
			},
			Usage: "minimum log severity (debug, info, warn, error) to log",
		},
	}
	cmd := &cobra.Command{
		Use:   "soroban-rpc",
		Short: "Run the remote soroban-rpc server",
		Run: func(_ *cobra.Command, _ []string) {
			configOpts.Require()
			configOpts.SetValues()
			logger.SetLevel(logLevel)

			hc := &horizonclient.Client{
				HorizonURL: horizonURL,
				HTTP: &http.Client{
					Timeout: horizonclient.HorizonTimeout,
				},
				AppName: "Soroban RPC",
			}
			hc.SetHorizonTimeout(horizonclient.HorizonTimeout)

			handler, err := internal.NewJSONRPCHandler(internal.HandlerParams{
				AccountStore: methods.AccountStore{Client: hc},
				Logger:       logger,
			})
			if err != nil {
				logger.Fatalf("could not create handler: %v", err)
			}
			supporthttp.Run(supporthttp.Config{
				ListenAddr: fmt.Sprintf(":%d", port),
				Handler:    handler,
				OnStarting: func() {
					logger.Infof("Starting Soroban JSON RPC server on %v", port)
				},
				OnStopping: func() {
					handler.Close()
				},
			})
		},
	}

	if err := configOpts.Init(cmd); err != nil {
		logger.WithError(err).Fatal("could not parse config options")
	}

	if err := cmd.Execute(); err != nil {
		logger.WithError(err).Fatal("could not run")
	}
}
