package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"sync"

	"github.com/GoogleCloudPlatform/functions-framework-go/funcframework"

	"github.com/brexhq/substation/v2"

	"github.com/brexhq/substation/v2/internal/file"
)

// errFunctionMissingHandler is returned when the Function is deployed without a configured handler.
var errFunctionMissingHandler = fmt.Errorf("SUBSTATION_FUNCTION_HANDLER environment variable is missing")

// errFunctionInvalidHandler is returned when the Function is deployed with an unsupported handler.
var errFunctionInvalidHandler = fmt.Errorf("SUBSTATION_FUNCTION_HANDLER environment variable is invalid")

func init() {
	handler, ok := os.LookupEnv("SUBSTATION_FUNCTION_HANDLER")
	if !ok {
		panic(fmt.Errorf("init handler %s: %v", handler, errFunctionMissingHandler))
	}

	switch handler {
	case "GCP_STORAGE":
		if err := funcframework.RegisterCloudEventFunctionContext(context.Background(), "/", cloudStorageHandler); err != nil {
			panic(fmt.Errorf("init handler %s: %v", handler, err))
		}
	case "GCP_HTTP":
		if err := funcframework.RegisterHTTPFunctionContext(context.Background(), "/", httpHandler); err != nil {
			panic(fmt.Errorf("init handler %s: %v", handler, err))
		}
	case "GCP_PUBSUB":
		if err := funcframework.RegisterCloudEventFunctionContext(context.Background(), "/", pubsubHandler); err != nil {
			panic(fmt.Errorf("init handler %s: %v", handler, err))
		}
	default:
		panic(fmt.Errorf("init handler %s: %v", handler, errFunctionInvalidHandler))
	}
}

type customConfig struct {
	substation.Config

	Concurrency int `json:"concurrency"`
}

var (
	configOnce sync.Once
	configData []byte
	configErr  error
)

func getConfig(ctx context.Context) (io.Reader, error) {
	configOnce.Do(func() {
		buf := new(bytes.Buffer)

		cfg, found := os.LookupEnv("SUBSTATION_CONFIG")
		if !found {
			configErr = fmt.Errorf("no config found")
			return
		}

		path, err := file.Get(ctx, cfg)
		if err != nil {
			configErr = err
			return
		}
		defer os.Remove(path)

		conf, err := os.Open(path)
		if err != nil {
			configErr = err
			return
		}
		defer conf.Close()

		if _, err := io.Copy(buf, conf); err != nil {
			configErr = err
			return
		}

		configData = buf.Bytes()
	})

	if configErr != nil {
		return nil, configErr
	}

	return bytes.NewReader(configData), nil
}

func main() {
	// Use PORT environment variable, or default to 8080
	port := "8080"
	if envPort := os.Getenv("PORT"); envPort != "" {
		port = envPort
	}

	if err := funcframework.Start(port); err != nil {
		log.Fatalf("funcframework.Start: %v\n", err)
	}
}
