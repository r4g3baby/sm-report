package main

import (
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: "02 Jan 15:04"})
	zerolog.SetGlobalLevel(zerolog.InfoLevel)

	setUpConfig()
	setUpDatabase()
	setUpDiscord()

	log.Info().Msg("bot is now running, press CTRL-C to exit")

	stopSignal := make(chan os.Signal, 1)
	signal.Notify(stopSignal, syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP, os.Interrupt)

	sig := <-stopSignal
	log.Info().Str("signal", sig.String()).Msg("received shutdown signal")

	closeDiscord()
	closeDatabase()
}
