package config

import (
	"fmt"
	"os/user"

	"github.com/murdinc/cli"
	"github.com/murdinc/crusher/servers"

	"gopkg.in/ini.v1"
)

type CrusherConfig struct {
	Servers servers.Servers
}

// Reads in the config and returns a CrusherConfig struct
func ReadConfig() (*CrusherConfig, error) {

	config := new(CrusherConfig)

	currentUser, _ := user.Current()
	configLocation := currentUser.HomeDir + "/.crusher"

	cfg, err := ini.Load(configLocation)
	if err != nil {
		return config, err
	}

	remotes := cfg.Sections()

	for _, remote := range remotes {

		// We dont want the default right now
		if remote.Name() == "DEFAULT" {
			continue
		}

		server := new(servers.Server)

		err := remote.MapTo(server)
		if err != nil {
			return config, err
		}

		server.Name = remote.Name()
		config.Servers = append(config.Servers, *server)
	}

	return config, err
}

// Interactive new server setup
func (c *CrusherConfig) AddServer() {
	c.addServerDialog()

	more := cli.PromptBool("Awesome! Do you want to configure any more servers?")
	if more {
		c.AddServer()
	} else {
		cli.Information("Okay, I will save this configuration then!")
	}

	c.SaveConfig()

}

// Save our list of servers into the config file
func (c *CrusherConfig) SaveConfig() error {

	currentUser, _ := user.Current()
	configLocation := currentUser.HomeDir + "/.crusher"

	cfg := ini.Empty()

	for _, server := range c.Servers {
		err := cfg.Section(server.Name).ReflectFrom(&server)
		if err != nil {
			return err
		}

		// Hack to get bools to play nice, and not just output "<bool Value>" - I'll probably open a pull request once I track down the issue.
		cfg.Section(server.Name).NewKey("PassAuth", fmt.Sprintf("%t", server.PassAuth))
	}

	err := cfg.SaveToIndent(configLocation, "\t")
	if err != nil {
		return err
	}

	return nil
}

// Delete a specific server from the config file
func (c *CrusherConfig) DeleteServer() {
	count := len(c.Servers)
	cli.Information(fmt.Sprintf("There are [%d] servers configured currently", count))
	c.Servers.PrintAllServerInfo()

	index := cli.PromptInt("Which server would you like to delete from the config?", count) - 1

	c.Servers[index].PrintServerInfo()

	sure := cli.PromptBool("Are you sure you want to delete this server?")
	if sure {
		c.Servers, c.Servers[len(c.Servers)-1] = append(c.Servers[:index], c.Servers[index+1:]...), servers.Server{}
		c.SaveConfig()
	}

}

// Input flow for interactive server setup
func (c *CrusherConfig) addServerDialog() {
	name := cli.PromptString("What would you like to name this server?")
	host := cli.PromptString(fmt.Sprintf("What is the Hostname or IP of [%s]?", name))
	username := cli.PromptString(fmt.Sprintf("What Username would you like to use to connect to [%s]?", name))
	spec := cli.PromptString(fmt.Sprintf("What Spec would you like to assign to [%s]?", name))
	passAuth := cli.PromptBool(fmt.Sprintf("Does [%s] require password authentication?", name))

	server := servers.New(name, host, username, spec, passAuth)
	server.PrintServerInfo()

	correct := cli.PromptBool("Great! Does that look correct?")
	if correct {
		c.Servers = append(c.Servers, *server)
	} else {
		cli.Information("Okay, lets try that again then..")
		c.addServerDialog()
	}

}
