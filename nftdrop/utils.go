package main

import (
	"compress/gzip"
	"io"
	"io/ioutil"
	"net/http"
)

func readRespBody(resp *http.Response) ([]byte, error) {
	var reader io.ReadCloser
	var err error
	switch resp.Header.Get("Content-Encoding") {
	case "gzip":
		reader, err = gzip.NewReader(resp.Body)
		if err != nil {
			return nil, err
		}
		defer reader.Close()
	default:
		reader = resp.Body
	}

	body, err := ioutil.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	return body, nil
}
