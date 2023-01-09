package main

import (
	"compress/gzip"
	"encoding/json"
	"io"
	"net/http"

	"github.com/go-resty/resty/v2"
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

	body, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	return body, nil
}

var restyClient = resty.New()

func VerifyCaptcha(clientCaptcha string, secret string) (bool, error) {
	data := make(map[string]string)
	data["response"] = clientCaptcha
	data["secret"] = secret

	re, err := restyClient.R().
		EnableTrace().
		SetHeader("Content-Type", "application/x-www-form-urlencoded").SetFormData(data).
		Post("https://hcaptcha.com/siteverify")
	if err != nil {
		return false, err
	}

	var responseBodyData struct {
		Success bool `json:"success"`
	}

	err = json.Unmarshal(re.Body(), &responseBodyData)
	if err != nil {
		return false, err
	}

	return responseBodyData.Success, nil
}
