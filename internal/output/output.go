package output

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/pterm/pterm"
	"gopkg.in/yaml.v3"
)

type Mode string

const (
	ModeTable Mode = "table"
	ModeJSON  Mode = "json"
	ModeYAML  Mode = "yaml"
)

func ParseMode(raw string) (Mode, error) {
	switch raw {
	case "", string(ModeTable):
		return ModeTable, nil
	case string(ModeJSON):
		return ModeJSON, nil
	case string(ModeYAML):
		return ModeYAML, nil
	default:
		return "", fmt.Errorf("invalid output mode: %s", raw)
	}
}

func InitStyles() {
	if os.Getenv("NO_COLOR") != "" {
		pterm.DisableColor()
	}
}

func EmitJSON(value any) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

func EmitYAML(value any) error {
	encoder := yaml.NewEncoder(os.Stdout)
	encoder.SetIndent(2)
	defer encoder.Close()
	return encoder.Encode(value)
}
