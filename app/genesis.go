package app

import "encoding/json"

// GenesisState maps module names to their raw JSON genesis state.
type GenesisState map[string]json.RawMessage
