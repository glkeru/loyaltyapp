package points

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

type CalculateResponse struct {
	Points int32 `json:"points"`
}

func CalculateOrder(ctx context.Context, orderJson string) (points int32, err error) {

	// config
	host := os.Getenv("ENGINE_HOST")
	if host == "" {
		return 0, fmt.Errorf("env ENGINE_HOST is not set")
	}
	port := os.Getenv("ENGINE_PORT")
	if port == "" {
		return 0, fmt.Errorf("env ENGINE_PORT is not set")
	}

	// вызов расчета баллов
	client := &http.Client{Timeout: 5 * time.Second}
	orderData := []byte(orderJson)
	req, err := http.NewRequest("POST", host+":"+port, bytes.NewBuffer(orderData))
	if err != nil {
		return 0, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("Engine service HTTP error: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}

	calcResponse := &CalculateResponse{}
	err = json.Unmarshal(body, calcResponse)
	if err != nil {
		return 0, err
	}

	return calcResponse.Points, nil
}
