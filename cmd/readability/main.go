package main

import (
	"fmt"
	"github.com/rs/zerolog/log"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/nkanaev/yarr/src/content/readability"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Println("usage: <script> [url]")
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

	content, err := readability.ExtractContent(r)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to extract content")
	}
	fmt.Println(content)
}
