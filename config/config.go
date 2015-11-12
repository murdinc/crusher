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

	targets := cfg.Sections()

	for _, target := range targets {

		// We dont want the default right now
		if target.Name() == "DEFAULT" {
			continue
		}

		server := new(servers.Server)

		err := target.MapTo(server)
		if err != nil {
			return config, err
		}

		server.Nickname = target.Name()
		config.Servers = append(config.Servers, *server)
	}

	return config, err
}

// Interactive new server setup
func (c *CrusherConfig) AddServer() {
	c.addServerDialog()

	more := cli.PromptBool("Awesome! Do you want to configure any more server?")
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
		err := cfg.Section(server.Nickname).ReflectFrom(&server)
		fmt.Sprintf("ERROR: %s", err)
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
	cli.Information(fmt.Sprintf("There are [%d] target servers configured currently", count))
	c.Servers.PrintAllServerInfo()

	index := cli.PromptInt("Which target server would you like to delete from the config?", count) - 1

	c.Servers[index].PrintServerInfo()

	sure := cli.PromptBool("Are you sure you want to delete this target server?")
	if sure {
		c.Servers, c.Servers[len(c.Servers)-1] = append(c.Servers[:index], c.Servers[index+1:]...), servers.Server{}
		c.SaveConfig()
	}

}

// Input flow for interactive server setup
func (c *CrusherConfig) addServerDialog() {
	nickname := cli.PromptString("What would you like to nickname this server?")
	host := cli.PromptString(fmt.Sprintf("What is the Hostname or IP of [%s]?", nickname))
	username := cli.PromptString(fmt.Sprintf("What Username would you like to use to connect to [%s]?", nickname))
	class := cli.PromptString(fmt.Sprintf("What Class of server is [%s]?", nickname))

	server := servers.New(nickname, host, username, class)
	server.PrintServerInfo()

	correct := cli.PromptBool("Great! Does that look correct?")
	if correct {
		c.Servers = append(c.Servers, *server)
	} else {
		cli.Information("Okay, lets try that again then..")
		c.addServerDialog()
	}

}
