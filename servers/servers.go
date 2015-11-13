package servers

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/murdinc/cli"
	"github.com/murdinc/crusher/commands"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

// Represents a single remote server
type Server struct {
	Name     string `ini:"-"` // considered Sections in config file
	Host     string
	Username string
	Spec     string
	PassAuth bool
	Password string `ini:"-"` // Not stored in config, just where it gets temporarily stored when we ask for it.
}

// Slice of remote servers with attached methods
type Servers []Server

// Remote Job
type RemoteJob struct {
	net.Conn
	Server    Server
	SSHConf   *ssh.ClientConfig
	Timeout   time.Duration
	Commands  commands.Commands
	Responses chan string
	Errors    chan error
	WaitGroup *sync.WaitGroup
}

// Assembles a new Server struct
func New(name, host, username, spec string, passAuth bool) *Server {
	// Maybe do sanity checking here until a function with a callback is added to the cli library?
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

	printTable(collumns, rows)
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
	var wg sync.WaitGroup
	wg.Add(len(targetGroup))

	for _, server := range targetGroup {

		sshConf := &ssh.ClientConfig{
			User: server.Username,
		}

		if server.PassAuth {
			sshConf.Auth = []ssh.AuthMethod{ssh.Password(server.Password)}
		} else {
			cli.Information("SSH Key Auth is not yet implemented")
			wg.Done()
			continue
			//sshConf.Auth =ssh.ClientAuth{ .. ssh stuff .. }
		}

		timeout := time.Second * 7
		job := RemoteJob{
			Server:    server,
			Responses: responses,
			Errors:    errors,
			Timeout:   timeout,
			SSHConf:   sshConf,
			WaitGroup: &wg}

		go job.Run()

	}

	// Display Output of Jobs
	go func() {
		for {
			select {
			case err := <-errors:
				printErr(err.Error())
			case resp := <-responses:
				printResp(resp)
			}
		}
	}()

	wg.Wait()

	time.Sleep(time.Second)
}

func printResp(msg string) {
	template := `{{ ansi "fggreen"}}✓  {{ . }}{{ansi ""}}
	`
	cli.PrintAnsi(template, msg)
}

func printErr(msg string) {
	template := `{{ ansi "fgred"}}X  {{ . }}{{ansi ""}}
	`
	cli.PrintAnsi(template, msg)
}

// Runs the remote Jobs and returns results on the job channels
func (job *RemoteJob) Run() {

	defer job.WaitGroup.Done()

	header := addSpaces(fmt.Sprintf("[%s - %s]", job.Server.Name, job.Server.Host), 35)

	// Open a connection with a timeout
	conn, err := net.DialTimeout("tcp", job.Server.Host+":22", job.Timeout)

	if err != nil {
		job.Errors <- fmt.Errorf("%s DialTimeout Failed: %s", header, err)
		return
	}

	job.Conn = conn

	//
	c, chans, reqs, err := ssh.NewClientConn(job.Conn, job.Server.Host, job.SSHConf)
	if err != nil {
		job.Errors <- fmt.Errorf("%s NewClientConn Failed: %s", header, err)
		return
	}
	client := ssh.NewClient(c, chans, reqs)

	// ##################################################################################################################

	// open an SFTP session over an existing ssh connection.
	sftp, err := sftp.NewClient(client)
	if err != nil {
		job.Errors <- fmt.Errorf("%s SFTP NewClient Failed: %s", header, err)
		return
	}
	defer sftp.Close()

	// walk a directory
	w := sftp.Walk("/home/ubuntu/")
	for w.Step() {
		if w.Err() != nil {
			job.Errors <- fmt.Errorf("%s SFTP NewClient Failed: %s", header, w.Err())
			continue
		}
		job.Responses <- fmt.Sprintf("%s Response: %s", header, w.Path())
		//log.Println(w.Path())
	}

	// ##################################################################################################################

	session, err := client.NewSession()
	if err != nil {
		job.Errors <- fmt.Errorf("%s NewSession Failed: %s", header, err)
		return
	}

	defer session.Close()

	var stdoutBuf bytes.Buffer
	session.Stdout = &stdoutBuf

	// Command Looper™
	cmd := "uptime"
	err = session.Run(cmd)
	if err != nil {
		job.Errors <- fmt.Errorf("%s Command Failed! Command: %s Response: %s", header, cmd, err)
	}

	job.Responses <- fmt.Sprintf("%s Response: %s", header, stdoutBuf.String())

	// End of the line
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

func addSpaces(s string, w int) string {
	if len(s) < w {
		s += strings.Repeat(" ", w-len(s))
	}
	return s
}
