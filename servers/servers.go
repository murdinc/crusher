package servers

import (
	"fmt"
	"os"

	"github.com/murdinc/cli"
)

type Server struct {
	Nickname string `ini:"-"`
	Host     string
	Username string
	Class    string
}

type Servers []Server

// Assembles a new Server struct
func New(nickname, host, username, class string) *Server {
	server := new(Server)

	server.Nickname = nickname
	server.Host = host
	server.Username = username
	server.Class = class

	return server

}

// Prints all server config data in a table
func (servers Servers) Configure(class string) {

	collumns := []string{"Nickname", "Host", "Username", "Class"}
	var rows [][]string

	for _, s := range servers {
		if s.Class == class {
			rows = append(rows, []string{
				s.Nickname,
				s.Host,
				s.Username,
				s.Class,
			})
		}
	}

	if len(rows) == 0 {
		cli.Information(fmt.Sprintf("I couldn't find any servers set up with the class [%s], here is what I do have: ", class))
		servers.PrintAllServerInfo()
		os.Exit(0)
	}

	cli.Information(fmt.Sprintf("I found the following servers with the class [%s]:", class))
	PrintTable(collumns, rows)

	configure := cli.PromptBool("Do you want to proceed with configuring these servers?")

	if configure {
		cli.Information("Great! Hold onto your butts.")
	} else {
		cli.Information("Okay, maybe next time..")
	}

}

// Prints a single server config data in a table
func (s *Server) PrintServerInfo() {

	collumns := []string{"Nickname", "Host", "Username", "Class"}
	var rows [][]string

	rows = append(rows, []string{
		s.Nickname,
		s.Host,
		s.Username,
		s.Class,
	})

	PrintTable(collumns, rows)
}

// Prints all server config data in a table
func (servers Servers) PrintAllServerInfo() {

	// Build the table elements
	collumns := []string{"#", "Nickname", "Host", "Username", "Class"}

	var rows [][]string

	for i, s := range servers {
		rows = append(rows, []string{
			fmt.Sprint(i + 1),
			s.Nickname,
			s.Host,
			s.Username,
			s.Class,
		})
	}

	PrintTable(collumns, rows)
}

// Table helper
func PrintTable(collumns []string, rows [][]string) {
	fmt.Println("")
	t := cli.NewTable(rows, &cli.TableOptions{
		Padding:      1,
		UseSeparator: true,
	})
	t.SetHeader(collumns)
	fmt.Println(t.Render())
}
