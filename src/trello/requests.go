/*
 * trellofs - A Trello POSIX filesystem
 * Copyright (C) 2022  Joao Eduardo Luis <joao@wipwd.dev>
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 */
package trello

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
)

type TrelloCtx struct {
	ID    string
	Key   string
	Token string

	client *http.Client
}

func Trello(id string, key string, token string) *TrelloCtx {
	return &TrelloCtx{id, key, token, &http.Client{}}
}

func (t *TrelloCtx) NewRequest(
	method string,
	endpoint string,
	body io.Reader,
) (*http.Request, error) {

	if !strings.HasPrefix(endpoint, "/") {
		endpoint = fmt.Sprintf("/%s", endpoint)
	}
	ep := fmt.Sprintf("https://api.trello.com/1%s", endpoint)
	req, err := http.NewRequest(method, ep, body)
	if err != nil {
		return nil, err
	}

	auth := fmt.Sprintf("OAuth oauth_consumer_key=\"%s\", oauth_token=\"%s\"",
		t.Key, t.Token)
	req.Header.Add("Authorization", auth)
	req.Header.Add("Accept", "application/json")
	return req, nil
}

func doTestAPIGet(endpoint string) ([]byte, error) {
	if strings.HasPrefix(endpoint, "https://api.trello.com/1/") {
		return nil, errors.New(
			fmt.Sprintf("improper endpoint for GET: %s", endpoint),
		)
	}
	lst := strings.Split(endpoint, "?")
	ep := lst[0]
	if strings.HasPrefix(ep, "/") {
		ep = strings.TrimPrefix(ep, "/")
	}
	if ep == "" {
		return nil, errors.New("empty endpoint")
	}
	fn := strings.ReplaceAll(ep, "/", "-")
	tdir := os.Getenv("TRELLOFS_TEST")
	testfn := fmt.Sprintf("%s/%s.json", tdir, fn)
	if _, err := os.Stat(testfn); errors.Is(err, os.ErrNotExist) {
		return nil, err
	}

	contents, err := ioutil.ReadFile(testfn)
	if err != nil {
		return nil, err
	}
	return contents, nil
}

func (t *TrelloCtx) ApiGet(endpoint string) ([]byte, error) {

	if os.Getenv("TRELLOFS_TEST") != "" {
		return doTestAPIGet(endpoint)
	}

	req, err := t.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	resp, err := t.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return body, nil
}

func MakeEndpoint(endpoint string, fields []string) string {
	f := ""
	if fields != nil && len(fields) > 0 {
		f = fmt.Sprintf("?fields=%s", strings.Join(fields, ","))
	}
	return fmt.Sprintf("%s%s", endpoint, f)
}
