package busylib_test

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"time"

	busylib "github.com/lxdb/busylib-go"
)

func ExampleClient() {
	client, err := busylib.NewClient()
	if err != nil {
		log.Print(err)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	status, err := client.System().Status(ctx)
	if err != nil {
		log.Print(err)
		return
	}
	log.Print(status.Firmware.Version)
}

func ExampleClient_Prepare() {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte("{}"))
	}))
	defer server.Close()

	client, err := busylib.NewClient(
		busylib.WithBaseURL(server.URL),
		busylib.WithVersionNegotiation(busylib.VersionNegotiationDisabled),
	)
	if err != nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	prepared, err := client.Prepare(busylib.Request{
		Method:       http.MethodGet,
		Path:         "/api/status",
		ResponseMode: busylib.ResponseModeJSON,
		RequestID:    "example-request",
	})
	if err != nil {
		return
	}

	targetURL := prepared.URL()
	fmt.Printf(
		"request: %s %s request_id=%s\n",
		prepared.Method(),
		targetURL.Path,
		prepared.RequestID(),
	)

	response, err := client.DoPrepared(ctx, prepared)
	if err != nil {
		return
	}
	fmt.Printf("response status: %d\n", response.StatusCode)

	// Output:
	// request: GET /api/status request_id=example-request
	// response status: 200
}

func ExampleNewDisplayElements() {
	elements := busylib.NewDisplayElements(
		"example",
		busylib.NewTextElement("title", "Build complete", busylib.FontNormal),
	)
	_ = elements
}
