package tools

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"net/url"
)

type RequestApiInput struct {
	Url         string
	Method      string
	Header      http.Header
	QueryString url.Values
	PostBody    []byte
}

func RequestApi(input RequestApiInput) (interface{}, error) {
	client := &http.Client{}

	var req *http.Request
	var err error

	if input.Method == http.MethodPost {
		req, err = http.NewRequest(input.Method, input.Url, bytes.NewBuffer(input.PostBody))
		if len(input.QueryString) != 0 {
			req.URL.RawQuery = input.QueryString.Encode()
		}
	} else if input.Method == http.MethodGet {
		req, err = http.NewRequest(input.Method, input.Url, nil)
		req.URL.RawQuery = input.QueryString.Encode()
	}

	if err != nil {
		return new(interface{}), err
	}

	req.Header = input.Header

	resp, err := client.Do(req)
	if err != nil {
		return new(interface{}), err
	}

	respBody, _ := ioutil.ReadAll(resp.Body)
	defer resp.Body.Close()

	var result interface{}
	if err = json.Unmarshal(respBody, &result); err != nil {
		return string(respBody), err
	}

	if resp.Status != "200 OK" {
		return result, errors.New("resp status not ok, " + string(respBody))
	}

	return result, nil
}
