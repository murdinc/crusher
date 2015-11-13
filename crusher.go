package main

import (
	"fmt"
	"os"

	"github.com/murdinc/cli"
	"github.com/murdinc/crusher/config"
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
	}

	app.Commands = []cli.Command{
		{
			Name:        "list-servers",
			ShortName:   "l",
			Example:     "crusher list-servers",
			Description: "List all configured remote servers",
			Action: func(c *cli.Context) {
				cli.Information(fmt.Sprintf("There are [%d] remote servers configured currently", len(cfg.Servers)))
				cfg.Servers.PrintAllServerInfo()

			},
		},
		{
			Name:        "remote-configure",
			ShortName:   "rc",
			Example:     "crusher remote-configure web",
			Description: "Configure all remote servers of a specified spec",
			Arguments: []cli.Argument{
				cli.Argument{Name: "spec", Description: "The spec group of remote servers to configure", Optional: false},
			},
			Action: func(c *cli.Context) {
				cfg.Servers.RemoteConfigure(c.NamedArg("spec"))
			},
		},
		{
			Name:        "add-server",
			ShortName:   "a",
			Example:     "crusher add-server",
			Description: "Add a new remote server to the config",
			Action: func(c *cli.Context) {
				cfg.AddServer()
			},
		},
		{
			Name:        "delete-server",
			ShortName:   "d",
			Example:     "crusher delete-server",
			Description: "Delete a remote server from the config",
			Action: func(c *cli.Context) {
				cfg.DeleteServer()
			},
		},
	}

	app.Run(os.Args)
}

//
////////////////..........

func log(kind string, err error) {
	if err == nil {
		fmt.Printf("%s\n", kind)
	} else {
		cli.ShowErrorMessage(kind, fmt.Sprintf("Details: %s", err))
		//os.Exit(1)
		//fmt.Printf("[ERROR - %s]: %s\n", kind, err)
	}
}

func prompt(string) string {

	return ""

}
