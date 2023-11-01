package cmd

import (
	"context"
	"database/sql"
	"fmt"
	"go/types"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/stellar/go/services/horizon/internal/db2/history"

	horizon "github.com/stellar/go/services/horizon/internal"
	"github.com/stellar/go/services/horizon/internal/db2/schema"
	"github.com/stellar/go/services/horizon/internal/ingest"
	support "github.com/stellar/go/support/config"
	"github.com/stellar/go/support/db"
	"github.com/stellar/go/support/errors"
	hlog "github.com/stellar/go/support/log"
)

var dbCmd = &cobra.Command{
	Use:   "db [command]",
	Short: "commands to manage horizon's postgres db",
}

var dbMigrateCmd = &cobra.Command{
	Use:   "migrate [command]",
	Short: "commands to run schema migrations on horizon's postgres db",
}

func requireAndSetFlags(names ...string) error {
	set := map[string]bool{}
	for _, name := range names {
		set[name] = true
	}
	for _, flag := range flags {
		if set[flag.Name] {
			flag.Require()
			if err := flag.SetValue(); err != nil {
				return err
			}
			delete(set, flag.Name)
		}
	}
	if len(set) == 0 {
		return nil
	}
	var missing []string
	for name := range set {
		missing = append(missing, name)
	}
	return fmt.Errorf("could not find %s flags", strings.Join(missing, ","))
}

var dbInitCmd = &cobra.Command{
	Use:   "init",
	Short: "install schema",
	Long:  "init initializes the postgres database used by horizon.",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireAndSetFlags(horizon.DatabaseURLFlagName, horizon.IngestFlagName); err != nil {
			return err
		}

		db, err := sql.Open("postgres", config.DatabaseURL)
		if err != nil {
			return err
		}

		numMigrationsRun, err := schema.Migrate(db, schema.MigrateUp, 0)
		if err != nil {
			return err
		}

		if numMigrationsRun == 0 {
			log.Println("No migrations applied.")
		} else {
			log.Printf("Successfully applied %d migrations.\n", numMigrationsRun)
		}
		return nil
	},
}

func migrate(dir schema.MigrateDir, count int) error {
	if !config.Ingest {
		log.Println("Skipping migrations because ingest flag is not enabled")
		return nil
	}

	dbConn, err := db.Open("postgres", config.DatabaseURL)
	if err != nil {
		return err
	}

	numMigrationsRun, err := schema.Migrate(dbConn.DB.DB, dir, count)
	if err != nil {
		return err
	}

	if numMigrationsRun == 0 {
		log.Println("No migrations applied.")
	} else {
		log.Printf("Successfully applied %d migrations.\n", numMigrationsRun)
	}
	return nil
}

var dbMigrateDownCmd = &cobra.Command{
	Use:   "down COUNT",
	Short: "run downwards db schema migrations",
	Long:  "performs a downards schema migration command",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireAndSetFlags(horizon.DatabaseURLFlagName, horizon.IngestFlagName); err != nil {
			return err
		}

		// Only allow invocations with 1 args.
		if len(args) != 1 {
			return ErrUsage{cmd}
		}

		count, err := strconv.Atoi(args[0])
		if err != nil {
			log.Println(err)
			return ErrUsage{cmd}
		}

		return migrate(schema.MigrateDown, count)
	},
}

var dbMigrateRedoCmd = &cobra.Command{
	Use:   "redo COUNT",
	Short: "redo db schema migrations",
	Long:  "performs a redo schema migration command",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireAndSetFlags(horizon.DatabaseURLFlagName, horizon.IngestFlagName); err != nil {
			return err
		}

		// Only allow invocations with 1 args.
		if len(args) != 1 {
			return ErrUsage{cmd}
		}

		count, err := strconv.Atoi(args[0])
		if err != nil {
			log.Println(err)
			return ErrUsage{cmd}
		}

		return migrate(schema.MigrateRedo, count)
	},
}

var dbMigrateStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "print current database migration status",
	Long:  "print current database migration status",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireAndSetFlags(horizon.DatabaseURLFlagName); err != nil {
			return err
		}

		// Only allow invocations with 0 args.
		if len(args) != 0 {
			fmt.Println(args)
			return ErrUsage{cmd}
		}

		dbConn, err := db.Open("postgres", config.DatabaseURL)
		if err != nil {
			return err
		}

		status, err := schema.Status(dbConn.DB.DB)
		if err != nil {
			return err
		}

		fmt.Println(status)
		return nil
	},
}

var dbMigrateUpCmd = &cobra.Command{
	Use:   "up [COUNT]",
	Short: "run upwards db schema migrations",
	Long:  "performs an upwards schema migration command",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireAndSetFlags(horizon.DatabaseURLFlagName, horizon.IngestFlagName); err != nil {
			return err
		}

		// Only allow invocations with 0-1 args.
		if len(args) > 1 {
			return ErrUsage{cmd}
		}

		count := 0
		if len(args) == 1 {
			var err error
			count, err = strconv.Atoi(args[0])
			if err != nil {
				log.Println(err)
				return ErrUsage{cmd}
			}
		}

		return migrate(schema.MigrateUp, count)
	},
}

var dbReapCmd = &cobra.Command{
	Use:   "reap",
	Short: "reaps (i.e. removes) any reapable history data",
	Long:  "reap removes any historical data that is earlier than the configured retention cutoff",
	RunE: func(cmd *cobra.Command, args []string) error {
		app, err := horizon.NewAppFromFlags(config, flags)
		if err != nil {
			return err
		}
		ctx := context.Background()
		app.UpdateHorizonLedgerState(ctx)
		return app.DeleteUnretainedHistory(ctx)
	},
}

var dbReingestCmd = &cobra.Command{
	Use:   "reingest",
	Short: "reingest commands",
	Long:  "reingest ingests historical data for every ledger or ledgers specified by subcommand",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Use one of the subcomands...")
		return ErrUsage{cmd}
	},
}

var (
	reingestForce       bool
	parallelWorkers     uint
	parallelJobSize     uint32
	retries             uint
	retryBackoffSeconds uint
)

func ingestRangeCmdOpts() support.ConfigOptions {
	return support.ConfigOptions{
		{
			Name:        "force",
			ConfigKey:   &reingestForce,
			OptType:     types.Bool,
			Required:    false,
			FlagDefault: false,
			Usage: "[optional] if this flag is set, horizon will be blocked " +
				"from ingesting until the reingestion command completes (incompatible with --parallel-workers > 1)",
		},
		{
			Name:        "parallel-workers",
			ConfigKey:   &parallelWorkers,
			OptType:     types.Uint,
			Required:    false,
			FlagDefault: uint(1),
			Usage:       "[optional] if this flag is set to > 1, horizon will parallelize reingestion using the supplied number of workers",
		},
		{
			Name:        "parallel-job-size",
			ConfigKey:   &parallelJobSize,
			OptType:     types.Uint32,
			Required:    false,
			FlagDefault: uint32(100000),
			Usage:       "[optional] parallel workers will run jobs processing ledger batches of the supplied size",
		},
		{
			Name:        "retries",
			ConfigKey:   &retries,
			OptType:     types.Uint,
			Required:    false,
			FlagDefault: uint(0),
			Usage:       "[optional] number of reingest retries",
		},
		{
			Name:        "retry-backoff-seconds",
			ConfigKey:   &retryBackoffSeconds,
			OptType:     types.Uint,
			Required:    false,
			FlagDefault: uint(5),
			Usage:       "[optional] backoff seconds between reingest retries",
		},
	}
}

var dbReingestRangeCmdOpts = ingestRangeCmdOpts()
var dbReingestRangeCmd = &cobra.Command{
	Use:   "range [Start sequence number] [End sequence number]",
	Short: "reingests ledgers within a range",
	Long:  "reingests ledgers between X and Y sequence number (closed intervals)",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := dbReingestRangeCmdOpts.RequireE(); err != nil {
			return err
		}
		if err := dbReingestRangeCmdOpts.SetValues(); err != nil {
			return err
		}

		if len(args) != 2 {
			return ErrUsage{cmd}
		}

		argsUInt32 := make([]uint32, 2)
		for i, arg := range args {
			if seq, err := strconv.ParseUint(arg, 10, 32); err != nil {
				cmd.Usage()
				return fmt.Errorf(`invalid sequence number "%s"`, arg)
			} else {
				argsUInt32[i] = uint32(seq)
			}
		}

		err := horizon.ApplyFlags(config, flags, horizon.ApplyOptions{RequireCaptiveCoreConfig: false, AlwaysIngest: true})
		if err != nil {
			return err
		}
		return runDBReingestRange(
			[]history.LedgerRange{{StartSequence: argsUInt32[0], EndSequence: argsUInt32[1]}},
			reingestForce,
			parallelWorkers,
			*config,
		)
	},
}

var dbFillGapsCmdOpts = ingestRangeCmdOpts()
var dbFillGapsCmd = &cobra.Command{
	Use:   "fill-gaps [Start sequence number] [End sequence number]",
	Short: "Ingests any gaps found in the horizon db",
	Long:  "Ingests any gaps found in the horizon db. The command takes an optional start and end parameters which restrict the range of ledgers ingested.",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := dbFillGapsCmdOpts.RequireE(); err != nil {
			return err
		}
		if err := dbFillGapsCmdOpts.SetValues(); err != nil {
			return err
		}

		if len(args) != 0 && len(args) != 2 {
			hlog.Errorf("Expected either 0 arguments or 2 but found %v arguments", len(args))
			return ErrUsage{cmd}
		}

		var start, end uint64
		var withRange bool
		if len(args) == 2 {
			var err error
			start, err = strconv.ParseUint(args[0], 10, 32)
			if err != nil {
				cmd.Usage()
				return fmt.Errorf(`invalid sequence number "%s"`, args[0])
			}
			end, err = strconv.ParseUint(args[1], 10, 32)
			if err != nil {
				cmd.Usage()
				return fmt.Errorf(`invalid sequence number "%s"`, args[1])
			}
			withRange = true
		}

		err := horizon.ApplyFlags(config, flags, horizon.ApplyOptions{RequireCaptiveCoreConfig: false, AlwaysIngest: true})
		if err != nil {
			return err
		}
		var gaps []history.LedgerRange
		if withRange {
			gaps, err = runDBDetectGapsInRange(*config, uint32(start), uint32(end))
			if err != nil {
				return err
			}
			hlog.Infof("found gaps %v within range [%v, %v]", gaps, start, end)
		} else {
			gaps, err = runDBDetectGaps(*config)
			if err != nil {
				return err
			}
			hlog.Infof("found gaps %v", gaps)
		}

		return runDBReingestRange(gaps, reingestForce, parallelWorkers, *config)
	},
}

func runDBReingestRange(ledgerRanges []history.LedgerRange, reingestForce bool, parallelWorkers uint, config horizon.Config) error {
	var err error

	if reingestForce && parallelWorkers > 1 {
		return errors.New("--force is incompatible with --parallel-workers > 1")
	}

	ingestConfig := ingest.Config{
		NetworkPassphrase:           config.NetworkPassphrase,
		HistoryArchiveURLs:          config.HistoryArchiveURLs,
		CheckpointFrequency:         config.CheckpointFrequency,
		ReingestEnabled:             true,
		MaxReingestRetries:          int(retries),
		ReingestRetryBackoffSeconds: int(retryBackoffSeconds),
		EnableCaptiveCore:           config.EnableCaptiveCoreIngestion,
		CaptiveCoreBinaryPath:       config.CaptiveCoreBinaryPath,
		CaptiveCoreConfigUseDB:      config.CaptiveCoreConfigUseDB,
		RemoteCaptiveCoreURL:        config.RemoteCaptiveCoreURL,
		CaptiveCoreToml:             config.CaptiveCoreToml,
		CaptiveCoreStoragePath:      config.CaptiveCoreStoragePath,
		StellarCoreCursor:           config.CursorName,
		StellarCoreURL:              config.StellarCoreURL,
		RoundingSlippageFilter:      config.RoundingSlippageFilter,
		EnableIngestionFiltering:    config.EnableIngestionFiltering,
	}

	if ingestConfig.HistorySession, err = db.Open("postgres", config.DatabaseURL); err != nil {
		return fmt.Errorf("cannot open Horizon DB: %v", err)
	}

	if !config.EnableCaptiveCoreIngestion {
		if config.StellarCoreDatabaseURL == "" {
			return fmt.Errorf("flag --%s cannot be empty", horizon.StellarCoreDBURLFlagName)
		}
		if ingestConfig.CoreSession, err = db.Open("postgres", config.StellarCoreDatabaseURL); err != nil {
			ingestConfig.HistorySession.Close()
			return fmt.Errorf("cannot open Core DB: %v", err)
		}
	}

	if parallelWorkers > 1 {
		system, systemErr := ingest.NewParallelSystems(ingestConfig, parallelWorkers)
		if systemErr != nil {
			return systemErr
		}

		return system.ReingestRange(
			ledgerRanges,
			parallelJobSize,
		)
	}

	system, systemErr := ingest.NewSystem(ingestConfig)
	if systemErr != nil {
		return systemErr
	}
	defer system.Shutdown()

	err = system.ReingestRange(ledgerRanges, reingestForce)
	if err != nil {
		if _, ok := errors.Cause(err).(ingest.ErrReingestRangeConflict); ok {
			return fmt.Errorf(`The range you have provided overlaps with Horizon's most recently ingested ledger.
It is not possible to run the reingest command on this range in parallel with
Horizon's ingestion system.
Either reduce the range so that it doesn't overlap with Horizon's ingestion system,
or, use the force flag to ensure that Horizon's ingestion system is blocked until
the reingest command completes.`)
		}

		return err
	}
	hlog.Info("Range run successfully!")
	return nil
}

var dbDetectGapsCmd = &cobra.Command{
	Use:   "detect-gaps",
	Short: "detects ingestion gaps in Horizon's database",
	Long:  "detects ingestion gaps in Horizon's database and prints a list of reingest commands needed to fill the gaps",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireAndSetFlags(horizon.DatabaseURLFlagName); err != nil {
			return err
		}

		if len(args) != 0 {
			return ErrUsage{cmd}
		}
		gaps, err := runDBDetectGaps(*config)
		if err != nil {
			return err
		}
		if len(gaps) == 0 {
			hlog.Info("No gaps found")
			return nil
		}
		fmt.Println("Horizon commands to run in order to fill in the gaps:")
		cmdname := os.Args[0]
		for _, g := range gaps {
			fmt.Printf("%s db reingest range %d %d\n", cmdname, g.StartSequence, g.EndSequence)
		}
		return nil
	},
}

func runDBDetectGaps(config horizon.Config) ([]history.LedgerRange, error) {
	horizonSession, err := db.Open("postgres", config.DatabaseURL)
	if err != nil {
		return nil, err
	}
	defer horizonSession.Close()
	q := &history.Q{horizonSession}
	return q.GetLedgerGaps(context.Background())
}

func runDBDetectGapsInRange(config horizon.Config, start, end uint32) ([]history.LedgerRange, error) {
	horizonSession, err := db.Open("postgres", config.DatabaseURL)
	if err != nil {
		return nil, err
	}
	defer horizonSession.Close()
	q := &history.Q{horizonSession}
	return q.GetLedgerGapsInRange(context.Background(), start, end)
}

func init() {
	if err := dbReingestRangeCmdOpts.Init(dbReingestRangeCmd); err != nil {
		log.Fatal(err.Error())
	}
	if err := dbFillGapsCmdOpts.Init(dbFillGapsCmd); err != nil {
		log.Fatal(err.Error())
	}

	viper.BindPFlags(dbReingestRangeCmd.PersistentFlags())
	viper.BindPFlags(dbFillGapsCmd.PersistentFlags())

	RootCmd.AddCommand(dbCmd)
	dbCmd.AddCommand(
		dbInitCmd,
		dbMigrateCmd,
		dbReapCmd,
		dbReingestCmd,
		dbDetectGapsCmd,
		dbFillGapsCmd,
	)
	dbMigrateCmd.AddCommand(
		dbMigrateDownCmd,
		dbMigrateRedoCmd,
		dbMigrateStatusCmd,
		dbMigrateUpCmd,
	)
	dbReingestCmd.AddCommand(dbReingestRangeCmd)
}
