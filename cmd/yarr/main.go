package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/journald"
	"github.com/rs/zerolog/log"

	"github.com/nkanaev/yarr/src/platform"
	"github.com/nkanaev/yarr/src/server"
	"github.com/nkanaev/yarr/src/storage"
	"github.com/nkanaev/yarr/src/worker"
)

var Version string = "0.0"
var GitHash string = "unknown"

var OptList = make([]string, 0)

func opt(envVar, defaultValue string) string {
	OptList = append(OptList, envVar)
	value := os.Getenv(envVar)
	if value != "" {
		return value
	}
	return defaultValue
}

func parseAuthfile(authfile io.Reader) (username, password string, err error) {
	scanner := bufio.NewScanner(authfile)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			return "", "", fmt.Errorf("wrong syntax (expected `username:password`)")
		}
		username = parts[0]
		password = parts[1]
		break
	}
	return username, password, nil
}

func main() {
	platform.FixConsoleIfNeeded()

	var addr, db, authfile, auth, certfile, keyfile, basepath, logfile string
	var ver, open, journal bool

	flag.CommandLine.SetOutput(os.Stdout)

	flag.Usage = func() {
		out := flag.CommandLine.Output()
		fmt.Fprintf(out, "Usage of %s:\n", os.Args[0])
		flag.PrintDefaults()
		fmt.Fprintln(out, "\nThe environmental variables, if present, will be used to provide\nthe default values for the params above:")
		fmt.Fprintln(out, " ", strings.Join(OptList, ", "))
	}

	flag.StringVar(&addr, "addr", opt("YARR_ADDR", "127.0.0.1:7070"), "address to run server on")
	flag.StringVar(&basepath, "base", opt("YARR_BASE", ""), "base path of the service url")
	flag.StringVar(&authfile, "auth-file", opt("YARR_AUTHFILE", ""), "`path` to a file containing username:password. Takes precedence over --auth (or YARR_AUTH)")
	flag.StringVar(&auth, "auth", opt("YARR_AUTH", ""), "string with username and password in the format `username:password`")
	flag.StringVar(&certfile, "cert-file", opt("YARR_CERTFILE", ""), "`path` to cert file for https")
	flag.StringVar(&keyfile, "key-file", opt("YARR_KEYFILE", ""), "`path` to key file for https")
	flag.StringVar(&db, "db", opt("YARR_DB", ""), "storage file `path`")
	flag.StringVar(&logfile, "log-file", opt("YARR_LOGFILE", ""), "`path` to log file to use instead of stdout")
	flag.BoolVar(&ver, "version", false, "print application version")
	flag.BoolVar(&open, "open", false, "open the server in browser")
	flag.BoolVar(&journal, "journal", false, "log to systemd journal")
	flag.Parse()

	if ver {
		fmt.Printf("v%s (%s)\n", Version, GitHash)
		return
	}

	if journal {
		log.Logger = zerolog.New(journald.NewJournalDWriter())
	} else if logfile != "" {
		file, err := os.OpenFile(logfile, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0644)
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to setup log file: ")
		}
		defer file.Close()
		log.Logger = zerolog.New(file).With().Timestamp().Logger()
	} else {
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	}

	if open && strings.HasPrefix(addr, "unix:") {
		log.Fatal().Msgf("Cannot open %s in browser", addr)
	}

	if db == "" {
		configPath, err := os.UserConfigDir()
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to get config dir")
		}

		storagePath := filepath.Join(configPath, "yarr")
		if err := os.MkdirAll(storagePath, 0755); err != nil {
			log.Fatal().Err(err).Msg("Failed to create app config dir")
		}
		db = filepath.Join(storagePath, "storage.db")
	}

	log.Info().Msgf("using db file %s", db)

	var username, password string
	var err error
	if authfile != "" {
		f, err := os.Open(authfile)
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to open auth file")
		}
		defer f.Close()
		username, password, err = parseAuthfile(f)
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to parse auth file")
		}
	} else if auth != "" {
		username, password, err = parseAuthfile(strings.NewReader(auth))
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to parse auth literal")
		}
	}

	if (certfile != "" || keyfile != "") && (certfile == "" || keyfile == "") {
		log.Fatal().Msg("Both cert & key files are required")
	}

	store, err := storage.New(db)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to initialise database")
	}

	worker.SetVersion(Version)
	srv := server.NewServer(store, addr)

	if basepath != "" {
		srv.BasePath = "/" + strings.Trim(basepath, "/")
	}

	if certfile != "" && keyfile != "" {
		srv.CertFile = certfile
		srv.KeyFile = keyfile
	}

	if username != "" && password != "" {
		srv.Username = username
		srv.Password = password
	}

	log.Info().Msgf("starting server at %s", srv.GetAddr())
	if open {
		platform.Open(srv.GetAddr())
	}
	platform.Start(srv)
}
