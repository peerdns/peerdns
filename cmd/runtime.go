package cmd

import (
	"context"
	"github.com/peerdns/peerdns/pkg/config"
	"github.com/peerdns/peerdns/pkg/logger"
	"github.com/peerdns/peerdns/pkg/observability"
	"github.com/peerdns/peerdns/pkg/resources"
	"github.com/peerdns/peerdns/pkg/runtime"
	"github.com/peerdns/peerdns/pkg/shutdown"
	_ "github.com/peerdns/peerdns/services/chain"
	_ "github.com/peerdns/peerdns/services/consensus"
	_ "github.com/peerdns/peerdns/services/dns"
	_ "github.com/peerdns/peerdns/services/gateway"
	_ "github.com/peerdns/peerdns/services/health"
	_ "github.com/peerdns/peerdns/services/router"
	_ "github.com/peerdns/peerdns/services/sequencer"
	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"
	"go.uber.org/zap"
	"os"
	"os/signal"
)

func RuntimeCommand() *cli.Command {
	return &cli.Command{
		Name:  "runtime",
		Usage: "Manage PeerDNS runtime a.k.a. services",
		Subcommands: []*cli.Command{
			{
				Name:  "run",
				Usage: "Run one or many services",
				Flags: []cli.Flag{
					&cli.StringSliceFlag{
						Name:     "services",
						Usage:    "...",
						Required: false,
						Aliases:  []string{"s"},
						Value: cli.NewStringSlice(
							runtime.ChainServiceType.String(),
							runtime.ConsensusServiceType.String(),
							runtime.RouterServiceType.String(),
							runtime.DNSServiceType.String(),
							runtime.HealthServiceType.String(),
							runtime.GatewayServiceType.String(),
							runtime.SequencerServiceType.String(),
						),
					},
				},
				Action: func(cmd *cli.Context) error {
					gLog := logger.G()

					ctx, cancel := context.WithCancel(cmd.Context)

					// Handle OS interrupt signals for graceful shutdown
					signalChan := make(chan os.Signal, 1)
					signal.Notify(signalChan, os.Interrupt)

					// Create a shutdown manager. This manager is used to gracefully shut down
					// services. We need to be extremely careful to process records
					// prior to shut down. As well as traffic management, where the system cannot close the service
					// prior to every connection being routed towards different destination (DPoP).
					// NOTE: Care about urfave context but if it becomes an issue, drop it - at runtime, relevant as irrelevant.
					shutdownManager := shutdown.NewManager(ctx, gLog)
					shutdownManager.Start()

					// Initialize metrics and tracing system (opentelemetry && prometheus)
					obs, err := observability.Initialize(ctx, config.G(), gLog)
					if err != nil {
						gLog.Fatal("Failed to initialize observability", zap.Error(err))
					}

					// Do not like dependency injection systems where you cannot find where lie tail where head.
					// Therefore, following is initialization of peerdns version of dependency injection system.
					// Initialize the global resource manager.
					if _, err := resources.InitializeManager(obs); err != nil {
						cancel()
						return errors.Wrap(err, "failure to initialize global resource manager")
					}

					// At this moment we need to construct and load our base service which can be
					// reused at any point in time...
					baseService, bsErr := runtime.InitializeBaseService(shutdownManager.Context(), config.G(), gLog, resources.G(), obs, shutdownManager)
					if bsErr != nil {
						cancel()
						return errors.Wrap(bsErr, "failure to initialize runtime base service")
					}

					services := make([]runtime.ServiceType, 0)
					for _, s := range cmd.StringSlice("services") {
						services = append(services, runtime.ServiceType(s))
					}

					// Service dependency injection... Each service is preloaded above look at _
					// There is init inside that triggers service registration ({service}/entrypoint.go)
					// Chosen this mechanism because it is clean and modular.
					// By modular, any developer can easily extend runtime by additional services while
					// maintaining sanity while loading default or integrated services.
					if err := runtime.InjectServices(shutdownManager.Context(), baseService); err != nil {
						cancel()
						return errors.Wrap(err, "failure to inject runtime service dependencies")
					}

					// Start the runtime that is going to look into the services, report any errors while bootstrapping
					// and if not, just run the services that are started.
					// NOTE: Pay close attention to how errors are triggered within each service. This process might
					// need to be extended in the future as some services should gracefully restart, some return error.
					// QUESTION: Should graceful restart be part of the system such as systemd or docker alone?
					go func() {
						if err := baseService.Start(services...); err != nil {
							close(signalChan)
							cancel()
							gLog.Error(
								"failure to start runtime service",
								zap.Error(err),
							)
						}
					}()

					// Wait for interrupt signal before processing with global system shutdown mechanisms.
					<-signalChan
					gLog.Info("Received interrupt signal, initiating shutdown...")

					// Cancel the context to signal all goroutines to shut down
					cancel()

					// Wait for shutdown to complete
					shutdownManager.Wait()

					gLog.Info("Runtime exited")
					return nil
				},
			},
		},
	}
}
