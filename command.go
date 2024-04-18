package main

// Command have both runnable command data (Path and Args) and group data (SubCommands).
type Command struct {
	Path string   `json:"path,omitempty" yaml:"path,omitempty"`
	Args []string `json:"args,omitempty" yaml:"args,omitempty,flow"`
}
