package httpprovider

import (
	"fmt"
	"io/ioutil"
	"net/http"
)

func get(client http.Client, url string) ([]byte, error) {
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("error invoking http request: %v", err)
	}

	defer resp.Body.Close()
	return ioutil.ReadAll(resp.Body)
}
