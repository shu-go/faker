package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/shu-go/findcfg"
	"github.com/shu-go/gli/v2"
)

// Version is app version
var Version string

const userConfigFolder = "faker"

type globalCmd struct {
	Add    string `cli:"add,a" help:"add/replace a command"`
	Remove string `cli:"remove,r,delete,d" help:"remove a command"`

	Lock   string `cli:"lock"`
	Unlock string `cli:"unlock"`

	List     bool `cli:"list,list-by-name"`
	ListPath bool `cli:"list-by-path"`

	Config bool `help:"set configuration entry"`
}

// Before checks commandline validity.
func (c globalCmd) Before(args []string) error {
	if c.Add != "" && c.Remove != "" {
		return errors.New("don't pass both --add and --remove!!")
	}

	if c.Add != "" && len(args) < 1 {
		// require c.Add and args
		return errors.New("--add requires at least 2 args")
	}

	if c.Config && len(args) > 0 && len(args)%2 != 0 {
		return errors.New("--config requires an even number of arguments")
	}

	return nil
}

func (c globalCmd) Run(args []string) error {
	configPath := configPath()

	config, err := loadConfig(configPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
	}

	if config == nil {
		fmt.Fprintln(os.Stderr, "upgrade config (empty)")

		config = &Config{}
		err = config.Upgrade(configPath)
		if err != nil {
			return err
		}

	} else if config.Version < 2 {
		fmt.Fprintln(os.Stderr, "upgrade config (version < 2)")

		config = &Config{}
		err = config.Upgrade(configPath)
		if err != nil {
			return err
		}
	}

	if c.Config {
		if len(args) == 0 {
			printConfigs(configPath, *config)
			return nil
		}

		err := setConfig(config, args)
		if err != nil {
			return err
		}

		err = saveConfig(configPath, *config)
		if err != nil {
			return err
		}

		return nil
	}

	if c.Add == "" && c.Remove == "" && c.Lock == "" && c.Unlock == "" && len(args) < 1 {
		printCommands(configPath, *config, c.ListPath)
		return nil
	}
	if c.List || c.ListPath {
		printCommands(configPath, *config, c.ListPath)
		return nil
	}

	if c.Add != "" {
		err := addCommand(*config, c.Add, args[0], args[1:])
		if err != nil {
			return err
		}

		err = saveConfig(configPath, *config)
		if err != nil {
			return err
		}
		return nil
	}

	if c.Remove != "" {
		err := removeCommand(*config, c.Remove)
		if err != nil {
			return err
		}

		err = saveConfig(configPath, *config)
		if err != nil {
			return err
		}
		return nil
	}

	if c.Lock != "" {
		err := lockCommand(*config, c.Lock, true)
		if err != nil {
			return err
		}

		err = saveConfig(configPath, *config)
		if err != nil {
			return err
		}
		return nil
	}

	if c.Unlock != "" {
		err := lockCommand(*config, c.Unlock, false)
		if err != nil {
			return err
		}

		err = saveConfig(configPath, *config)
		if err != nil {
			return err
		}
		return nil
	}

	fcmd, fargs, err := config.FindCommand(args)
	if err != nil {
		return err
	}

	exitCode, err := execCommand(fcmd, fargs)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
	}
	if exitCode != 0 {
		os.Exit(exitCode)
	}

	return nil
}

func appName(defaultName string) string {
	appname, err := os.Executable()
	if err != nil {
		return defaultName
	}
	appname = filepath.Base(appname)
	ext := filepath.Ext(appname)
	return appname[:len(appname)-len(ext)]
}

func configPath() string {
	exePath, err := os.Executable()
	if err != nil {
		return ""
	}
	configName := filepath.Base(exePath)
	configName = configName[:len(configName)-len(filepath.Ext(configName))]

	finder := findcfg.New(
		findcfg.Name(configName),
		findcfg.YAML(),
		findcfg.JSON(),
		findcfg.Ext(".config", ".conf"),
		findcfg.ExecutableDir(),
		findcfg.UserConfigDir(userConfigFolder),
	)
	found := finder.Find()

	if found != nil {
		return found.Path
	}
	return finder.FallbackPath()
}

func loadConfig(configPath string) (*Config, error) {
	file, err := os.Open(configPath)
	if err != nil {
		return NewConfig(), nil
	}
	defer file.Close()

	if in(filepath.Ext(configPath), ".yaml", ".yml") {
		return LoadYAMLConfig(file)
	}
	if in(filepath.Ext(configPath), ".json") {
		return LoadConfig(file)
	}

	config, err := LoadYAMLConfig(file)
	if err == nil {
		return config, err
	}
	return LoadConfig(file)
}

func saveConfig(configPath string, config Config) error {
	file, err := os.Create(configPath)
	if err != nil {
		return err
	}

	if in(filepath.Ext(configPath), ".yaml", ".yml") {
		err = config.SaveYAML(file)
	} else {
		err = config.Save(file)
	}

	if err != nil {
		return err
	}

	file.Close()

	return nil
}

func printCommands(configPath string, config Config, byPath bool) {
	fmt.Println("Commands:")
	config.PrintCommand(byPath)

	fmt.Println("")
	printConfigs(configPath, config)
}

func printConfigs(configPath string, config Config) {
	fmt.Printf("Config: %s\n", configPath)
	config.PrintVariables(os.Stdout)
}

func setConfig(config *Config, args []string) error {
	return config.SetVariables(args)
}

func addCommand(config Config, name, path string, args []string) error {
	return config.AddCommand(
		strings.Split(name, "."),
		Command{
			Path: path,
			Args: args,
		},
	)
}

func removeCommand(config Config, name string) error {
	return config.RemoveCommand(strings.Split(name, "."))
}

func lockCommand(config Config, name string, locked bool) error {
	config.LockCommand(strings.Split(name, "."), locked)
	return nil
}

func execCommand(fcmd *Command, fargs []string) (int, error) {
	oscmds := make([]exec.Cmd, 1)
	curr := &oscmds[0]
	curr.Path = fcmd.Path
	p, err := exec.LookPath(curr.Path)
	if err == nil {
		curr.Path = p
	}
	curr.Args = append(curr.Args, fcmd.Path)
	for i, a := range fcmd.Args {
		if strings.HasPrefix(a, "|") {
			oscmds = append(oscmds, exec.Cmd{})
			curr = &oscmds[len(oscmds)-1]

			if fcmd.Args[i] != "|" {
				curr.Path = a[1:]
				p, err := exec.LookPath(curr.Path)
				if err == nil {
					curr.Path = p
				}
				curr.Args = append(curr.Args, a[1:])
			}
		} else {
			if curr.Path == "" {
				curr.Path = a
				p, err := exec.LookPath(curr.Path)
				if err == nil {
					curr.Path = p
				}
				curr.Args = append(curr.Args, a[1:])
			} else {
				curr.Args = append(curr.Args, a)
			}
		}
	}

	oscmds[0].Args = append(oscmds[0].Args, fargs...)

	oscmds[0].Stdin = os.Stdin
	oscmds[0].Stderr = os.Stderr
	oscmds[len(oscmds)-1].Stdout = os.Stdout
	oscmds[len(oscmds)-1].Stderr = os.Stderr
	for i := 1; i < len(oscmds); i++ {
		stdoutPipe, err := oscmds[i-1].StdoutPipe()
		if err != nil {
			return 1, fmt.Errorf("stdoutPipe: %w", err)
		}
		oscmds[i].Stdin = stdoutPipe
		oscmds[i].Stderr = os.Stderr
	}

	for i := range oscmds {
		err = oscmds[i].Start()
		if err != nil {
			return 1, fmt.Errorf("start: %w", err)
		}
	}

	for i := range oscmds {
		err = oscmds[i].Wait()
		if i == len(oscmds)-1 && err != nil {
			var exit *exec.ExitError
			if errors.As(err, &exit) {
				return exit.ExitCode(), nil
			}
		}
	}
	return 0, nil
}

func in(s string, choices ...string) bool {
	if len(choices) == 0 {
		return false
	}

	for i := 0; i < len(choices); i++ {
		if strings.EqualFold(s, choices[i]) {
			return true
		}
	}

	return false
}

func main() {
	appname := appName("f")

	app := gli.NewWith(&globalCmd{})
	app.Name = appname
	app.Desc = "command faker"
	app.Version = Version
	app.Usage = `# add (replace) a command
  {appname} --add gitinit git init
  {appname} --add goinit go mod init
# list commands
  {appname}
# run a command
  {appname} gitinit
# remove a command
  {appname} --remove gitinit
# add sub command
  {appname} --add m.c calc
  {appname} m c
# make another faker
  copy {appname} another.exe
  another --add gitinit echo hoge hoge
# sub match
  {appname} --config submatch true
  {appname} --add sub notepad
  {appname} su
  {appname} s
  {appname} --add subsub calc
  {appname} s # error: ambiguous

----

config dir:
    1. exe path
        {appname}.yaml
        Place the yaml in the same location as the executable.
    2. config directory 
        {CONFIG_DIR}/{userConfigFolder}/{appname}.yaml
        Windows: %appdata%\{userConfigFolder}\{appname}.yaml
        (see https://cs.opensource.google/go/go/+/go1.17.3:src/os/file.go;l=457)
    If none of 1,2 files exist, --add writes to 1.
`
	app.Usage = strings.ReplaceAll(app.Usage, "{appname}", appname)
	app.Usage = strings.ReplaceAll(app.Usage, "{userConfigFolder}", userConfigFolder)
	app.Copyright = "(C) 2021 Shuhei Kubota"
	app.DoubleHyphen = false
	app.SuppressErrorOutput = true
	err := app.Run(os.Args)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
