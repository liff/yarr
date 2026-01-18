package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/rs/zerolog/log"

	"github.com/nkanaev/yarr/src/parser"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Println("usage: <script> [url|filepath]")
		return
	}
	url := os.Args[1]
	var r io.Reader

	if strings.HasPrefix(url, "http") {
		res, err := http.Get(url)
		if err != nil {
			log.Fatal().Err(err).Msgf("failed to get url %s", url)
		}
		r = res.Body
	} else {
		var err error
		r, err = os.Open(url)
		if err != nil {
			log.Fatal().Err(err).Msg("failed to open file")
		}
	}
	feed, err := parser.Parse(r)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to parse feed: %s")
	}
	body, err := json.MarshalIndent(feed, "", "  ")
	if err != nil {
		log.Fatal().Err(err).Msg("failed to marshall feed")
	}
	fmt.Println(string(body))
}
