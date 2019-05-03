package hesperides

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type Client struct {
	url        string
	httpClient *http.Client
	username   string
	password   string
	app        string
	platform   string
}

func New(nodes []string, username string, password string, app string, platform string) (*Client, error) {
	if len(nodes) == 0 {
		return nil, errors.New("An endpoint is required as a --node argument")
	}
	if len(nodes) > 1 {
		return nil, errors.New("A single endpoint must be provided as --node argument")
	}

	client := &Client{
		url:        nodes[0],
		httpClient: &http.Client{
			Timeout: time.Second * 10,
		},
		username:   username,
		password:   password,
		app:        app,
		platform:   platform,
	}

	err := client.testConnection()
	return client, err
}

func (c *Client) testConnection() error {
	var err error
	maxTime := 20 * time.Second

	for i := 1 * time.Second; i < maxTime; i *= time.Duration(2) {
		if _, err = c.makeRequest("/rest/versions"); err != nil {
			time.Sleep(i)
		} else {
			return nil
		}
	}
	return err
}

func (c *Client) makeRequest(path string) ([]byte, error) {
	req, _ := http.NewRequest("GET", strings.Join([]string{c.url, path}, ""), nil)
	req.Header.Set("Accept", "application/json")
	basicAuth := base64.StdEncoding.EncodeToString([]byte(c.username + ":" + c.password))
	req.Header.Add("Authorization", "Basic "+basicAuth)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var respBody []byte
	if respBody, err = ioutil.ReadAll(resp.Body); err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		return nil, errors.New("HTTP error : " + strconv.Itoa(resp.StatusCode) + " - " + string(respBody))
	}
	return respBody, nil
}

type KeyValue struct {
	Name  string
	Value string
}

func (c *Client) GetValues(prefix string, keys []string) (map[string]string, error) {
	propertiesPath := strings.NewReplacer("/", "#").Replace(prefix)
	fmt.Println(propertiesPath)
	path := fmt.Sprintf("/rest/applications/%s/platforms/%s/properties?path=%s", c.app, c.platform, url.QueryEscape(propertiesPath))
	body, err := c.makeRequest(path)
	vars := map[string]string{}
	if err != nil {
		return vars, err
	}
	jsonResponse := make(map[string][]KeyValue)
	if err = json.Unmarshal(body, &jsonResponse); err != nil {
		return vars, err
	}
	// Transforming .key_value_properties list into a map:
	keyValues := map[string]string{}
	for _, keyValue := range jsonResponse["key_value_properties"] {
		keyValues[keyValue.Name] = keyValue.Value
	}
	// Filtering based on $keys parameter:
	for _, key := range keys {
		vars[key] = keyValues[key]
	}
	return vars, nil
}

type watchResponse struct {
	waitIndex uint64
	err       error
}

func (c *Client) WatchPrefix(prefix string, keys []string, waitIndex uint64, stopChan chan bool) (uint64, error) {
	// return something > 0 to trigger an initial retrieval from the store
	if waitIndex == 0 {
		return 1, nil
	}

	respChan := make(chan watchResponse)
	go func() {
		versionId := 0
		for {
			newVersionId, err := c.getVersionId(prefix)
			if err != nil {
				respChan <- watchResponse{0, err}
				return
			}

			if versionId != newVersionId {
				respChan <- watchResponse{uint64(newVersionId), nil}
				return
			}

			versionId = newVersionId
		}
	}()

	select {
	case <-stopChan:
		return waitIndex, nil
	case r := <-respChan:
		return r.waitIndex, r.err
	}
}

type timeout interface {
	Timeout() bool
}

func (c *Client) getVersionId(prefix string) (int, error) {
	path := fmt.Sprintf("/rest/applications/%s/platforms/%s", c.app, c.platform)
	for {
		resp, err := c.makeRequest(path)
		if err != nil {
			// Lucas: TODO understand this code...
			t, ok := err.(timeout)
			if ok && t.Timeout() {
				continue
			}
			return 0, err
		}
		jsonResponse := make(map[string]string)
		if err = json.Unmarshal(resp, &jsonResponse); err != nil {
			return 0, err
		}
		return strconv.Atoi(jsonResponse["version_id"])
	}
}
