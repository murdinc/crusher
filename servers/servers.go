package servers

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/murdinc/cli"
)

type Server struct {
	Name     string `ini:"-"` // considered Sections in config file
	Host     string
	Username string
	Spec     string
	PassAuth bool
	Password string `ini:"-"` // Not stored in config, just where it gets temporarily stored when we ask for it.
}

type Servers []Server

// Assembles a new Server struct
func New(name, host, username, spec string, passAuth bool) *Server {
	server := new(Server)

	server.Name = name
	server.Host = host
	server.Username = username
	server.Spec = spec
	server.PassAuth = passAuth

	return server

}

// Prints a single server config data in a table
func (s *Server) PrintServerInfo() {

	collumns := []string{"Name", "Host", "Username", "Spec", "Password Auth?"}
	var rows [][]string

	rows = append(rows, []string{
		s.Name,
		s.Host,
		s.Username,
		s.Spec,
		fmt.Sprintf("%t", s.PassAuth),
	})

	PrintTable(collumns, rows)
}

type RemoteJob struct {
	Server  Server
	SSHConf *ssh.ClientConfig
	net.Conn
	Timeout time.Duration
	//
	Commands  []string
	Responses chan string
	Errors    chan error
}

func (r *RemoteJob) Read(b []byte) (int, error) {
	err := r.Conn.SetReadDeadline(time.Now().Add(r.Timeout))
	if err != nil {
		return 0, err
	}
	return r.Conn.Read(b)
}

func (r *RemoteJob) Write(b []byte) (int, error) {
	err := r.Conn.SetWriteDeadline(time.Now().Add(r.Timeout))
	if err != nil {
		return 0, err
	}
	return r.Conn.Write(b)
}

// Run Remote Configuration on a target spec group
func (s Servers) RemoteConfigure(spec string) {

	// Get our list of targets
	targetGroup := s.getTargetGroup(spec)

	configure := cli.PromptBool("Do you want to configure these servers?")

	if !configure {
		cli.Information("Okay, maybe next time..")
		os.Exit(0)
	}

	// TODO check if spec exists..
	// spec, err = GetSpec(spec)
	// commands := spec.Commands()

	// Get passwords for hosts that need them
	for i, server := range targetGroup {
		if server.PassAuth {
			targetGroup[i].Password = cli.PromptPassword(fmt.Sprintf("Please enter your password for user [%s] on remote server [%s]:", server.Username, server.Host))
		}
	}

	cli.Information("Great! I'll make it so..")

	responses := make(chan string, 10)
	errors := make(chan error, 10)

	// hold onto your butts
	for _, server := range targetGroup {

		sshConf := &ssh.ClientConfig{
			User: server.Username,
		}

		if server.PassAuth {
			sshConf.Auth = []ssh.AuthMethod{ssh.Password(server.Password)}
		} else {
			//sshConf.Auth =ssh.ClientAuth{ .. ssh stuff .. }
		}

		timeout := time.Second * 7
		job := RemoteJob{Server: server, Responses: responses, Errors: errors, Timeout: timeout, SSHConf: sshConf}

		go job.Run()

	}

	// Display Output of Jobs
	for done := 0; done < len(targetGroup); {
		select {
		case resp := <-responses:
			if resp == "done" {
				done++
				continue
			}
			fmt.Println(resp)

		case err := <-errors:
			fmt.Println(err.Error())
		}
	}
}

// Runs the remote Jobs and returns results on the job channels
func (job *RemoteJob) Run() {

	// Open a connection with a timeout
	conn, err := net.DialTimeout("tcp", job.Server.Host+":22", job.Timeout)
	if err != nil {
		job.Errors <- fmt.Errorf("[%s] DialTimeout Failed: %s", job.Server.Host, err)
		return
	}

	job.Conn = conn

	//
	c, chans, reqs, err := ssh.NewClientConn(job.Conn, job.Server.Host, job.SSHConf)
	if err != nil {
		job.Errors <- fmt.Errorf("[%s] NewClientConn Failed: %s", job.Server.Host, err)
		job.Responses <- "done"
		return
	}
	client := ssh.NewClient(c, chans, reqs)

	session, err := client.NewSession()
	if err != nil {
		job.Errors <- fmt.Errorf("[%s] NewSession Failed: %s", job.Server.Host, err)
		job.Responses <- "done"
		return
	}

	defer session.Close()

	var stdoutBuf bytes.Buffer
	session.Stdout = &stdoutBuf
	session.Run("uptime")

	job.Responses <- fmt.Sprintf("[%s - %s] Resp: %s", job.Server.Name, job.Server.Host, stdoutBuf.String())

	// End of the line
	job.Responses <- "done"

}

// Prints all server config data in a table
func (servers Servers) PrintAllServerInfo() {

	// Build the table elements
	collumns := []string{"#", "Name", "Host", "Username", "Spec", "Password Auth?"}

	var rows [][]string

	for i, s := range servers {
		rows = append(rows, []string{
			fmt.Sprint(i + 1),
			s.Name,
			s.Host,
			s.Username,
			s.Spec,
			fmt.Sprintf("%t", s.PassAuth),
		})
	}

	printTable(collumns, rows)
}

// Gets the target group of servers for a specified spec
func (servers Servers) getTargetGroup(spec string) Servers {

	collumns := []string{"Name", "Host", "Username", "Spec", "Password Auth?"}
	var rows [][]string
	var targetGroup Servers

	for _, s := range servers {
		if s.Spec == spec {

			rows = append(rows, []string{
				s.Name,
				s.Host,
				s.Username,
				s.Spec,
				fmt.Sprintf("%t", s.PassAuth),
			})

			targetGroup = append(targetGroup, s)
		}
	}

	if len(rows) == 0 {
		cli.Information(fmt.Sprintf("I couldn't find any servers set up with the spec [%s], here is what I do have: ", spec))
		servers.PrintAllServerInfo()
		os.Exit(0)
	}

	cli.Information(fmt.Sprintf("I found the following servers with the spec [%s]:", spec))
	printTable(collumns, rows)

	return targetGroup

}

// Table helper
func printTable(collumns []string, rows [][]string) {
	fmt.Println("")
	t := cli.NewTable(rows, &cli.TableOptions{
		Padding:      1,
		UseSeparator: true,
	})
	t.SetHeader(collumns)
	fmt.Println(t.Render())
}
