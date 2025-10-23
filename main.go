package main

import (
	"flag"
	"os"

	"github.com/sirupsen/logrus"

	"github.com/harvester/storage-validator/pkg/validation"
)

var (
	configFile string
	debug      bool
	Version    string
)

func main() {
	flag.StringVar(&configFile, "config", "config.yaml", "Path to config file")
	flag.BoolVar(&debug, "debug", false, "Debug mode")
	flag.Parse()

	if debug {
		logrus.SetLevel(logrus.DebugLevel)
	} else {
		logrus.SetFormatter(&logrus.TextFormatter{})
	}

	v := &validation.ValidationRun{
		ConfigFile: configFile,
		Version:    Version,
	}

	// run validation
	if err := v.Execute(); err != nil {
		logrus.Errorf("error while running validation: %v", err)
		os.Exit(1)
	}
}
