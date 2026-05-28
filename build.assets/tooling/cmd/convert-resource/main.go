package main

import (
	"bytes"
	"io"

	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
)

type kindObject struct {
	kind string
}

func convertToTerraform(r io.Reader) (io.Writer, error) {
	var buf bytes.Buffer
	data, err := io.Copy(&buf, r)
	if err != nil {
		return nil, trace.Errorf("unable to read input YAML: %w", err)
	}

	json, err := utils.ToJSON(data)
	if err != nil {
		return nil, trace.Errorf("unable to process input YAML as JSON (which we need to do to convert it to a Teleport resource type): %w", err)

	}

	var o kindObject
	if err = json.NewDecoder(r).Decode(&o); err != nil {
		return nil, trace.Errorf("unable to detect a kind in the input resource: %w", err)
	}

	return nil, nil
}

func main() {
}
