package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/shu-go/gli"
)

// Version is app version
var Version string

const userConfigFolder = "faker"

func init() {
	if Version == "" {
		Version = "dev-" + time.Now().Format("20060102")
	}
}

type globalCmd struct {
	Add    string `help:"add/replace a command"`
	Remove string `help:"remove a command"`

	List     bool `cli:"list,list-by-name"`
	ListPath bool `cli:"list-by-path"`

	Config bool `help:"set configuration entry"`
}

// Before checks commandline validity.
func (c globalCmd) Before(args []string) error {
	if c.Add != "" && c.Remove != "" {
		return errors.New("don't pass both --add and --remove!!")
	}

	if c.Config && len(args) > 0 && len(args)%2 != 0 {
		return errors.New("--config requires an even number of arguments")
	}

	return nil
}

func (c globalCmd) Run(args []string) error {
	configPath := determineConfigPath()

	config, err := loadConfig(configPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
	}

	if c.Config {
		if len(args) == 0 {
			printConfigs(configPath, config)
			return nil
		}

		err := setConfig(&config, args)
		if err != nil {
			return err
		}

		err = saveConfig(configPath, config)
		if err != nil {
			return err
		}

		return nil
	}

	if c.Add == "" && c.Remove == "" && len(args) < 1 {
		printCommands("", *config.RootCommand, configPath, config, c.ListPath)
		return nil
	}
	if c.List || c.ListPath {
		fcmd, _, err := config.FindCommand(args)
		if err != nil {
			return err
		}
		printCommands(args[len(args)-1], *fcmd, configPath, config, c.ListPath)
		return nil
	}

	if c.Add != "" {
		err := addCommand(config, c.Add, args[0], args[1:])
		if err != nil {
			return err
		}

		err = saveConfig(configPath, config)
		if err != nil {
			return err
		}
		return nil
	}

	if c.Remove != "" {
		err := removeCommand(config, c.Remove)
		if err != nil {
			return err
		}

		err = saveConfig(configPath, config)
		if err != nil {
			return err
		}
		return nil
	}

	fcmd, fargs, err := config.FindCommand(args)
	if err != nil {
		return err
	}
	if !fcmd.IsRunnable() {
		printCommands(args[len(args)-len(fargs)-1], *fcmd, configPath, config, c.ListPath)
		return nil
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

func determineAppName(defaultName string) string {
	appname, err := os.Executable()
	if err != nil {
		return defaultName
	}
	appname = filepath.Base(appname)
	ext := filepath.Ext(appname)
	return appname[:len(appname)-len(ext)]
}

func determineConfigPath() string {
	ep, err := os.Executable()
	if err != nil {
		return ""
	}

	ext := filepath.Ext(ep)
	if ext == "" {
		ep += ".json"
	} else {
		ep = ep[:len(ep)-len(ext)] + ".json"
	}

	info, err := os.Stat(ep)
	if err == nil && !info.IsDir() {
		return ep
	}
	// remember the ep

	// if ep not found, search for config dir

	configname := filepath.Base(ep)

	cp, err := os.UserConfigDir()
	if err != nil {
		return ep
	}

	cp = filepath.Join(cp, userConfigFolder, configname)

	info, err = os.Stat(cp)
	if err == nil && !info.IsDir() {
		return cp
	}

	return ep
}

func loadConfig(configPath string) (Config, error) {
	file, err := os.Open(configPath)
	if err != nil {
		return *NewConfig(), nil
	}
	defer file.Close()

	config, _ := LoadConfig(file)
	if config == nil {
		type OldCommand struct {
			Name string
			Path string
			Args []string
		}

		type OldConfig struct {
			Commands []OldCommand
		}

		_, err = file.Seek(0, 0)
		if err != nil {
			file.Close()
			return *NewConfig(), err
		}

		content, err := ioutil.ReadAll(file)
		if err != nil {
			file.Close()
			return *NewConfig(), err
		}

		file.Close()

		var oldConfig OldConfig
		err = json.Unmarshal(content, &oldConfig)
		if err != nil {
			return *NewConfig(), err
		}

		config = NewConfig()
		for _, oc := range oldConfig.Commands {
			cmd := Command{
				Path: oc.Path,
				Args: oc.Args,
			}
			err := config.AddCommand([]string{oc.Name}, cmd)
			return *config, err
		}

		err = saveConfig(configPath, *config)
		if err != nil {
			return *config, err
		}
	}

	return *config, nil
}

func saveConfig(configPath string, config Config) error {
	file, err := os.Create(configPath)
	if err != nil {
		return err
	}

	err = config.Save(file)
	if err != nil {
		return err
	}

	file.Close()

	return nil
}

func printCommands(name string, fcmd Command, configPath string, config Config, byPath bool) {
	fmt.Println("Commands:")
	fcmd.PrintCommand(name, byPath, -1)

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
	names := strings.Split(name, ".")
	cmd := Command{
		Path: path,
		Args: args,
	}

	err := config.AddCommand(names, cmd)
	if err != nil {
		return err
	}

	return nil
}

func removeCommand(config Config, name string) error {
	names := strings.Split(name, ".")

	err := config.RemoveCommand(names)
	if err != nil {
		return err
	}

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
	//rog.Print("fcmd.Args:", fcmd.Args)
	for i, a := range fcmd.Args {
		//rog.Print(a)

		if strings.HasPrefix(a, "|") {
			oscmds = append(oscmds, exec.Cmd{})
			curr = &oscmds[len(oscmds)-1]

			//rog.Print("new cmd")

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
			//rog.Printf("curr: %T", curr)
		}
	}

	//rog.Print("oscmds", len(oscmds))

	oscmds[0].Args = append(oscmds[0].Args, fargs...)

	oscmds[0].Stdin = os.Stdin
	oscmds[0].Stderr = os.Stderr
	oscmds[len(oscmds)-1].Stdout = os.Stdout
	oscmds[len(oscmds)-1].Stderr = os.Stderr
	for i := 1; i < len(oscmds); i++ {
		//rog.Print("pipe")
		stdoutPipe, err := oscmds[i-1].StdoutPipe()
		if err != nil {
			return 1, fmt.Errorf("stdoutPipe: %w", err)
		}
		oscmds[i].Stdin = stdoutPipe
		oscmds[i].Stderr = os.Stderr
	}
	//rog.Printf("oscmds:%#v", oscmds)

	for i := range oscmds {
		//rog.Printf("starting %#v", c)
		err = oscmds[i].Start()
		if err != nil {
			return 1, fmt.Errorf("start: %w", err)
		}
	}

	for i := range oscmds {
		err = oscmds[i].Wait()
		//rog.Print(oscmds[i], err)
		if i == len(oscmds)-1 && err != nil {
			var exit *exec.ExitError
			if errors.As(err, &exit) {
				return exit.ExitCode(), nil
			}
		}
	}
	return 0, nil
}

func main() {
	appname := determineAppName("f")

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
        {appname}.json
        Place the json in the same location as the executable.
    2. config directory 
        {CONFIG_DIR}/{userConfigFolder}/{appname}.json
        Windows: %appdata%\{userConfigFolder}\{appname}.json
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
