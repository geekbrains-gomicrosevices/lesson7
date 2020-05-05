package requester

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
)

func PatchJSON(url string, params url.Values, result interface{}) error {

	request, err := http.NewRequest(
		http.MethodPatch,
		url,
		strings.NewReader(params.Encode()),
	)
	if err != nil {
		return fmt.Errorf("Build request error: %w", err)
	}

	request.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return fmt.Errorf("Response error: %w", err)
	}

	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("Bad status: %d", response.StatusCode)
	}

	// Тело ответа не требуется
	if result == nil {
		return nil
	}

	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return fmt.Errorf("Can't read response %w", err)
	}

	err = json.Unmarshal(body, result)
	if err != nil {
		return fmt.Errorf("Can't parse response: %w", err)
	}

	return nil
}

func GetJSON(url string, out interface{}) error {
	res, err := http.DefaultClient.Get(url)
	if err != nil {
		return fmt.Errorf("make request error: %w", err)
	}

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return fmt.Errorf("read response error: %w", err)
	}

	err = json.Unmarshal(body, out)

	if err != nil {
		return fmt.Errorf("parse response error '%s': %w", body, err)
	}

	return nil
}
