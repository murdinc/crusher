package main

import (
	"fmt"
	"os"

	"github.com/murdinc/cli"
	"github.com/murdinc/crusher/config"
	"github.com/murdinc/crusher/specr"
	"github.com/murdinc/terminal"
)

// Main Function
////////////////..........
func main() {

	app := cli.NewApp()
	app.Name = "crusher"
	app.Usage = "Command Line Configuration Management System"
	app.Version = "1.1"
	app.Author = "Ahmad Abdo"
	app.Email = "send@ahmad.pizza"

	app.Commands = []cli.Command{
		{
			Name:        "list-servers",
			ShortName:   "l",
			Usage:       "crusher list-servers",
			Description: "List all configured remote servers",
			Action: func(c *cli.Context) error {
				cfg := getConfig()
				terminal.Information(fmt.Sprintf("There are [%d] remote servers configured currently", len(cfg.Servers)))
				cfg.Servers.PrintAllServerInfo()
				return nil
			},
		},
		{
			Name:        "remote-configure",
			ShortName:   "rc",
			Usage:       "crusher remote-configure hello_world",
			Description: "Configure one or many remote servers",
			Arguments: []cli.Argument{
				cli.Argument{Name: "search", Description: "The server or spec group to remote configure", Optional: false},
			},
			Action: func(c *cli.Context) error {
				specList, err := specr.GetSpecs()
				if err != nil {
					terminal.ShowErrorMessage("Error Reading Spec Files!", err.Error())
					return err
				}

				cfg := getConfig()
				cfg.Servers.RemoteConfigure(c.NamedArg("search"), specList)
				return nil
			},
		},
		{
			Name:        "local-configure",
			ShortName:   "lc",
			Usage:       "crusher local-configure hello_world",
			Description: "Configure this local machine with a given spec",
			Arguments: []cli.Argument{
				cli.Argument{Name: "spec", Description: "The spec to configure on this machine", Optional: false},
			},
			Action: func(c *cli.Context) error {
				specList, err := specr.GetSpecs()
				if err != nil {
					terminal.ShowErrorMessage("Error Reading Spec Files!", err.Error())
					return err
				}

				specName := c.NamedArg("spec")
				if !specList.SpecExists(specName) {
					terminal.ShowErrorMessage("Unable to find Spec!", fmt.Sprintf("I was unable to find a spec named [%s].", specName))
					return nil
				}

				specList.LocalConfigure(specName)
				return nil
			},
		},
		{
			Name:        "add-server",
			ShortName:   "a",
			Usage:       "crusher add-server",
			Description: "Add a new remote server to the config",
			Action: func(c *cli.Context) error {
				cfg := getConfig()
				return cfg.AddServer()
			},
		},
		{
			Name:        "delete-server",
			ShortName:   "d",
			Usage:       "crusher delete-server",
			Description: "Delete a remote server from the config",
			Action: func(c *cli.Context) error {
				cfg := getConfig()
				return cfg.DeleteServer()
			},
		},
		{
			Name:        "available-specs",
			ShortName:   "s",
			Usage:       "crusher available-specs",
			Description: "List all available specs",
			Action: func(c *cli.Context) error {
				specList, err := specr.GetSpecs()
				if err != nil {
					terminal.ShowErrorMessage("Error Reading Spec Files!", err.Error())
					return err
				}

				terminal.Information(fmt.Sprintf("There are [%d] specs available currently", len(specList.Specs)))
				specList.PrintSpecInformation()

				return nil
			},
		},
		{
			Name:        "show-spec",
			ShortName:   "ss",
			Usage:       "crusher show-spec",
			Description: "Show what a given spec will build",
			Arguments: []cli.Argument{
				cli.Argument{Name: "spec", Description: "The spec to show", Optional: false},
			},
			Action: func(c *cli.Context) error {
				specList, err := specr.GetSpecs()
				if err != nil {
					terminal.ShowErrorMessage("Error Reading Spec Files!", err.Error())
				}

				specName := c.NamedArg("spec")
				if !specList.SpecExists(specName) {
					terminal.ShowErrorMessage("Unable to find Spec!", fmt.Sprintf("I was unable to find a spec named [%s].", specName))
					return nil
				}

				specList.ShowSpec(specName)
				return nil
			},
		},
	}

	app.Run(os.Args)
}

func getConfig() *config.CrusherConfig {
	// Check Config
	cfg, err := config.ReadConfig()
	if err != nil || len(cfg.Servers) == 0 {
		// No Config Found, ask if we want to create one
		create := terminal.BoxPromptBool("Crusher configuration file not found or empty!", "Do you want to add some servers now?")
		if !create {
			terminal.Information("Alright then, maybe next time.. ")
			os.Exit(0)
			return nil
		}
		// Add Some Servers to our config
		cfg.AddServer()
		os.Exit(0)
		return nil
	}

	return cfg
}

//
////////////////..........
