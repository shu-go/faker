package main

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

// Command have both runnable command data (Path and Args) and group data (SubCommands).
type Command struct {
	Path string   `json:"path,omitempty"`
	Args []string `json:"args,omitempty"`

	SubCommands map[string]*Command `json:"sub,omitempty"`
}

// IsGroup returns if Command c is also a group.
// It does not care about c have runnable command data.
func (c Command) IsGroup() bool {
	return len(c.SubCommands) != 0
}

// IsRunnable returns if Command c is runnable.
func (c Command) IsRunnable() bool {
	return c.Path != ""
}

// FindCommand returns a sub/subsub/... command found in SumCommands recursively.
func (c Command) FindCommand(args []string, exact bool) (*Command, []string, error) {
	curr := &c

	lastIdx := -1
	for i, a := range args {
		c, err := curr.findChildCommand(a, exact)
		if err != nil {
			return nil, nil, err
		}

		if c == nil {
			break
		}

		lastIdx = i
		curr = c
	}

	//rog.Print(lastIdx)
	if lastIdx == -1 {
		return nil, nil, errors.New("not found")
	}

	return curr, args[lastIdx+1:], nil
}

func (c Command) findChildCommand(name string, exact bool) (*Command, error) {
	if exact {
		if c, found := c.SubCommands[name]; found {
			return c, nil
		}
		return nil, nil
	}

	var candidates []*Command
	for n, sub := range c.SubCommands {
		if n == name {
			return sub, nil
		}
		if strings.HasPrefix(n, name) {
			candidates = append(candidates, sub)
		}
	}
	if len(candidates) == 1 {
		return candidates[0], nil
	} else if len(candidates) > 1 {
		return nil, errors.New("ambiguous")
	}
	return nil, nil
}

// PrintCommand prints itself and its SubCommands to stddout.
func (c Command) PrintCommand(name string, byPath bool, indent int) {
	//rog.Printf("PrintCommand: %v, %v", name, indent)

	if name != "" {
		var cmdInfo string
		if c.IsRunnable() {
			cmdInfo = fmt.Sprintf("%s %v", c.Path, c.Args)
		}

		if indent < 0 {
			indent = 0
		}
		spaces := strings.Repeat("  ", indent)
		fmt.Printf("\t%s%s:\t%s\n", spaces, name, cmdInfo)
	}

	keyNames := []struct {
		Key, Name string
	}{}
	for k := range c.SubCommands {
		key := k
		name := k
		if byPath {
			key = c.SubCommands[k].Path
		}
		keyNames = append(keyNames, struct{ Key, Name string }{Key: key, Name: name})
	}
	sort.Slice(keyNames, func(i, j int) bool {
		return keyNames[i].Key < keyNames[j].Key
	})

	for _, k := range keyNames {
		v := c.SubCommands[k.Name]
		//rog.Printf("PrintCommand: %v", k)
		v.PrintCommand(k.Name, byPath, indent+1)
	}
}

// AddSubCommand adds a Command newCmd to the location specified by names.
func (c *Command) AddSubCommand(names []string, newCmd Command) error {
	if len(names) == 0 {
		return nil
	}

	isLast := len(names) == 1
	name := names[0]
	//rog.Print("isLast: ", isLast)

	cmd, found := c.SubCommands[name]
	//rog.Print(found, cmd)

	if found {
		if isLast {
			newCmd.SubCommands = cmd.SubCommands
			*cmd = newCmd
		} else {
			return cmd.AddSubCommand(names[1:], newCmd)
		}
	} else {
		if c.SubCommands == nil {
			c.SubCommands = make(map[string]*Command)
		}
		if isLast {
			c.SubCommands[name] = &newCmd
		} else {
			c.SubCommands[name] = &Command{SubCommands: make(map[string]*Command)}
			return c.SubCommands[name].AddSubCommand(names[1:], newCmd)
			//rog.Print(curr)
		}
	}

	return nil
}

// RemoveSubCommand removes a Command at the location specified by names.
func (c *Command) RemoveSubCommand(names []string) error {
	if len(names) == 0 {
		return nil
	}

	isLast := len(names) == 1
	name := names[0]

	cmd, found := c.SubCommands[name]
	if found {
		isGroup := cmd.IsGroup()

		if isLast {
			delete(c.SubCommands, name)
		} else {
			if isGroup {
				return cmd.RemoveSubCommand(names[1:])
			}
			return errors.New("not found")
		}
	} else {
		return errors.New("not found")
	}

	return nil
}

// Clean removes commands from SubCommands where not runnable and not a group.
func (c *Command) Clean() {
	subNames := []string{}
	for n, sub := range c.SubCommands {
		sub.Clean()
		if !sub.IsGroup() && !sub.IsRunnable() {
			subNames = append(subNames, n)
		}
	}

	for _, n := range subNames {
		delete(c.SubCommands, n)
	}
}
