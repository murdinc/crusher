package main

import (
	"fmt"
	"os"

	"github.com/murdinc/cli"
	"github.com/murdinc/crusher/config"
	"github.com/murdinc/crusher/specr"
)

// Main Function
////////////////..........
func main() {

	app := cli.NewApp()
	app.Name = "crusher"
	app.Usage = "Command Line Configuration Management System"
	app.Version = "1.0"
	app.Author = "Ahmad Abdo"
	app.Email = "send@ahmad.pizza"

	app.Commands = []cli.Command{
		{
			Name:        "list-servers",
			ShortName:   "l",
			Example:     "crusher list-servers",
			Description: "List all configured remote servers",
			Action: func(c *cli.Context) {
				cfg := getConfig()
				cli.Information(fmt.Sprintf("There are [%d] remote servers configured currently", len(cfg.Servers)))
				cfg.Servers.PrintAllServerInfo()

			},
		},
		{
			Name:        "remote-configure",
			ShortName:   "rc",
			Example:     "crusher remote-configure hello_world",
			Description: "Configure one or many remote servers",
			Arguments: []cli.Argument{
				cli.Argument{Name: "search", Description: "The server or spec group to remote configure", Optional: false},
			},
			Action: func(c *cli.Context) {
				specList, err := specr.GetSpecs()
				if err != nil {
					cli.ShowErrorMessage("Error Reading Spec Files!", err.Error())
					return
				}

				search := c.NamedArg("search")

				cfg := getConfig()
				cfg.Servers.RemoteConfigure(search, specList)
			},
		},
		{
			Name:        "local-configure",
			ShortName:   "lc",
			Example:     "crusher local-configure hello_world",
			Description: "Configure this local machine with a given spec",
			Arguments: []cli.Argument{
				cli.Argument{Name: "spec", Description: "The spec to configure on this machine", Optional: false},
			},
			Action: func(c *cli.Context) {
				specList, err := specr.GetSpecs()
				if err != nil {
					cli.ShowErrorMessage("Error Reading Spec Files!", err.Error())
					return
				}

				specName := c.NamedArg("spec")
				if !specList.SpecExists(specName) {
					cli.ShowErrorMessage("Unable to find Spec!", fmt.Sprintf("I was unable to find a spec named [%s].", specName))
					return
				}

				//specList.ShowSpec(c.NamedArg("spec"))
				specList.LocalConfigure(specName)

			},
		},
		{
			Name:        "add-server",
			ShortName:   "a",
			Example:     "crusher add-server",
			Description: "Add a new remote server to the config",
			Action: func(c *cli.Context) {
				cfg := getConfig()
				cfg.AddServer()
			},
		},
		{
			Name:        "delete-server",
			ShortName:   "d",
			Example:     "crusher delete-server",
			Description: "Delete a remote server from the config",
			Action: func(c *cli.Context) {
				cfg := getConfig()
				cfg.DeleteServer()
			},
		},
		{
			Name:        "available-specs",
			ShortName:   "s",
			Example:     "crusher available-specs",
			Description: "List all available specs",
			Action: func(c *cli.Context) {
				specList, err := specr.GetSpecs()
				if err != nil {
					cli.ShowErrorMessage("Error Reading Spec Files!", err.Error())
				}

				cli.Information(fmt.Sprintf("There are [%d] specs available currently", len(specList.Specs)))
				specList.PrintSpecTable()
			},
		},
		{
			Name:        "show-spec",
			ShortName:   "ss",
			Example:     "crusher show-spec",
			Description: "Show what a given spec will build",
			Arguments: []cli.Argument{
				cli.Argument{Name: "spec", Description: "The spec to show", Optional: false},
			},
			Action: func(c *cli.Context) {
				specList, err := specr.GetSpecs()
				if err != nil {
					cli.ShowErrorMessage("Error Reading Spec Files!", err.Error())
				}

				specName := c.NamedArg("spec")
				if !specList.SpecExists(specName) {
					cli.ShowErrorMessage("Unable to find Spec!", fmt.Sprintf("I was unable to find a spec named [%s].", specName))
					return
				}

				specList.ShowSpec(specName)
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
		create := cli.BoxPromptBool("Crusher configuration file not found or empty!", "Do you want to add some servers now?")
		if !create {
			cli.Information("Alright then, maybe next time.. ")
			os.Exit(0)
		}
		// Add Some Servers to our config
		cfg.AddServer()
		os.Exit(0)
	}

	return cfg
}

//
////////////////..........
