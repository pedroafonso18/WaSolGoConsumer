package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"wasolgo/internal/parser"
)

func SendRequest(req *parser.Request) error {
	if req.Url == "" {
		return fmt.Errorf("request url is empty. cannot send http request")
	}

	jsonBody, err := json.Marshal(req.Body)
	if err != nil {
		fmt.Printf("Error when marshalling the json: %v", err)
		return err
	}

	httpReq, err := http.NewRequest(req.Method, req.Url, bytes.NewBuffer(jsonBody))
	if err != nil {
		fmt.Printf("Error when creating the HTTP request: %v", err)
		return err
	}

	for key, value := range req.Headers {
		httpReq.Header.Set(key, value)
	}

	client := &http.Client{}
	resp, err := client.Do(httpReq)
	if err != nil {
		fmt.Printf("Error when sending the HTTP request: %v", err)
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		fmt.Printf("Request failed with status: %s", resp.Status)
		return nil
	}

	fmt.Printf("Request was successfull with status: %s", resp.Status)
	return nil
}
