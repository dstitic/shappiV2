package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

const (
	sandboxBaseURL  = "https://api-sandbox.dhl.com/dpi"
	authURL         = sandboxBaseURL + "/oauth/accesstoken"
	createOrderURL  = sandboxBaseURL + "/shipping/v1/orders"
	getItemLabelURL = sandboxBaseURL + "/shipping/v1/items/%s/label"
)

type DHLClient struct {
	ClientID     string
	ClientSecret string
	AccessToken  string
	HTTPClient   *http.Client
}

type TokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
}

type CreateOrderResponse struct {
	OrderID string `json:"orderId"`
}

func NewDHLClient(clientID, clientSecret string) *DHLClient {
	return &DHLClient{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *DHLClient) GetAccessToken(ctx context.Context) error {
	auth := base64.StdEncoding.EncodeToString([]byte(c.ClientID + ":" + c.ClientSecret))

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, authURL, nil)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	req.Header.Add("Authorization", "Basic "+auth)
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("sending request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var tokenResp TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return fmt.Errorf("decoding response: %w", err)
	}

	c.AccessToken = tokenResp.AccessToken
	return nil
}

func (c *DHLClient) CreateOrder(ctx context.Context, orderData map[string]interface{}) (string, error) {
	jsonData, err := json.Marshal(orderData)
	if err != nil {
		return "", fmt.Errorf("marshaling order data: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, createOrderURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
	}

	req.Header.Add("Authorization", "Bearer "+c.AccessToken)
	req.Header.Add("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("sending request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(body))
	}

	var createOrderResp CreateOrderResponse
	if err := json.NewDecoder(resp.Body).Decode(&createOrderResp); err != nil {
		return "", fmt.Errorf("decoding response: %w", err)
	}

	return createOrderResp.OrderID, nil
}

func (c *DHLClient) GetItemLabel(ctx context.Context, itemID string) ([]byte, error) {
	url := fmt.Sprintf(getItemLabelURL, itemID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Add("Authorization", "Bearer "+c.AccessToken)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sending request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	label, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}

	return label, nil
}

func main() {
	client := NewDHLClient("your-client-id", "your-client-secret")

	ctx := context.Background()

	err := client.GetAccessToken(ctx)
	if err != nil {
		fmt.Printf("Error getting access token: %v\n", err)
		return
	}

	orderData := map[string]interface{}{
		"productCode": "GPP",
		"receiverDetails": map[string]interface{}{
			"name": map[string]interface{}{
				"firstName": "John",
				"lastName":  "Doe",
			},
			"address": map[string]interface{}{
				"street":     "Sample Street",
				"houseNo":    "123",
				"postalCode": "12345",
				"city":       "Sample City",
				"country":    "DE",
			},
		},
		"shipmentDetails": map[string]interface{}{
			"weightInGrams": 1000,
			"length":        20,
			"width":         15,
			"height":        10,
		},
	}

	orderID, err := client.CreateOrder(ctx, orderData)
	if err != nil {
		fmt.Printf("Error creating order: %v\n", err)
		return
	}
	fmt.Printf("Order created with ID: %s\n", orderID)

	label, err := client.GetItemLabel(ctx, orderID)
	if err != nil {
		fmt.Printf("Error getting item label: %v\n", err)
		return
	}
	fmt.Printf("Label received, size: %d bytes\n", len(label))

	// Save the label to a file or process it further

	err = os.WriteFile("label.pdf", label, 0644)
	if err != nil {
		fmt.Printf("Error saving label: %v\n", err)
	}
}
