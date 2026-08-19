package main

import (
	"flag"
	"os"

	"github.com/sirupsen/logrus"

	"github.com/harvester/storage-validator/pkg/validation"
)

var (
	configFile    string
	debug         bool
	accessMode    string
	singleNode    bool
	skipMigration bool
	Version       string
)

func main() {
	flag.StringVar(&configFile, "config", "config.yaml", "Path to config file")
	flag.BoolVar(&debug, "debug", false, "Debug mode")
	flag.StringVar(&accessMode, "access-mode", "", "Override PVC access mode for all created volumes, e.g. ReadWriteOnce for node-local/RWO-only drivers such as LVM CSI. Defaults to config or ReadWriteMany.")
	flag.BoolVar(&singleNode, "single-node", false, "Allow running on a single-node cluster by relaxing the pre-flight >=2 Ready nodes requirement to >=1. Implies -skip-migration.")
	flag.BoolVar(&skipMigration, "skip-migration", false, "Skip the live-migration check (for node-local storage such as LVM CSI whose volumes cannot migrate).")
	flag.Parse()

	if debug {
		logrus.SetLevel(logrus.DebugLevel)
	} else {
		logrus.SetFormatter(&logrus.TextFormatter{})
	}

	v := &validation.ValidationRun{
		ConfigFile:            configFile,
		Version:               Version,
		AccessModeOverride:    accessMode,
		SingleNodeOverride:    singleNode,
		SkipMigrationOverride: skipMigration,
	}

	// run validation
	if err := v.Execute(); err != nil {
		logrus.Errorf("error while running validation: %v", err)
		os.Exit(1)
	}
}
